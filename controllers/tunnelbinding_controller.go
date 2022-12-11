/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sort"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/record"
)

// TunnelBindingReconciler reconciles a TunnelBinding object
type TunnelBindingReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	Recorder           record.EventRecorder
	OverwriteUnmanaged bool

	// Custom data for ease of (re)use

	ctx            context.Context
	log            logr.Logger
	binding        *networkingv1alpha1.TunnelBinding
	configmap      *corev1.ConfigMap
	fallbackTarget string
	cfAPI          *CloudflareAPI
}

// labelsForBinding returns the labels for selecting the Bindings served by a Tunnel.
func (r TunnelBindingReconciler) labelsForBinding() map[string]string {
	labels := map[string]string{
		tunnelNameLabel: r.binding.Name,
		tunnelKindLabel: r.binding.Kind,
	}

	return labels
}

func (r *TunnelBindingReconciler) initStruct(ctx context.Context, tunnelBinding *networkingv1alpha1.TunnelBinding) error {
	r.ctx = ctx
	r.binding = tunnelBinding

	var err error
	namespacedName := apitypes.NamespacedName{Name: r.binding.TunnelRef.Name, Namespace: r.binding.Namespace}

	// Process based on Tunnel Kind
	switch r.binding.TunnelRef.Kind {
	case "ClusterTunnel":
		clusterTunnel := &networkingv1alpha1.ClusterTunnel{}
		if err := r.Get(r.ctx, namespacedName, clusterTunnel); err != nil {
			r.log.Error(err, "Failed to get ClusterTunnel", "namespacedName", namespacedName)
			r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "ErrTunnel", "Error getting ClusterTunnel")
			return err
		}

		r.fallbackTarget = clusterTunnel.Spec.FallbackTarget

		if r.cfAPI, _, err = getAPIDetails(r.ctx, r.Client, r.log, clusterTunnel.Spec, clusterTunnel.Status, r.binding.Namespace); err != nil {
			r.log.Error(err, "unable to get API details")
			r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "ErrApiConfig", "Error getting API details")
			return err
		}
	case "Tunnel":
		tunnel := &networkingv1alpha1.Tunnel{}
		if err := r.Get(r.ctx, namespacedName, tunnel); err != nil {
			r.log.Error(err, "Failed to get Tunnel", "namespacedName", namespacedName)
			r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "ErrTunnel", "Error getting Tunnel")
			return err
		}

		r.fallbackTarget = tunnel.Spec.FallbackTarget

		if r.cfAPI, _, err = getAPIDetails(r.ctx, r.Client, r.log, tunnel.Spec, tunnel.Status, r.binding.Namespace); err != nil {
			r.log.Error(err, "unable to get API details")
			r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "ErrApiConfig", "Error getting API details")
			return err
		}
	default:
		err = fmt.Errorf("invalid kind")
		r.log.Error(err, "unsupported tunnelRef Kind")
		r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "ErrTunnelKind", "Unsupported tunnel kind")
		return err
	}

	r.configmap = &corev1.ConfigMap{}
	if err := r.Get(r.ctx, namespacedName, r.configmap); err != nil {
		r.log.Error(err, "unable to get configmap for configuration")
		r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "ErrConfigMap", "Error finding ConfigMap for Tunnel referenced by TunnelBinding")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnelbindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnelbindings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnelbindings/finalizers,verbs=update
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels,verbs=get
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels/status,verbs=get
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels,verbs=get
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TunnelBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx)

	// Fetch TunnelBinding from API
	tunnelBinding := &networkingv1alpha1.TunnelBinding{}
	if err := r.Get(ctx, req.NamespacedName, tunnelBinding); err != nil {
		if apierrors.IsNotFound(err) {
			// Service object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Info("Service deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		r.log.Error(err, "unable to fetch Service")
		return ctrl.Result{}, err
	}

	if err := r.initStruct(ctx, tunnelBinding); err != nil {
		r.log.Error(err, "initialization failed")
		return ctrl.Result{}, err
	}

	// Check if TunnelBinding is marked for deletion
	if r.binding.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.deletionLogic()
	}

	r.setStatus()

	// Configure ConfigMap
	r.Recorder.Event(tunnelBinding, corev1.EventTypeNormal, "Configuring", "Configuring ConfigMap")
	if err := r.configureCloudflareDaemon(); err != nil {
		r.log.Error(err, "unable to configure ConfigMap", "key", configmapKey)
		r.Recorder.Event(tunnelBinding, corev1.EventTypeWarning, "FailedConfigure", "Failed to configure ConfigMap")
		return ctrl.Result{}, err
	}
	r.Recorder.Event(tunnelBinding, corev1.EventTypeNormal, "Configured", "Configured Cloudflare Tunnel")

	if err := r.creationLogic(); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TunnelBindingReconciler) setStatus() {
	status := make([]networkingv1alpha1.ServiceInfo, 0, len(r.binding.Subjects))
	for _, sub := range r.binding.Subjects {
		hostname, target, err := r.getConfigForSubject(sub)
		if err != nil {
			r.log.Error(err, "error getting config for service", "svc", sub.Name)
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "ErrBuildConfig",
				fmt.Sprintf("Error building Tunnel configuration, svc: %s", sub.Name))
		}
		status = append(status, networkingv1alpha1.ServiceInfo{Hostname: hostname, Target: target})
	}
	r.binding.Status.Services = status
}

