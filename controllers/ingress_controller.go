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
	networkingv1 "k8s.io/api/networking/v1"
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
	// FQDN to create a DNS entry for and route traffic from internet on, defaults to Ingress host subdomain + cloudflare domain
	fqdnAnnotation = "tunnels.networking.cfargotunnel.com/fqdn"
	// If this annotation is set to false, do not limit searching Tunnel to Ingress namespace, and pick the 1st one found (Might be random?)
	// If set to anything other than false, use it as a namspace where Tunnel exists
	tunnelNSAnnotation = "tunnels.networking.cfargotunnel.com/ns"

	// Protocol to use between cloudflared and the Ingress. Defaults to HTTPS. Allowed values are in tunnelValidProtoMap (http, https, tcp, udp)
	tunnelProtoAnnotation = "tunnels.networking.cfargotunnel.com/proto"
	defaultTunnelProto    = "https"

	// Checksum of the config, used to restart pods in the deployment
	tunnelConfigChecksum = "tunnels.networking.cfargotunnel.com/checksum"

	tunnelFinalizerAnnotation = "tunnels.networking.cfargotunnel.com/finalizer"
	tunnelDomainAnnotation    = "tunnels.networking.cfargotunnel.com/domain"
	configmapKey              = "config.yaml"
)

var tunnelValidProtoMap map[string]bool = map[string]bool{
	"http":  true,
	"https": true,
	"tcp":   true,
	"udp":   true,
}

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;update;patch

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch Ingress from API
	ingress := &networkingv1.Ingress{}

	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		if apierrors.IsNotFound(err) {
			// Ingress object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Ingress deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Ingress")
		return ctrl.Result{}, err
	}

	// Read Ingress annotations. If both annotations are not set, return without doing anything
	tunnelName, okName := ingress.Annotations[tunnelNameAnnotation]
	tunnelId, okId := ingress.Annotations[tunnelIdAnnotation]
	fqdn := ingress.Annotations[fqdnAnnotation]
	tunnelNS, okNS := ingress.Annotations[tunnelNSAnnotation]
	tunnelCRD, okCRD := ingress.Annotations[tunnelCRAnnotation]

	if !(okCRD || okName || okId) {
		// If an ingress with annotation is edited to remove just annotations, cleanup wont happen.
		// Not an issue as such, since it will be overwritten the next time it is used.
		log.Info("No related annotations not found, skipping Ingress")
		// Check if our finalizer is present on a non managed resource and remove it. This can happen if annotations were removed from the Ingress.
		if controllerutil.ContainsFinalizer(ingress, tunnelFinalizerAnnotation) {
			log.Info("Finalizer found on unmanaged Ingress, removing it")
			controllerutil.RemoveFinalizer(ingress, tunnelFinalizerAnnotation)
			err := r.Update(ctx, ingress)
			if err != nil {
				log.Error(err, "unable to remove finalizer from unmanaged Ingress")
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
		labels[tunnelNSAnnotation] = ingress.Namespace
		listOpts = append(listOpts, client.InNamespace(ingress.Namespace))
	} else if okNS && tunnelNS != "false" {
		labels[tunnelNSAnnotation] = tunnelNS
		listOpts = append(listOpts, client.InNamespace(tunnelNS))
	} // else, no filter on namespace, pick the 1st one

	listOpts = append(listOpts, client.MatchingLabels(labels))

	log.Info("setting tunnel", "listOpts", listOpts)

	// Check if Ingress is marked for deletion
	if ingress.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(ingress, tunnelFinalizerAnnotation) {
			// Run finalization logic. If the finalization logic fails,
			// don't remove the finalizer so that we can retry during the next reconciliation.

			if err := r.configureCloudflare(log, ctx, ingress, fqdn, listOpts, true); err != nil {
				return ctrl.Result{}, err
			}

			// Remove tunnelFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(ingress, tunnelFinalizerAnnotation)
			err := r.Update(ctx, ingress)
			if err != nil {
				log.Error(err, "unable to continue with Ingress deletion")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Add finalizer for Ingress
		if !controllerutil.ContainsFinalizer(ingress, tunnelFinalizerAnnotation) {
			controllerutil.AddFinalizer(ingress, tunnelFinalizerAnnotation)
			if err := r.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Configure ConfigMap
		if err := r.configureCloudflare(log, ctx, ingress, fqdn, listOpts, false); err != nil {
			log.Error(err, "unable to configure ConfigMap", "key", configmapKey)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *IngressReconciler) getConfigMapConfiguration(ctx context.Context, log logr.Logger, listOpts []client.ListOption) (corev1.ConfigMap, Configuration, error) {
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

func (r *IngressReconciler) setConfigMapConfiguration(ctx context.Context, log logr.Logger, configmap corev1.ConfigMap, config Configuration) error {
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

func (r *IngressReconciler) configureCloudflare(log logr.Logger, ctx context.Context, ingress *networkingv1.Ingress, fqdn string, listOpts []client.ListOption, cleanup bool) error {
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

	ingressProto := defaultTunnelProto
	tunnelProto := ingress.Annotations[tunnelProtoAnnotation]
	if tunnelProto != "" && tunnelValidProtoMap[tunnelProto] {
		ingressProto = tunnelProto
	}

	var finalIngress []UnvalidatedIngressRule
	if cleanup {
		finalIngress = make([]UnvalidatedIngressRule, 0, len(config.Ingress))
	}
	// Loop through the Ingress rules
	for _, rule := range ingress.Spec.Rules {
		ingressSpecHost := fmt.Sprintf("%s://%s", ingressProto, rule.Host)

		// Generate fqdn string from Ingress Spec if not provided
		if fqdn == "" {
			ingressHost := strings.Split(rule.Host, ".")[0]
			fqdn = fmt.Sprintf("%s.%s", ingressHost, tunnelDomain)
			log.Info("using default domain value", "domain", tunnelDomain)
		}
		log.Info("setting fqdn", "fqdn", fqdn)

		// Find if the host already exists in config. If so, modify
		found := false
		for i, v := range config.Ingress {
			if cleanup {
				// TODO: Enhance this logic
				if v.Hostname != fqdn && v.Service != ingressSpecHost {
					finalIngress = append(finalIngress, v)
				}
			} else if v.Hostname == fqdn {
				log.Info("found existing ingress for host, modifying the service", "service", ingressSpecHost)
				config.Ingress[i].Service = ingressSpecHost
				found = true
				break
			}
		}

		// Else add a new entry to the beginning. The last entry has to be the 404 entry
		if !cleanup && !found {
			log.Info("adding ingress for host to point to service", "service", ingressSpecHost)
			config.Ingress = append([]UnvalidatedIngressRule{{
				Hostname: fqdn,
				Service:  ingressSpecHost,
			}}, config.Ingress...)
		}
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
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(r)
}
