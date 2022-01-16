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
)

const (
	// One of the Tunne CRD, ID, Name is mandatory
	// Tunnel CR Name
	tunnelCRAnnotation = "tunnels.networking.cfargotunnel.com/cr"
	// Tunnel ID matching Tunnel Resource
	tunnelIdAnnotation = "tunnels.networking.cfargotunnel.com/id"
	// Tunnel Name matching Tunnel Resource Spec
	tunnelNameAnnotation = "tunnels.networking.cfargotunnel.com/name"
	// FQDN to create a DNS entry for and route traffic from internet on, defaults to Service name + cloudflare domain
	fqdnAnnotation = "tunnels.networking.cfargotunnel.com/fqdn"
	// If this annotation is set to false, do not limit searching Tunnel to Service namespace, and pick the 1st one found (Might be random?)
	// If set to anything other than false, use it as a namspace where Tunnel exists
	tunnelNSAnnotation = "tunnels.networking.cfargotunnel.com/ns"

	// Protocol to use between cloudflared and the Service.
	// Defaults to http if protocol is tcp and port is 80, https if protocol is tcp and port is 443
	// Else, defaults to tcp if Service Proto is tcp and udp if Service Proto is udp.
	// Allowed values are in tunnelValidProtoMap (http, https, tcp, udp)
	tunnelProtoAnnotation = "tunnels.networking.cfargotunnel.com/proto"
	tunnelProtoHTTP       = "http"
	tunnelProtoHTTPS      = "https"
	tunnelProtoTCP        = "tcp"
	tunnelProtoUDP        = "udp"

	// Checksum of the config, used to restart pods in the deployment
	tunnelConfigChecksum = "tunnels.networking.cfargotunnel.com/checksum"

	tunnelFinalizerAnnotation = "tunnels.networking.cfargotunnel.com/finalizer"
	tunnelDomainAnnotation    = "tunnels.networking.cfargotunnel.com/domain"
	configmapKey              = "config.yaml"
)