func (r *TunnelBindingReconciler) deletionLogic() error {
	if controllerutil.ContainsFinalizer(r.binding, tunnelFinalizer) {
		// Run finalization logic. If the finalization logic fails,
		// don't remove the finalizer so that we can retry during the next reconciliation.

		errors := false
		var err error
		for _, info := range r.binding.Status.Services {
			if err = r.deleteDNSLogic(info.Hostname); err != nil {
				errors = true
			}
		}
		if errors {
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FinalizerNotUnset", "Not removing Finalizer due to errors")
			return err
		}

		// Remove tunnelFinalizer. Once all finalizers have been
		// removed, the object will be deleted.
		controllerutil.RemoveFinalizer(r.binding, tunnelFinalizer)
		err = r.Update(r.ctx, r.binding)
		if err != nil {
			r.log.Error(err, "unable to delete Finalizer")
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedFinalizerUnset", "Failed to remove Finalizer")
			return err
		}
		r.Recorder.Event(r.binding, corev1.EventTypeNormal, "FinalizerUnset", "Finalizer removed")
	}
	// Already removed our finalizer, all good.
	return nil
}

func (r *TunnelBindingReconciler) creationLogic() error {
	// Add finalizer for Service
	if !controllerutil.ContainsFinalizer(r.binding, tunnelFinalizer) {
		controllerutil.AddFinalizer(r.binding, tunnelFinalizer)
	}

	// Add labels for Service
	if r.binding.Labels == nil {
		r.binding.Labels = make(map[string]string)
	}
	for k, v := range r.labelsForBinding() {
		r.binding.Labels[k] = v
	}

	// Update Service resource
	if err := r.Update(r.ctx, r.binding); err != nil {
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedMetaSet", "Failed to set TunnelBinding Finalizer and Labels")
		return err
	}
	r.Recorder.Event(r.binding, corev1.EventTypeNormal, "MetaSet", "TunnelBinding Finalizer and Labels added")

	errors := false
	var err error
	// Create DNS entries
	for _, info := range r.binding.Status.Services {
		err = r.createDNSLogic(info.Hostname)
		if err != nil {
			errors = true
		}
	}
	if errors {
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedDNSCreatePartial", "Some DNS entries failed to create")
		return err
	}
	return nil
}

