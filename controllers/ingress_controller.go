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
	"fmt"
	"os"
	"strings"

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
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

const (
	cloudflareTunnelAnnotation = "cfargotunnel.com/tunnel"
	cloudflareHostAnnotation   = "cfargotunnel.com/host"
	cloudflareFinalizer        = "cfargotunnel.com/finalizer"
)

var (
	cloudflareDefaultTunnel string = os.Getenv("CLOUDFLARE_DEFAULT_TUNNEL")
	cloudflareDefaultDomain string = os.Getenv("CLOUDFLARE_DEFAULT_DOMAIN")
	configmapNamespace      string = getEnv("CLOUDFLARE_CONFIGMAP_NAMESPACE", "cloudflare")
	configmapName           string = getEnv("CLOUDFLARE_CONFIGMAP_NAME", "cloudflared")
	configmapKey            string = getEnv("CLOUDFLARE_CONFIGMAP_KEY", "config.yaml")
)

var configmapNamespacedName = apitypes.NamespacedName{
	Namespace: configmapNamespace,
	Name:      configmapName,
}

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch Ingress from API
	var tunnel, host string
	var ok bool
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
	if tunnel, ok = ingress.Annotations[cloudflareTunnelAnnotation]; !ok {
		if host, ok = ingress.Annotations[cloudflareHostAnnotation]; !ok {
			// If an ingress with annotation is edited to remove just annotations, cleanup wont happen.
			// Not an issue as such, since it will be overwritten the next time it is used.
			log.Info("annotation not found, skipping Ingress", "annotation", cloudflareTunnelAnnotation)
			return ctrl.Result{}, nil
		}
	}

	if tunnel == "" {
		tunnel = cloudflareDefaultTunnel
		log.Info("using default tunnel value", "tunnel", tunnel)
	}

	log.Info("setting tunnel", "tunnel", tunnel)

	// Check if Ingress is marked for deletion
	if ingress.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(ingress, cloudflareFinalizer) {
			// Run finalization logic. If the finalization logic fails,
			// don't remove the finalizer so that we can retry during the next reconciliation.

			if err := r.configureCloudflare(log, ctx, ingress, host, true); err != nil {
				return ctrl.Result{}, err
			}

			// Remove cloudflareFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(ingress, cloudflareFinalizer)
			err := r.Update(ctx, ingress)
			if err != nil {
				log.Error(err, "unable to continue with Ingress deletion")
				return ctrl.Result{}, err
			}
		}
	} else {
		// Add finalizer for Ingress
		if !controllerutil.ContainsFinalizer(ingress, cloudflareFinalizer) {
			controllerutil.AddFinalizer(ingress, cloudflareFinalizer)
			if err := r.Update(ctx, ingress); err != nil {
				return ctrl.Result{}, err
			}
		}
		// Configure ConfigMap
		if err := r.configureCloudflare(log, ctx, ingress, host, false); err != nil {
			log.Error(err, "unable to configure ConfigMap", "key", configmapKey)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *IngressReconciler) getConfigMapConfiguration(ctx context.Context, log logr.Logger) (corev1.ConfigMap, Configuration, error) {
	// Fetch ConfigMap from API
	var configmap corev1.ConfigMap
	var ok bool
	if err := r.Get(ctx, configmapNamespacedName, &configmap); err != nil {
		log.Error(err, "unable to fetch ConfigMap", "namespace", configmapNamespacedName.Namespace, "name", configmapNamespacedName.Name)
		return corev1.ConfigMap{}, Configuration{}, err
	}

	// Read ConfigMap YAML
	var configStr string
	if configStr, ok = configmap.Data[configmapKey]; !ok {
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
	// Push updated changesv
	var configStr string
	if configBytes, err := yaml.Marshal(config); err == nil {
		configStr = string(configBytes)
	} else {
		log.Error(err, "unable to marshal config to ConfigMap", "key", configmapKey)
		return err
	}
	configmap.Data[configmapKey] = configStr
	return r.Update(ctx, &configmap)
}

func (r *IngressReconciler) configureCloudflare(log logr.Logger, ctx context.Context, ingress *networkingv1.Ingress, host string, cleanup bool) error {
	var config Configuration
	var configmap corev1.ConfigMap
	var err error
	if configmap, config, err = r.getConfigMapConfiguration(ctx, log); err != nil {
		log.Error(err, "unable to get ConfigMap")
		return err
	}

	var finalIngress []UnvalidatedIngressRule
	if cleanup {
		finalIngress = make([]UnvalidatedIngressRule, 0, len(config.Ingress))
	}
	// Loop through the Ingress rules
	for _, rule := range ingress.Spec.Rules {
		ingressSpecHost := rule.Host

		// Generate host string from Ingress Spec if not provided
		if host == "" {
			ingressHost := strings.Split(ingressSpecHost, ".")[0]
			host = fmt.Sprintf("%s.%s", ingressHost, cloudflareDefaultDomain)
			log.Info("using default domain value", "domain", cloudflareDefaultDomain)
		}
		log.Info("setting host", "host", host)

		// Find if the host already exists in config. If so, modify
		found := false
		for i, v := range config.Ingress {
			if cleanup {
				if v.Hostname != host {
					finalIngress = append(finalIngress, v)
				}
			} else if v.Hostname == host {
				log.Info("found existing ingress for host, modifying the service", "service", ingressSpecHost)
				config.Ingress[i].Service = ingressSpecHost
				found = true
				break
			}
		}

		// Else add a new entry
		if !cleanup && !found {
			log.Info("adding ingress for host to point to service", "service", ingressSpecHost)
			config.Ingress = append(config.Ingress, UnvalidatedIngressRule{
				Hostname: host,
				Service:  ingressSpecHost,
			})
		}
	}

	if cleanup {
		if len(finalIngress) > 0 {
			config.Ingress = finalIngress
		} else {
			config.Ingress = nil
			log.Info("nothing left, setting config to nil")
		}
	}
	return r.setConfigMapConfiguration(ctx, log, configmap, config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(r)
}
