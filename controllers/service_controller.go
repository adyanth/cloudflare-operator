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
	"strings"

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

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Namespace string

	// Custom data for ease of (re)use

	ctx             context.Context
	log             logr.Logger
	config          *UnvalidatedIngressRule
	tunnel          *networkingv1alpha1.Tunnel
	clusterTunnel   *networkingv1alpha1.ClusterTunnel
	service         *corev1.Service
	configmap       *corev1.ConfigMap
	namespacedName  apitypes.NamespacedName
	cfAPI           *CloudflareAPI
	isClusterTunnel bool
}

// labelsForService returns the labels for selecting the resources served by a Tunnel.
func (r ServiceReconciler) labelsForService() map[string]string {
	labels := map[string]string{
		configHostnameLabel: r.config.Hostname,
		configServiceLabel:  encodeCfService(r.config.Service),
	}

	if r.isClusterTunnel {
		labels[clusterTunnelAnnotation] = r.clusterTunnel.Name
		labels[tunnelDomainLabel] = r.clusterTunnel.Spec.Cloudflare.Domain
	} else {
		labels[tunnelAnnotation] = r.tunnel.Name
		labels[tunnelDomainLabel] = r.tunnel.Spec.Cloudflare.Domain
	}

	return labels
}

func decodeLabel(label string, service corev1.Service) string {
	labelSplit := strings.Split(label, configServiceLabelSplit)
	return fmt.Sprintf("%s://%s.%s.svc:%s", labelSplit[0], service.Name, service.Namespace, labelSplit[1])
}

func encodeCfService(cfService string) string {
	protoSplit := strings.Split(cfService, "://")
	domainSplit := strings.Split(protoSplit[1], ":")
	return fmt.Sprintf("%s%s%s", protoSplit[0], configServiceLabelSplit, domainSplit[1])
}

func (r *ServiceReconciler) initStruct(ctx context.Context, service *corev1.Service) error {
	r.ctx = ctx
	r.service = service

	// Read Service annotations. If both annotations are not set, return without doing anything
	tunnelName, okTunnel := r.service.Annotations[tunnelAnnotation]
	clusterTunnelName, okClusterTunnel := r.service.Annotations[clusterTunnelAnnotation]

	if okTunnel == okClusterTunnel {
		err := fmt.Errorf("cannot have both tunnel and cluster tunnel annotations")
		r.log.Error(err, "error reading annotations")
		r.Recorder.Event(service, corev1.EventTypeWarning, "ErrAnno", "Conflicting annotations found")
		return err
	}

	var err error

	if okClusterTunnel {
		r.isClusterTunnel = true

		r.namespacedName = apitypes.NamespacedName{Name: clusterTunnelName, Namespace: r.Namespace}
		r.clusterTunnel = &networkingv1alpha1.ClusterTunnel{}
		if err := r.Get(r.ctx, r.namespacedName, r.clusterTunnel); err != nil {
			r.log.Error(err, "Failed to get ClusterTunnel", "namespacedName", r.namespacedName)
			r.Recorder.Event(service, corev1.EventTypeWarning, "ErrTunnel", "Error getting ClusterTunnel")
			return err
		}

		if r.cfAPI, _, err = getAPIDetails(r.ctx, r.Client, r.log, r.clusterTunnel.Spec, r.clusterTunnel.Status, r.Namespace); err != nil {
			r.log.Error(err, "unable to get API details")
			r.Recorder.Event(service, corev1.EventTypeWarning, "ErrApiConfig", "Error getting API details")
			return err
		}
	} else {
		r.isClusterTunnel = false

		r.namespacedName = apitypes.NamespacedName{Name: tunnelName, Namespace: r.service.Namespace}
		r.tunnel = &networkingv1alpha1.Tunnel{}
		if err := r.Get(r.ctx, r.namespacedName, r.tunnel); err != nil {
			r.log.Error(err, "Failed to get Tunnel", "namespacedName", r.namespacedName)
			r.Recorder.Event(service, corev1.EventTypeWarning, "ErrTunnel", "Error getting Tunnel")
			return err
		}

		if r.cfAPI, _, err = getAPIDetails(r.ctx, r.Client, r.log, r.tunnel.Spec, r.tunnel.Status, r.service.Namespace); err != nil {
			r.log.Error(err, "unable to get API details")
			r.Recorder.Event(service, corev1.EventTypeWarning, "ErrApiConfig", "Error getting API details")
			return err
		}
	}

	r.configmap = &corev1.ConfigMap{}
	if err := r.Get(r.ctx, r.namespacedName, r.configmap); err != nil {
		r.log.Error(err, "unable to get configmap for configuration")
		r.Recorder.Event(service, corev1.EventTypeWarning, "ErrConfigMap", "Error finding ConfigMap for Tunnel referenced by Service")
		return err
	}

	var config UnvalidatedIngressRule
	if config, err = r.getConfigForService("", nil); err != nil {
		r.log.Error(err, "error getting config for service")
		r.Recorder.Event(service, corev1.EventTypeWarning, "ErrBuildConfig", "Error building Tunnel configuration")
		return err
	}
	r.config = &config

	return nil
}