func (r *TunnelBindingReconciler) createDNSLogic(hostname string) error {
	txtId, dnsTxtResponse, canUseDns, err := r.cfAPI.GetManagedDnsTxt(hostname)
	if err != nil {
		// We should not use this entry
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedReadingTxt", "Failed to read existing TXT DNS entry")
		return err
	}
	if !canUseDns {
		// We cannot use this entry
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedReadingTxt", fmt.Sprintf("FQDN already managed by Tunnel Name: %s, Id: %s", dnsTxtResponse.TunnelName, dnsTxtResponse.TunnelId))
		return err
	}
	existingId, err := r.cfAPI.GetDNSCNameId(hostname)
	// Check if a DNS record exists
	if err == nil || existingId != "" {
		// without a managed TXT record when we are not supposed to overwrite it
		if !r.OverwriteUnmanaged && txtId == "" {
			err := fmt.Errorf("unmanaged FQDN present")
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedReadingTxt", "FQDN present but unmanaged by Tunnel")
			return err
		}
		// To overwrite
		dnsTxtResponse.DnsId = existingId
	}

	newDnsId, err := r.cfAPI.InsertOrUpdateCName(hostname, dnsTxtResponse.DnsId)
	if err != nil {
		r.log.Error(err, "Failed to insert/update DNS entry", "Hostname", hostname)
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedCreatingDns", fmt.Sprintf("Failed to insert/update DNS entry: %s", err.Error()))
		return err
	}
	if err := r.cfAPI.InsertOrUpdateTXT(hostname, txtId, newDnsId); err != nil {
		r.log.Error(err, "Failed to insert/update TXT entry", "Hostname", hostname)
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedCreatingTxt", fmt.Sprintf("Failed to insert/update TXT entry: %s", err.Error()))
		if err := r.cfAPI.DeleteDNSId(hostname, newDnsId, dnsTxtResponse.DnsId != ""); err != nil {
			r.log.Info("Failed to delete DNS entry, left in broken state", "Hostname", hostname)
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedDeletingDns", "Failed to delete DNS entry, left in broken state")
			return err
		}
		if dnsTxtResponse.DnsId != "" {
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "DeletedDns", "Deleted DNS entry, retrying")
			r.log.Info("Deleted DNS entry", "Hostname", hostname)
		} else {
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "PreventDeleteDns", "Prevented DNS entry deletion, retrying")
			r.log.Info("Did not delete DNS entry", "Hostname", hostname)
		}
		return err
	}

	r.log.Info("Inserted/Updated DNS/TXT entry")
	r.Recorder.Event(r.binding, corev1.EventTypeNormal, "CreatedDns", "Inserted/Updated DNS/TXT entry")
	return nil
}

func (r *TunnelBindingReconciler) deleteDNSLogic(hostname string) error {
	// Delete DNS entry
	txtId, dnsTxtResponse, canUseDns, err := r.cfAPI.GetManagedDnsTxt(hostname)
	if err != nil {
		// We should not use this entry
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedReadingTxt", "Failed to read existing TXT DNS entry, not cleaning up")
	} else if !canUseDns {
		// We cannot use this entry. This should be happen if all controllers are using DNS management with the same prefix.
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedReadingTxt", fmt.Sprintf("FQDN already managed by Tunnel Name: %s, Id: %s, not cleaning up", dnsTxtResponse.TunnelName, dnsTxtResponse.TunnelId))
	} else {
		if id, err := r.cfAPI.GetDNSCNameId(hostname); err != nil {
			r.log.Error(err, "Error fetching DNS record", "Hostname", hostname)
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedDeletingDns", "Error fetching DNS record")
		} else if id != dnsTxtResponse.DnsId {
			err := fmt.Errorf("DNS ID from TXT and real DNS record does not match")
			r.log.Error(err, "DNS ID from TXT and real DNS record does not match", "Hostname", hostname)
			r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedDeletingDns", "DNS/TXT ID Mismatch")
		} else {
			if err := r.cfAPI.DeleteDNSId(hostname, dnsTxtResponse.DnsId, true); err != nil {
				r.log.Info("Failed to delete DNS entry", "Hostname", hostname)
				r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedDeletingDns", fmt.Sprintf("Failed to delete DNS entry: %s", err.Error()))
				return err
			}
			r.log.Info("Deleted DNS entry", "Hostname", hostname)
			r.Recorder.Event(r.binding, corev1.EventTypeNormal, "DeletedDns", "Deleted DNS entry")
			if err := r.cfAPI.DeleteDNSId(hostname, txtId, true); err != nil {
				r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedDeletingTxt", fmt.Sprintf("Failed to delete TXT entry: %s", err.Error()))
				return err
			}
			r.log.Info("Deleted DNS TXT entry", "Hostname", hostname)
			r.Recorder.Event(r.binding, corev1.EventTypeNormal, "DeletedTxt", "Deleted DNS TXT entry")
		}
	}
	return nil
}

