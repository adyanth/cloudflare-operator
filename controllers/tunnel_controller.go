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
	"time"

	yaml "gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
)

// TunnelReconciler reconciles a Tunnel object
type TunnelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// labelsForTunnel returns the labels for selecting the resources
// belonging to the given Tunnel CR name.
func labelsForTunnel(cf networkingv1alpha1.Tunnel) map[string]string {
	return map[string]string{
		"tunnels.networking.cfargotunnel.com/cr":     cf.Name,
		"tunnels.networking.cfargotunnel.com/app":    "cloudflared",
		"tunnels.networking.cfargotunnel.com/id":     cf.Status.TunnelId,
		"tunnels.networking.cfargotunnel.com/ns":     cf.Namespace,
		"tunnels.networking.cfargotunnel.com/name":   cf.Status.TunnelName,
		"tunnels.networking.cfargotunnel.com/domain": cf.Spec.Cloudflare.Domain,
	}
}

func getAPIDetails(c client.Client, ctx context.Context, log logr.Logger, tunnel networkingv1alpha1.Tunnel) (*CloudflarAPI, *corev1.Secret, error) {

	// Get secret containing API token
	cfCloudflareSecret := &corev1.Secret{}
	if err := c.Get(ctx, apitypes.NamespacedName{Name: tunnel.Spec.Cloudflare.Secret, Namespace: tunnel.Namespace}, cfCloudflareSecret); err != nil {
		log.Error(err, "secret not found", "secret", tunnel.Spec.Cloudflare.Secret)
		return &CloudflarAPI{}, &corev1.Secret{}, err
	}

	// Read secret for API Token
	cfAPITokenB64, ok := cfCloudflareSecret.Data[tunnel.Spec.Cloudflare.CLOUDFLARE_API_TOKEN]
	if !ok {
		log.Info("key not found in secret", "secret", tunnel.Spec.Cloudflare.Secret, "key", tunnel.Spec.Cloudflare.CLOUDFLARE_API_TOKEN)
	}

	// Read secret for API Key
	cfAPIKeyB64, ok := cfCloudflareSecret.Data[tunnel.Spec.Cloudflare.CLOUDFLARE_API_KEY]
	if !ok {
		log.Info("key not found in secret", "secret", tunnel.Spec.Cloudflare.Secret, "key", tunnel.Spec.Cloudflare.CLOUDFLARE_API_KEY)
	}

	cfAPI := &CloudflarAPI{
		Log:             log,
		AccountName:     tunnel.Spec.Cloudflare.AccountName,
		AccountId:       tunnel.Spec.Cloudflare.AccountId,
		Domain:          tunnel.Spec.Cloudflare.Domain,
		APIToken:        string(cfAPITokenB64),
		APIKey:          string(cfAPIKeyB64),
		APIEmail:        tunnel.Spec.Cloudflare.Email,
		ValidAccountId:  tunnel.Status.AccountId,
		ValidTunnelId:   tunnel.Status.TunnelId,
		ValidTunnelName: tunnel.Status.TunnelName,
		ValidZoneId:     tunnel.Status.ZoneId,
	}
	return cfAPI, cfCloudflareSecret, nil
}

