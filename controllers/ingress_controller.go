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

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

func (r *IngressReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Fetch Ingress from API
	var ingress networkingv1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if apierrors.IsNotFound(err) {
			// Ingress is deleted
			// TODO: Handle deletion from configmap
			log.Info("not handling cleanup for deleting Ingress")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Ingress")
		return ctrl.Result{}, err
	}

	// Read Ingress annotations. If both annotations are not set, return without doing anything
	var tunnel, host string
	var ok bool
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

	// Fetch ConfigMap from API
	var configmap corev1.ConfigMap
	if err := r.Get(ctx, configmapNamespacedName, &configmap); err != nil {
		log.Error(err, "unable to fetch ConfigMap", "namespace", configmapNamespacedName.Namespace, "name", configmapNamespacedName.Name)
		return ctrl.Result{}, err
	}

	// Read ConfigMap YAML
	var configStr string
	if configStr, ok = configmap.Data[configmapKey]; !ok {
		err := fmt.Errorf("unable to find key `%s` in ConfigMap", configmapKey)
		log.Error(err, "unable to find key in ConfigMap", "key", configmapKey)
		return ctrl.Result{}, err
	}

	var config Configuration
	if err := yaml.Unmarshal([]byte(configStr), &config); err != nil {
		log.Error(err, "unable to read config as YAML")
		return ctrl.Result{}, err
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
			if v.Hostname == host {
				log.Info("found existing ingress for host, modifying the service", "service", ingressSpecHost)
				config.Ingress[i].Service = ingressSpecHost
				found = true
				break
			}
		}

		// Else add a new entry
		if !found {
			log.Info("adding ingress for host to point to service", "service", ingressSpecHost)
			config.Ingress = append(config.Ingress, UnvalidatedIngressRule{
				Hostname: host,
				Service:  ingressSpecHost,
			})
		}
	}

	// Push updated changesv
	if configBytes, err := yaml.Marshal(config); err == nil {
		configStr = string(configBytes)
	} else {
		log.Error(err, "unable to marshal config to ConfigMap", "key", configmapKey)
		return ctrl.Result{}, err
	}
	configmap.Data[configmapKey] = configStr
	if err := r.Update(ctx, &configmap); err != nil {
		if apierrors.IsConflict(err) {
			// The Ingress has been updated since we read it.
			// Requeue the Ingress to try to reconciliate again.
			return ctrl.Result{Requeue: true}, nil
		}
		log.Error(err, "unable to update ConfigMap")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Complete(r)
}