func (r *TunnelBindingReconciler) getRelevantTunnelBindings() ([]networkingv1alpha1.TunnelBinding, error) {
	// Fetch TunnelBindings from API
	listOpts := []client.ListOption{client.MatchingLabels(map[string]string{
		tunnelNameLabel: r.binding.Name,
		tunnelKindLabel: r.binding.Kind,
	})}
	tunnelBindingList := &networkingv1alpha1.TunnelBindingList{}
	if err := r.List(r.ctx, tunnelBindingList, listOpts...); err != nil {
		r.log.Error(err, "failed to list Tunnel Bindings", "listOpts", listOpts)
		return tunnelBindingList.Items, err
	}

	bindings := tunnelBindingList.Items

	if len(bindings) == 0 {
		r.log.Info("No services found, tunnel not in use")
	}

	// Sort by binding name for idempotent config generation
	sort.Slice(bindings, func(i, j int) bool {
		return bindings[i].Name < bindings[j].Name
	})

	return bindings, nil
}

// Get the config entry to be added for this service
func (r TunnelBindingReconciler) getConfigForSubject(subject networkingv1alpha1.TunnelBindingSubject) (string, string, error) {
	hostname := subject.Spec.Fqdn
	target := "http_status:404"

	// Generate cfHostname string from Service Spec if not provided
	if hostname == "" {
		r.log.Info("Using current tunnel's domain for generating config")
		hostname = fmt.Sprintf("%s.%s", subject.Name, r.cfAPI.Domain)
		r.log.Info("using default domain value", "domain", r.cfAPI.Domain)
	}

	service := &corev1.Service{}
	if err := r.Get(r.ctx, apitypes.NamespacedName{Name: subject.Name, Namespace: r.binding.Namespace}, service); err != nil {
		r.log.Error(err, "Error getting referenced service")
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedService", "Failed to get Service")
		return hostname, target, err
	}

	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("no ports found in service spec, cannot proceed")
		r.log.Error(err, "unable to read service ports", "svc", service.Name)
		return hostname, target, err
	} else if len(service.Spec.Ports) > 1 {
		r.log.Info("Multiple ports definition found, picking the first in the list", "svc", service.Name)
	}

	servicePort := service.Spec.Ports[0]
	tunnelProto := subject.Spec.Protocol
	validProto := tunnelValidProtoMap[tunnelProto]

	serviceProto := r.getServiceProto(tunnelProto, validProto, servicePort)

	r.log.Info("Selected protocol", "protocol", serviceProto)

	target = fmt.Sprintf("%s://%s.%s.svc:%d", serviceProto, service.Name, service.Namespace, servicePort.Port)

	r.log.Info("generated cloudflare config", "hostname", hostname, "target", target)

	return hostname, target, nil
}

// getServiceProto returns the service protocol to be used
func (r *TunnelBindingReconciler) getServiceProto(tunnelProto string, validProto bool, servicePort corev1.ServicePort) string {
	var serviceProto string
	if tunnelProto != "" && !validProto {
		r.log.Info("Invalid Protocol provided, following default protocol logic")
	}

	if tunnelProto != "" && validProto {
		serviceProto = tunnelProto
	} else if servicePort.Protocol == corev1.ProtocolTCP {
		// Default protocol selection logic
		switch servicePort.Port {
		case 22:
			serviceProto = tunnelProtoSSH
		case 139, 445:
			serviceProto = tunnelProtoSMB
		case 443:
			serviceProto = tunnelProtoHTTPS
		case 3389:
			serviceProto = tunnelProtoRDP
		default:
			serviceProto = tunnelProtoHTTP
		}
	} else if servicePort.Protocol == corev1.ProtocolUDP {
		serviceProto = tunnelProtoUDP
	} else {
		err := fmt.Errorf("unsupported protocol")
		r.log.Error(err, "could not select protocol", "portProtocol", servicePort.Protocol, "annotationProtocol", tunnelProto)
	}
	return serviceProto
}

func (r *TunnelBindingReconciler) getConfigMapConfiguration() (*Configuration, error) {
	// Read ConfigMap YAML
	configStr, ok := r.configmap.Data[configmapKey]
	if !ok {
		err := fmt.Errorf("unable to find key `%s` in ConfigMap", configmapKey)
		r.log.Error(err, "unable to find key in ConfigMap", "key", configmapKey)
		return &Configuration{}, err
	}

	config := &Configuration{}
	if err := yaml.Unmarshal([]byte(configStr), config); err != nil {
		r.log.Error(err, "unable to read config as YAML")
		return &Configuration{}, err
	}
	return config, nil
}