//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	// Lookup the Tunnel resource
	tunnel := &networkingv1alpha1.Tunnel{}
	if err := r.Get(ctx, req.NamespacedName, tunnel); err != nil {
		if apierrors.IsNotFound(err) {
			// Tunnel object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("Tunnel deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		log.Error(err, "unable to fetch Tunnel")
		return ctrl.Result{}, err
	}

	// Creds is the tunnel credential JSON
	var tunnelCreds string

	cfAPI, cfCloudflareSecret, err := getAPIDetails(r.Client, ctx, log, *tunnel)
	if err != nil {
		log.Error(err, "error while getting API details")
		return ctrl.Result{}, err
	}

	okNewTunnel := tunnel.Spec.NewTunnel != networkingv1alpha1.NewTunnel{}
	okExistingTunnel := tunnel.Spec.ExistingTunnel != networkingv1alpha1.ExistingTunnel{}

	if okNewTunnel == okExistingTunnel {
		err := fmt.Errorf("spec ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		log.Error(err, "spec ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		return ctrl.Result{}, err
	}

	// Set tunnelId in status and get creds file
	if okExistingTunnel {
		cfAPI.TunnelName = tunnel.Spec.ExistingTunnel.Name
		cfAPI.TunnelId = tunnel.Spec.ExistingTunnel.Id

		// Read secret for credentials file
		cfCredFileB64, okCredFile := cfCloudflareSecret.Data[tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE]
		cfSecretB64, okSecret := cfCloudflareSecret.Data[tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET]

		if !okCredFile && !okSecret {
			err := fmt.Errorf("neither key not found in secret")
			log.Error(err, "neither key not found in secret", "secret", tunnel.Spec.Cloudflare.Secret, "key1", tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE, "key2", tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET)
			return ctrl.Result{}, err
		}

		if okCredFile {
			tunnelCreds = string(cfCredFileB64)
		} else {
			creds, err := cfAPI.GetTunnelCreds(string(cfSecretB64))
			if err != nil {
				log.Error(err, "error getting tunnel credentials from secret")
				return ctrl.Result{}, err
			}
			tunnelCreds = creds
		}
	}

	// New tunnel, create on Cloudflare
	if okNewTunnel && tunnel.Status.TunnelId == "" {
		cfAPI.TunnelName = tunnel.Spec.NewTunnel.Name
		_, creds, err := cfAPI.CreateCloudflareTunnel()
		if err != nil {
			log.Error(err, "unable to create Tunnel")
			return ctrl.Result{}, err
		}
		log.Info("Tunnel created on Cloudflare")
		tunnelCreds = creds
	}

	// Check if Tunnel is marked for deletion if new tunnel
	if okNewTunnel {
		if tunnel.GetDeletionTimestamp() != nil {
			if controllerutil.ContainsFinalizer(tunnel, tunnelFinalizerAnnotation) {
				// Run finalization logic. If the finalization logic fails,
				// don't remove the finalizer so that we can retry during the next reconciliation.

				log.Info("starting deletion cycle", "size", tunnel.Spec.Size)
				cfDeployment := &appsv1.Deployment{}
				var bypass bool
				if err := r.Get(ctx, apitypes.NamespacedName{Name: tunnel.Name, Namespace: tunnel.Namespace}, cfDeployment); err != nil {
					log.Error(err, "Error in getting deployments, might already be deleted?")
					bypass = true
				}
				if *cfDeployment.Spec.Replicas != 0 {
					log.Info("Scaling down cloudflared")
					var size int32 = 0
					cfDeployment.Spec.Replicas = &size
					if err := r.Update(ctx, cfDeployment); err != nil {
						log.Error(err, "Failed to update Deployment", "Deployment.Namespace", cfDeployment.Namespace, "Deployment.Name", cfDeployment.Name)
						return ctrl.Result{}, err
					}
					log.Info("Scaling down successful", "size", tunnel.Spec.Size)
					return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
				}
				if bypass || *cfDeployment.Spec.Replicas == 0 {
					if err := cfAPI.DeleteCloudflareTunnel(); err != nil {
						return ctrl.Result{}, err
					}
					log.Info("Tunnel deleted", "tunnelID", tunnel.Status.TunnelId)

					// Remove tunnelFinalizer. Once all finalizers have been
					// removed, the object will be deleted.
					controllerutil.RemoveFinalizer(tunnel, tunnelFinalizerAnnotation)
					err := r.Update(ctx, tunnel)
					if err != nil {
						log.Error(err, "unable to continue with tunnel deletion")
						return ctrl.Result{}, err
					}
					return ctrl.Result{}, nil
				}
			}
		} else {
			// Add finalizer for tunnel
			if !controllerutil.ContainsFinalizer(tunnel, tunnelFinalizerAnnotation) {
				controllerutil.AddFinalizer(tunnel, tunnelFinalizerAnnotation)
				if err := r.Update(ctx, tunnel); err != nil {
					return ctrl.Result{}, err
				}
			}
		}
	}

	tunnel.Labels = labelsForTunnel(*tunnel)
	if err := r.Update(ctx, tunnel); err != nil {
		return ctrl.Result{}, err
	}

	// Update status
	if err := cfAPI.ValidateAll(); err != nil {
		log.Error(err, "Failed to update Tunnel status", "Tunnel.Namespace", tunnel.Namespace, "Tunnel.Name", tunnel.Name)
		return ctrl.Result{}, err
	}
	tunnel.Status.AccountId = cfAPI.ValidAccountId
	tunnel.Status.TunnelId = cfAPI.ValidTunnelId
	tunnel.Status.TunnelName = cfAPI.ValidTunnelName
	tunnel.Status.ZoneId = cfAPI.ValidZoneId
	if err := r.Status().Update(ctx, tunnel); err != nil {
		log.Error(err, "Failed to update Tunnel status", "Tunnel.Namespace", tunnel.Namespace, "Tunnel.Name", tunnel.Name)
		return ctrl.Result{}, err
	}
	log.Info("Tunnel status is set", "status", tunnel.Status)

	// Check if Secret already exists
	cfSecret := &corev1.Secret{}
	if err := r.Get(ctx, apitypes.NamespacedName{Name: tunnel.Name, Namespace: tunnel.Namespace}, cfSecret); err != nil && apierrors.IsNotFound(err) {
		// Define a new Secret
		sec := r.secretForTunnel(tunnel, tunnelCreds)
		log.Info("Creating a new Secret", "Secret.Namespace", sec.Namespace, "Secret.Name", sec.Name)
		err = r.Create(ctx, sec)
		if err != nil {
			log.Error(err, "Failed to create new Secret", "Deployment.Namespace", sec.Namespace, "Deployment.Name", sec.Name)
			return ctrl.Result{}, err
		}
		log.Info("Secret created", "Secret.Namespace", sec.Namespace, "Secret.Name", sec.Name)
	} else if err != nil {
		log.Error(err, "Failed to get Secret")
		return ctrl.Result{}, err
	}

	// Check if ConfigMap already exists
	cfConfigMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, apitypes.NamespacedName{Name: tunnel.Name, Namespace: tunnel.Namespace}, cfConfigMap); err != nil && apierrors.IsNotFound(err) {
		// Define a new ConfigMap
		cm := r.configMapForTunnel(tunnel)
		log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		err = r.Create(ctx, cm)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap", "Deployment.Namespace", cm.Namespace, "Deployment.Name", cm.Name)
			return ctrl.Result{}, err
		}
		log.Info("ConfigMap created", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	}

	// Check if Deployment already exists
	cfDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, apitypes.NamespacedName{Name: tunnel.Name, Namespace: tunnel.Namespace}, cfDeployment); err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForTunnel(tunnel)
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		log.Info("Deployment created", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Ensure the Deployment size is the same as the spec
	size := tunnel.Spec.Size
	if *cfDeployment.Spec.Replicas != size {
		log.Info("Updating deployment", "currentReplica", *cfDeployment.Spec.Replicas, "desiredSize", size)
		cfDeployment.Spec.Replicas = &size
		if err := r.Update(ctx, cfDeployment); err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", cfDeployment.Namespace, "Deployment.Name", cfDeployment.Name)
			return ctrl.Result{}, err
		}
		log.Info("Deployment updated")
		// Ask to requeue after 1 minute in order to give enough time for the
		// pods be created on the cluster side and the operand be able
		// to do the next update step accurately.
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	return ctrl.Result{}, nil
}