var tunnelValidProtoMap map[string]bool = map[string]bool{
	tunnelProtoHTTP:  true,
	tunnelProtoHTTPS: true,
	tunnelProtoTCP:   true,
	tunnelProtoUDP:   true,
}

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch Service from API
	service := &corev1.Service{}

	if err := r.Get(ctx, req.NamespacedName, service); err != nil {
		if apierrors.IsNotFound(err) {
			// Service object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Service deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Service")
		return ctrl.Result{}, err
	}

	// Read Service annotations. If both annotations are not set, return without doing anything
	tunnelName, okName := service.Annotations[tunnelNameAnnotation]
	tunnelId, okId := service.Annotations[tunnelIdAnnotation]
	fqdn := service.Annotations[fqdnAnnotation]
	tunnelNS, okNS := service.Annotations[tunnelNSAnnotation]
	tunnelCRD, okCRD := service.Annotations[tunnelCRAnnotation]

	if !(okCRD || okName || okId) {
		// If a service with annotation is edited to remove just annotations, cleanup wont happen.
		// Not an issue as such, since it will be overwritten the next time it is used.
		log.Info("No related annotations not found, skipping Service")
		// Check if our finalizer is present on a non managed resource and remove it. This can happen if annotations were removed from the Service.
		if controllerutil.ContainsFinalizer(service, tunnelFinalizerAnnotation) {
			log.Info("Finalizer found on unmanaged Service, removing it")
			controllerutil.RemoveFinalizer(service, tunnelFinalizerAnnotation)
			err := r.Update(ctx, service)
			if err != nil {
				log.Error(err, "unable to remove finalizer from unmanaged Service")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// listOpts to search for ConfigMap. Set labels, and namespace restriction if
	listOpts := []client.ListOption{}
	labels := map[string]string{}
	if okId {
		labels[tunnelIdAnnotation] = tunnelId
	}
	if okName {
		labels[tunnelNameAnnotation] = tunnelName
	}
	if okCRD {
		labels[tunnelCRAnnotation] = tunnelCRD
	}

	if tunnelNS == "true" || !okNS {
		labels[tunnelNSAnnotation] = service.Namespace
		listOpts = append(listOpts, client.InNamespace(service.Namespace))
	} else if okNS && tunnelNS != "false" {
		labels[tunnelNSAnnotation] = tunnelNS
		listOpts = append(listOpts, client.InNamespace(tunnelNS))
	} // else, no filter on namespace, pick the 1st one

	listOpts = append(listOpts, client.MatchingLabels(labels))

	log.Info("setting tunnel", "listOpts", listOpts)

	// Check if Service is marked for deletion
	if service.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(service, tunnelFinalizerAnnotation) {
			// Run finalization logic. If the finalization logic fails,
			// don't remove the finalizer so that we can retry during the next reconciliation.

			if err := r.configureCloudflare(log, ctx, service, fqdn, listOpts, true); err != nil {
				return ctrl.Result{}, err
			}

			// Remove tunnelFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(service, tunnelFinalizerAnnotation)
			err := r.Update(ctx, service)
			if err != nil {
				log.Error(err, "unable to continue with Service deletion")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Add finalizer for Service
		if !controllerutil.ContainsFinalizer(service, tunnelFinalizerAnnotation) {
			controllerutil.AddFinalizer(service, tunnelFinalizerAnnotation)
			if err := r.Update(ctx, service); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Configure ConfigMap
		if err := r.configureCloudflare(log, ctx, service, fqdn, listOpts, false); err != nil {
			log.Error(err, "unable to configure ConfigMap", "key", configmapKey)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) getConfigMapConfiguration(ctx context.Context, log logr.Logger, listOpts []client.ListOption) (corev1.ConfigMap, Configuration, error) {
	// Fetch ConfigMap from API
	configMapList := &corev1.ConfigMapList{}
	if err := r.List(ctx, configMapList, listOpts...); err != nil {
		log.Error(err, "Failed to list ConfigMaps", "listOpts", listOpts)
		return corev1.ConfigMap{}, Configuration{}, err
	}
	if len(configMapList.Items) == 0 {
		err := fmt.Errorf("no configmaps found")
		log.Error(err, "Failed to list ConfigMaps", "listOpts", listOpts)
		return corev1.ConfigMap{}, Configuration{}, err
	}
	configmap := configMapList.Items[0]

	// Read ConfigMap YAML
	configStr, ok := configmap.Data[configmapKey]
	if !ok {
		err := fmt.Errorf("unable to find key `%s` in ConfigMap", configmapKey)
		log.Error(err, "unable to find key in ConfigMap", "key", configmapKey)
		return corev1.ConfigMap{}, Configuration{}, err
	}

	var config Configuration
	if err := yaml.Unmarshal([]byte(configStr), &config); err != nil {
		log.Error(err, "unable to read config as YAML")
		return corev1.ConfigMap{}, Configuration{}, err
	}
	return configmap, config, nil
}

func (r *ServiceReconciler) setConfigMapConfiguration(ctx context.Context, log logr.Logger, configmap corev1.ConfigMap, config Configuration) error {
	// Push updated changes
	var configStr string
	if configBytes, err := yaml.Marshal(config); err == nil {
		configStr = string(configBytes)
	} else {
		log.Error(err, "unable to marshal config to ConfigMap", "key", configmapKey)
		return err
	}
	configmap.Data[configmapKey] = configStr
	if err := r.Update(ctx, &configmap); err != nil {
		log.Error(err, "unable to marshal config to ConfigMap", "key", configmapKey)
		return err
	}

	// Set checksum as annotation on Deployment, causing a restart of the Pods to take config
	cfDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, apitypes.NamespacedName{Name: configmap.Name, Namespace: configmap.Namespace}, cfDeployment); err != nil {
		log.Error(err, "Error in getting deployment, failed to restart")
		return err
	}
	hash := md5.Sum([]byte(configStr))
	// Restart pods
	if cfDeployment.Spec.Template.Annotations == nil {
		cfDeployment.Spec.Template.Annotations = map[string]string{}
	}
	cfDeployment.Spec.Template.Annotations[tunnelConfigChecksum] = hex.EncodeToString(hash[:])
	if err := r.Update(ctx, cfDeployment); err != nil {
		log.Error(err, "Failed to update Deployment for restart")
		return err
	}
	log.Info("Restarted deployment")
	return nil
}

func (r *ServiceReconciler) configureCloudflare(log logr.Logger, ctx context.Context, service *corev1.Service, fqdn string, listOpts []client.ListOption, cleanup bool) error {
	var config Configuration
	var configmap corev1.ConfigMap
	tunnels := &networkingv1alpha1.TunnelList{}

	if err := r.List(ctx, tunnels, listOpts...); err != nil {
		log.Error(err, "unable to get tunnel")
		return err
	}

	if len(tunnels.Items) == 0 {
		err := fmt.Errorf("no tunnels found")
		log.Error(err, "Failed to list Tunnels", "listOpts", listOpts)
		return err
	}
	tunnel := tunnels.Items[0]
	cfAPI, _, err := getAPIDetails(r.Client, ctx, log, tunnel)
	if err != nil {
		log.Error(err, "unable to get API details")
		return err
	}

	if configmap, config, err = r.getConfigMapConfiguration(ctx, log, listOpts); err != nil {
		log.Error(err, "unable to get ConfigMap")
		return err
	}
	tunnelDomain := configmap.Labels[tunnelDomainAnnotation]

	if len(service.Spec.Ports) == 0 {
		err := fmt.Errorf("no ports found in service spec, cannot proceed")
		log.Error(err, "unable to read service")
		return err
	} else if len(service.Spec.Ports) > 1 {
		log.Info("Multiple ports definition found, picking the first in the list")
	}
	servicePort := service.Spec.Ports[0]

	// Logic to get serviceProto
	var serviceProto string
	tunnelProto := service.Annotations[tunnelProtoAnnotation]
	validProto := tunnelValidProtoMap[tunnelProto]

	if tunnelProto != "" && !validProto {
		log.Info("Invalid Protocol provided, following default protocol logic")
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
		log.Error(err, "could not select protocol", "portProtocol", servicePort.Protocol, "annotationProtocol", tunnelProto)
	}

	log.Info("Selected protocol", "protocol", serviceProto)

	var finalIngress []UnvalidatedIngressRule
	if cleanup {
		finalIngress = make([]UnvalidatedIngressRule, 0, len(config.Ingress))
	}

	cfIngressService := fmt.Sprintf("%s://%s.%s.svc:%d", serviceProto, service.Name, service.Namespace, servicePort.Port)

	// Generate fqdn string from Ingress Spec if not provided
	if fqdn == "" {
		fqdn = fmt.Sprintf("%s.%s", service.Name, tunnelDomain)
		log.Info("using default domain value", "domain", tunnelDomain)
	}
	log.Info("setting fqdn", "fqdn", fqdn)

	// Find if the host already exists in config. If so, modify
	found := false
	for i, v := range config.Ingress {
		if cleanup {
			if v.Hostname != fqdn && v.Service != cfIngressService {
				finalIngress = append(finalIngress, v)
			}
		} else if v.Hostname == fqdn {
			log.Info("found existing cfIngress for host, modifying the service", "service", cfIngressService)
			config.Ingress[i].Service = cfIngressService
			found = true
			break
		}
	}

	// Else add a new entry to the beginning. The last entry has to be the 404 entry
	if !cleanup && !found {
		log.Info("adding cfIngress for host to point to service", "service", cfIngressService)
		config.Ingress = append([]UnvalidatedIngressRule{{
			Hostname: fqdn,
			Service:  cfIngressService,
		}}, config.Ingress...)
	}

	// Delete record on cleanup and set/update on normal reconcile
	if cleanup {
		config.Ingress = finalIngress
		if err := cfAPI.DeleteDNSCName(fqdn); err != nil {
			return err
		}
		log.Info("Deleted DNS entry", "fqdn", fqdn)
	} else {
		if err := cfAPI.InsertOrUpdateCName(fqdn); err != nil {
			return err
		}
		log.Info("Inserted/Updated DNS entry", "fqdn", fqdn)
	}
	return r.setConfigMapConfiguration(ctx, log, configmap, config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}