func (r *TunnelBindingReconciler) setConfigMapConfiguration(config *Configuration) error {
	// Push updated changes
	var configStr string
	if configBytes, err := yaml.Marshal(config); err == nil {
		configStr = string(configBytes)
	} else {
		r.log.Error(err, "unable to marshal config to ConfigMap", "key", configmapKey)
		return err
	}
	r.configmap.Data[configmapKey] = configStr
	if err := r.Update(r.ctx, r.configmap); err != nil {
		r.log.Error(err, "unable to marshal config to ConfigMap", "key", configmapKey)
		return err
	}

	// Set checksum as annotation on Deployment, causing a restart of the Pods to take config
	cfDeployment := &appsv1.Deployment{}
	if err := r.Get(r.ctx, apitypes.NamespacedName{Name: r.configmap.Name, Namespace: r.configmap.Namespace}, cfDeployment); err != nil {
		r.log.Error(err, "Error in getting deployment, failed to restart")
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedConfigure", "Failed to get Deployment")
		return err
	}
	hash := md5.Sum([]byte(configStr))
	// Restart pods
	r.Recorder.Event(r.binding, corev1.EventTypeNormal, "ApplyingConfig", "Applying ConfigMap to Deployment")
	r.Recorder.Event(cfDeployment, corev1.EventTypeNormal, "ApplyingConfig", "Applying ConfigMap to Deployment")
	if cfDeployment.Spec.Template.Annotations == nil {
		cfDeployment.Spec.Template.Annotations = map[string]string{}
	}
	cfDeployment.Spec.Template.Annotations[tunnelConfigChecksum] = hex.EncodeToString(hash[:])
	if err := r.Update(r.ctx, cfDeployment); err != nil {
		r.log.Error(err, "Failed to update Deployment for restart")
		r.Recorder.Event(r.binding, corev1.EventTypeWarning, "FailedApplyingConfig", "Failed to apply ConfigMap to Deployment")
		r.Recorder.Event(cfDeployment, corev1.EventTypeWarning, "FailedApplyingConfig", "Failed to apply ConfigMap to Deployment")
		return err
	}
	r.log.Info("Restarted deployment")
	r.Recorder.Event(r.binding, corev1.EventTypeNormal, "AppliedConfig", "ConfigMap applied to Deployment")
	r.Recorder.Event(cfDeployment, corev1.EventTypeNormal, "AppliedConfig", "ConfigMap applied to Deployment")
	return nil
}

func (r *TunnelBindingReconciler) configureCloudflareDaemon() error {
	var config *Configuration
	var err error

	if config, err = r.getConfigMapConfiguration(); err != nil {
		r.log.Error(err, "unable to get ConfigMap")
		return err
	}

	bindings, err := r.getRelevantTunnelBindings()
	if err != nil {
		r.log.Error(err, "unable to get tunnel bindings")
		return err
	}

	// Total number of ingresses is the number of services + 1 for the catchall ingress
	// Set to 16 initially
	finalIngresses := make([]UnvalidatedIngressRule, 0, 16)
	for _, binding := range bindings {
		for i, subject := range binding.Subjects {
			targetService := ""
			if subject.Spec.Target != "" {
				targetService = subject.Spec.Target
			} else {
				targetService = binding.Status.Services[i].Target
			}

			originRequest := OriginRequestConfig{}
			originRequest.NoTLSVerify = &subject.Spec.NoTlsVerify
			if caPool := subject.Spec.CaPool; caPool != "" {
				caPath := fmt.Sprintf("/etc/cloudflared/certs/%s", caPool)
				originRequest.CAPool = &caPath
			}

			finalIngresses = append(finalIngresses, UnvalidatedIngressRule{
				Hostname:      binding.Status.Services[i].Hostname,
				Service:       targetService,
				OriginRequest: originRequest,
			})
		}
	}

	// Catchall ingress
	finalIngresses = append(finalIngresses, UnvalidatedIngressRule{
		Service: r.fallbackTarget,
	})

	config.Ingress = finalIngresses

	return r.setConfigMapConfiguration(config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("cloudflare-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.TunnelBinding{}).
		Complete(r)
}