//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels,verbs=get
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels/status,verbs=get
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels,verbs=get
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels/status,verbs=get
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx)

	// Fetch Service from API
	service := &corev1.Service{}
	if err := r.Get(ctx, req.NamespacedName, service); err != nil {
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

	_, okTunnel := service.Annotations[tunnelAnnotation]
	_, okClusterTunnel := service.Annotations[clusterTunnelAnnotation]

	if !(okTunnel || okClusterTunnel) {
		// If a service with annotation is edited to remove just annotations, cleanup wont happen.
		// Not an issue as such, since it will be overwritten the next time it is used.
		return ctrl.Result{}, r.unManagedService(ctx, service)
	}

	if err := r.initStruct(ctx, service); err != nil {
		r.log.Error(err, "initialization failed")
		return ctrl.Result{}, err
	}

	// Check if Service is marked for deletion
	if r.service.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.deletionLogic()
	}

	if err := r.creationLogic(); err != nil {
		return ctrl.Result{}, err
	}

	// Configure ConfigMap
	r.Recorder.Event(service, corev1.EventTypeNormal, "Configuring", "Configuring ConfigMap")
	if err := r.configureCloudflare(); err != nil {
		r.log.Error(err, "unable to configure ConfigMap", "key", configmapKey)
		r.Recorder.Event(service, corev1.EventTypeWarning, "FailedConfigure", "Failed to configure ConfigMap")
		return ctrl.Result{}, err
	}
	r.Recorder.Event(service, corev1.EventTypeNormal, "Configured", "Configured Cloudflare Tunnel")
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) unManagedService(ctx context.Context, service *corev1.Service) error {
	r.log.Info("No related annotations not found, skipping Service")
	// Check if our finalizer is present on a non managed resource and remove it. This can happen if annotations were removed from the Service.
	if controllerutil.ContainsFinalizer(service, tunnelFinalizerAnnotation) {
		r.log.Info("Finalizer found on unmanaged Service, removing it")
		controllerutil.RemoveFinalizer(service, tunnelFinalizerAnnotation)
		err := r.Update(ctx, service)
		if err != nil {
			r.log.Error(err, "unable to remove finalizer from unmanaged Service")
			r.Recorder.Event(service, corev1.EventTypeWarning, "FailedFinalizerUnset", "Failed to remove Service Finalizer from unmanaged Service")
			return err
		}
		r.Recorder.Event(service, corev1.EventTypeNormal, "FinalizerUnset", "Service Finalizer removed, unmanaged Service")
	}
	// Our finalizer not present, nothing to do.
	return nil
}