// configMapForTunnel returns a tunnel ConfigMap object
func (r *TunnelReconciler) configMapForTunnel(cfTunnel *networkingv1alpha1.Tunnel) *corev1.ConfigMap {
	ls := labelsForTunnel(*cfTunnel)
	initialConfigBytes, _ := yaml.Marshal(Configuration{
		TunnelId:     cfTunnel.Status.TunnelId,
		SourceFile:   "/etc/cloudflared/creds/credentials.json",
		Metrics:      "0.0.0.0:2000",
		NoAutoUpdate: true,
		Ingress: []UnvalidatedIngressRule{{
			Service: "http_status:404",
		}},
	})

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfTunnel.Name,
			Namespace: cfTunnel.Namespace,
			Labels:    ls,
		},
		Data: map[string]string{"config.yaml": string(initialConfigBytes)},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(cfTunnel, cm, r.Scheme)
	return cm
}

// secretForTunnel returns a tunnel Secret object
func (r *TunnelReconciler) secretForTunnel(cfTunnel *networkingv1alpha1.Tunnel, tunnelCreds string) *corev1.Secret {
	ls := labelsForTunnel(*cfTunnel)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfTunnel.Name,
			Namespace: cfTunnel.Namespace,
			Labels:    ls,
		},
		StringData: map[string]string{"credentials.json": tunnelCreds},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(cfTunnel, sec, r.Scheme)
	return sec
}

// deploymentForTunnel returns a tunnel Deployment object
func (r *TunnelReconciler) deploymentForTunnel(cfTunnel *networkingv1alpha1.Tunnel) *appsv1.Deployment {
	ls := labelsForTunnel(*cfTunnel)
	replicas := cfTunnel.Spec.Size

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfTunnel.Name,
			Namespace: cfTunnel.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image: "cloudflare/cloudflared:2021.4.0",
						Name:  "cloudflared",
						Args:  []string{"tunnel", "--config", "/etc/cloudflared/config/config.yaml", "run"},
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/ready",
									Port: intstr.IntOrString{IntVal: 2000},
								},
							},
							FailureThreshold:    1,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "config",
							MountPath: "/etc/cloudflared/config",
							ReadOnly:  true,
						}, {
							Name:      "creds",
							MountPath: "/etc/cloudflared/creds",
							ReadOnly:  true,
						}},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{"memory": resource.MustParse("128Mi"), "cpu": resource.MustParse("500m")},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "creds",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{SecretName: cfTunnel.Name},
						},
					}, {
						Name: "config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: cfTunnel.Name},
								Items: []corev1.KeyToPath{{
									Key:  "config.yaml",
									Path: "config.yaml",
								}},
							},
						},
					}},
				},
			},
		},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(cfTunnel, dep, r.Scheme)
	return dep
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Tunnel{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