func (r *ServiceReconciler) deletionLogic() error {
	if controllerutil.ContainsFinalizer(r.service, tunnelFinalizerAnnotation) {
		// Run finalization logic. If the finalization logic fails,
		// don't remove the finalizer so that we can retry during the next reconciliation.

		if err := r.cfAPI.DeleteDNSCName(r.config.Hostname); err != nil {
			r.Recorder.Event(r.service, corev1.EventTypeWarning, "FailedDeletingDns", "Failed to delete DNS entry")
			return err
		}
		r.log.Info("Deleted DNS entry", "Hostname", r.config.Hostname)
		r.Recorder.Event(r.service, corev1.EventTypeNormal, "DeletedDns", "Deleted DNS entry")

		// Remove tunnelFinalizer. Once all finalizers have been
		// removed, the object will be deleted.
		controllerutil.RemoveFinalizer(r.service, tunnelFinalizerAnnotation)
		err := r.Update(r.ctx, r.service)
		if err != nil {
			r.log.Error(err, "unable to continue with Service deletion")
			r.Recorder.Event(r.service, corev1.EventTypeWarning, "FailedFinalizerUnset", "Failed to remove Service Finalizer")
			return err
		}
		r.Recorder.Event(r.service, corev1.EventTypeNormal, "FinalizerUnset", "Service Finalizer removed")
	}
	// Already removed our finalizer, all good.
	return nil
}

func (r *ServiceReconciler) creationLogic() error {
	// Add finalizer for Service
	if !controllerutil.ContainsFinalizer(r.service, tunnelFinalizerAnnotation) {
		controllerutil.AddFinalizer(r.service, tunnelFinalizerAnnotation)
	}

	// Add labels for Service
	r.service.Labels = r.labelsForService()

	// Update Service resource
	if err := r.Update(r.ctx, r.service); err != nil {
		r.Recorder.Event(r.service, corev1.EventTypeWarning, "FailedMetaSet", "Failed to set Service Finalizer and Labels")
		return err
	}
	r.Recorder.Event(r.service, corev1.EventTypeNormal, "MetaSet", "Service Finalizer and Labels added")

	// Create DNS entry
	if err := r.cfAPI.InsertOrUpdateCName(r.config.Hostname); err != nil {
		r.Recorder.Event(r.service, corev1.EventTypeWarning, "FailedCreatingDns", "Failed to insert/update DNS entry")
		return err
	}
	r.log.Info("Inserted/Updated DNS entry")
	r.Recorder.Event(r.service, corev1.EventTypeNormal, "CreatedDns", "Inserted/Updated DNS entry")
	return nil
}

func (r *ServiceReconciler) getRelevantServices() ([]corev1.Service, error) {
	// Fetch Services from API
	var listOpts []client.ListOption
	if r.isClusterTunnel {
		listOpts = []client.ListOption{client.MatchingLabels(map[string]string{
			clusterTunnelAnnotation: r.clusterTunnel.Name,
		})}
	} else {
		listOpts = []client.ListOption{client.InNamespace(r.service.Namespace), client.MatchingLabels(map[string]string{
			tunnelAnnotation: r.tunnel.Name,
		})}
	}
	serviceList := &corev1.ServiceList{}
	if err := r.List(r.ctx, serviceList, listOpts...); err != nil {
		r.log.Error(err, "failed to list Services", "listOpts", listOpts)
		return []corev1.Service{}, err
	}

	if len(serviceList.Items) == 0 {
		r.log.Info("No services found, tunnel not in use", "listOpts", listOpts)
	}

	return serviceList.Items, nil
}

// Get the config entry to be added for this service
func (r ServiceReconciler) getConfigForService(tunnelDomain string, service *corev1.Service) (UnvalidatedIngressRule, error) {
	if service == nil {
		r.log.Info("Using current service for generating config")
		service = r.service
	}

	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("no ports found in service spec, cannot proceed")
		r.log.Error(err, "unable to read service")
		return UnvalidatedIngressRule{}, err
	} else if len(service.Spec.Ports) > 1 {
		r.log.Info("Multiple ports definition found, picking the first in the list")
	}

	servicePort := service.Spec.Ports[0]

	// Logic to get serviceProto
	var serviceProto string
	tunnelProto := service.Annotations[tunnelProtoAnnotation]
	validProto := tunnelValidProtoMap[tunnelProto]

	if tunnelProto != "" && !validProto {
		r.log.Info("Invalid Protocol provided, following default protocol logic")
	}

	if tunnelProto != "" && validProto {
		serviceProto = tunnelProto
	} else if servicePort.Protocol == corev1.ProtocolTCP {
		// Default protocol selection logic
		switch servicePort.Port {
		case 80:
			serviceProto = tunnelProtoHTTP
		case 443:
			serviceProto = tunnelProtoHTTPS
		default:
			serviceProto = tunnelProtoTCP
		}
	} else if servicePort.Protocol == corev1.ProtocolUDP {
		serviceProto = tunnelProtoUDP
	} else {
		err := fmt.Errorf("unsupported protocol")
		r.log.Error(err, "could not select protocol", "portProtocol", servicePort.Protocol, "annotationProtocol", tunnelProto)
	}

	r.log.Info("Selected protocol", "protocol", serviceProto)

	cfService := fmt.Sprintf("%s://%s.%s.svc:%d", serviceProto, service.Name, service.Namespace, servicePort.Port)

	cfHostname := service.Annotations[fqdnAnnotation]

	// Generate cfHostname string from Service Spec if not provided
	if cfHostname == "" {
		if tunnelDomain == "" {
			r.log.Info("Using current tunnel's domain for generating config")
			tunnelDomain = r.cfAPI.Domain
		}
		cfHostname = fmt.Sprintf("%s.%s", service.Name, tunnelDomain)
		r.log.Info("using default domain value", "domain", tunnelDomain)
	}

	r.log.Info("generated cloudflare config", "cfHostname", cfHostname, "cfService", cfService)

	return UnvalidatedIngressRule{Hostname: cfHostname, Service: cfService}, nil
}

func (r *ServiceReconciler) getConfigMapConfiguration() (*Configuration, error) {
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

func (r *ServiceReconciler) setConfigMapConfiguration(config *Configuration) error {
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
		r.Recorder.Event(r.service, corev1.EventTypeWarning, "FailedConfigure", "Failed to get Deployment")
		return err
	}
	hash := md5.Sum([]byte(configStr))
	// Restart pods
	r.Recorder.Event(r.service, corev1.EventTypeNormal, "ApplyingConfig", "Applying ConfigMap to Deployment")
	r.Recorder.Event(cfDeployment, corev1.EventTypeNormal, "ApplyingConfig", "Applying ConfigMap to Deployment")
	if cfDeployment.Spec.Template.Annotations == nil {
		cfDeployment.Spec.Template.Annotations = map[string]string{}
	}
	cfDeployment.Spec.Template.Annotations[tunnelConfigChecksum] = hex.EncodeToString(hash[:])
	if err := r.Update(r.ctx, cfDeployment); err != nil {
		r.log.Error(err, "Failed to update Deployment for restart")
		r.Recorder.Event(r.service, corev1.EventTypeWarning, "FailedApplyingConfig", "Failed to apply ConfigMap to Deployment")
		r.Recorder.Event(cfDeployment, corev1.EventTypeWarning, "FailedApplyingConfig", "Failed to apply ConfigMap to Deployment")
		return err
	}
	r.log.Info("Restarted deployment")
	r.Recorder.Event(r.service, corev1.EventTypeNormal, "AppliedConfig", "ConfigMap applied to Deployment")
	r.Recorder.Event(cfDeployment, corev1.EventTypeNormal, "AppliedConfig", "ConfigMap applied to Deployment")
	return nil
}

func (r *ServiceReconciler) configureCloudflare() error {
	var config *Configuration
	var err error

	if config, err = r.getConfigMapConfiguration(); err != nil {
		r.log.Error(err, "unable to get ConfigMap")
		return err
	}

	services, err := r.getRelevantServices()
	if err != nil {
		r.log.Error(err, "unable to get services")
		return err
	}

	// Total number of ingresses is the number of services + 1 for the catchall ingress
	finalIngresses := make([]UnvalidatedIngressRule, 0, len(services)+1)

	for _, service := range services {
		finalIngresses = append(finalIngresses, UnvalidatedIngressRule{
			Hostname: service.Labels[configHostnameLabel],
			Service:  decodeLabel(service.Labels[configServiceLabel], service),
		})
	}
	// Catchall ingress
	finalIngresses = append(finalIngresses, UnvalidatedIngressRule{
		Service: "http_status:404",
	})

	config.Ingress = finalIngresses

	return r.setConfigMapConfiguration(config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("cloudflare-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
