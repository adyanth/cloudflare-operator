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
	"k8s.io/client-go/tools/record"
)

// TunnelReconciler reconciles a Tunnel object
type TunnelReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// Custom data for ease of (re)use

	ctx         context.Context
	log         logr.Logger
	tunnel      *networkingv1alpha1.Tunnel
	cfAPI       *CloudflareAPI
	cfSecret    *corev1.Secret
	tunnelCreds string
}

// labelsForTunnel returns the labels for selecting the resources
// belonging to the given Tunnel CR name.
func labelsForTunnel(cf networkingv1alpha1.Tunnel) map[string]string {
	return map[string]string{
		tunnelAnnotation:          cf.Name,
		tunnelAppAnnotation:       "cloudflared",
		tunnelIdAnnotation:        cf.Status.TunnelId,
		tunnelNameAnnotation:      cf.Status.TunnelName,
		tunnelDomainAnnotation:    cf.Spec.Cloudflare.Domain,
		isClusterTunnelAnnotation: "false",
	}
}

func (r *TunnelReconciler) initStruct(ctx context.Context, tunnel *networkingv1alpha1.Tunnel) error {
	r.ctx = ctx
	r.tunnel = tunnel

	var err error

	if r.cfAPI, r.cfSecret, err = getAPIDetails(r.ctx, r.Client, r.log, r.tunnel.Spec, r.tunnel.Status, r.tunnel.Namespace); err != nil {
		r.log.Error(err, "unable to get API details")
		r.Recorder.Event(tunnel, corev1.EventTypeWarning, "ErrSpecSecret", "Error reading Secret to configure API")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=tunnels/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *TunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx)

	// Lookup the Tunnel resource
	tunnel := &networkingv1alpha1.Tunnel{}
	if err := r.Get(ctx, req.NamespacedName, tunnel); err != nil {
		if apierrors.IsNotFound(err) {
			// Tunnel object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Info("Tunnel deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		r.log.Error(err, "unable to fetch Tunnel")
		return ctrl.Result{}, err
	}

	if err := r.initStruct(ctx, tunnel); err != nil {
		return ctrl.Result{}, err
	}

	if res, ok, err := r.setupTunnel(); !ok {
		return res, err
	}

	// Update status
	if err := r.updateTunnelStatus(); err != nil {
		return ctrl.Result{}, err
	}

	// Create necessary resources
	if res, ok, err := r.createManagedResources(); !ok {
		return res, err
	}

	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) setupTunnel() (ctrl.Result, bool, error) {
	okNewTunnel := r.tunnel.Spec.NewTunnel != networkingv1alpha1.NewTunnel{}
	okExistingTunnel := r.tunnel.Spec.ExistingTunnel != networkingv1alpha1.ExistingTunnel{}

	// If both are set (or neither are), we have a problem
	if okNewTunnel == okExistingTunnel {
		err := fmt.Errorf("spec ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		r.log.Error(err, "spec ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "ErrSpecTunnel", "ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		return ctrl.Result{}, false, err
	}

	if okExistingTunnel {
		// Existing Tunnel, Set tunnelId in status and get creds file
		if err := r.setupExistingTunnel(); err != nil {
			return ctrl.Result{}, false, err
		}
	} else {
		// New tunnel, finalizer/cleanup logic + creation
		if r.tunnel.GetDeletionTimestamp() != nil {
			if res, ok, err := r.cleanupTunnel(); !ok {
				return res, false, err
			}
		} else {
			if err := r.setupNewTunnel(); err != nil {
				return ctrl.Result{}, false, err
			}
		}
	}

	return ctrl.Result{}, true, nil
}

func (r *TunnelReconciler) setupExistingTunnel() error {
	r.cfAPI.TunnelName = r.tunnel.Spec.ExistingTunnel.Name
	r.cfAPI.TunnelId = r.tunnel.Spec.ExistingTunnel.Id

	// Read secret for credentials file
	cfCredFileB64, okCredFile := r.cfSecret.Data[r.tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE]
	cfSecretB64, okSecret := r.cfSecret.Data[r.tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET]

	if !okCredFile && !okSecret {
		err := fmt.Errorf("neither key not found in secret")
		r.log.Error(err, "neither key not found in secret", "secret", r.tunnel.Spec.Cloudflare.Secret, "key1", r.tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE, "key2", r.tunnel.Spec.Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET)
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "ErrSpecSecret", "Neither Key found in Secret")
		return err
	}

	if okCredFile {
		r.tunnelCreds = string(cfCredFileB64)
	} else {
		creds, err := r.cfAPI.GetTunnelCreds(string(cfSecretB64))
		if err != nil {
			r.log.Error(err, "error getting tunnel credentials from secret")
			r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "ErrSpecApi", "Error in getting Tunnel Credentials from Secret")
			return err
		}
		r.tunnelCreds = creds
	}

	return nil
}

func (r *TunnelReconciler) setupNewTunnel() error {
	// New tunnel, not yet setup, create on Cloudflare
	if r.tunnel.Status.TunnelId == "" {
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Creating", "Tunnel is being created")
		r.cfAPI.TunnelName = r.tunnel.Spec.NewTunnel.Name
		_, creds, err := r.cfAPI.CreateCloudflareTunnel()
		if err != nil {
			r.log.Error(err, "unable to create Tunnel")
			r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedCreate", "Unable to create Tunnel on Cloudflare")
			return err
		}
		r.log.Info("Tunnel created on Cloudflare")
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Created", "Tunnel created successfully on Cloudflare")
		r.tunnelCreds = creds
	}

	// Add finalizer for tunnel
	if !controllerutil.ContainsFinalizer(r.tunnel, tunnelFinalizerAnnotation) {
		controllerutil.AddFinalizer(r.tunnel, tunnelFinalizerAnnotation)
		if err := r.Update(r.ctx, r.tunnel); err != nil {
			r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "FailedFinalizerSet", "Failed to add Tunnel Finalizer")
			return err
		}
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "FinalizerSet", "Tunnel Finalizer added")
	}
	return nil
}

func (r *TunnelReconciler) cleanupTunnel() (ctrl.Result, bool, error) {
	if controllerutil.ContainsFinalizer(r.tunnel, tunnelFinalizerAnnotation) {
		// Run finalization logic. If the finalization logic fails,
		// don't remove the finalizer so that we can retry during the next reconciliation.

		r.log.Info("starting deletion cycle", "size", r.tunnel.Spec.Size)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Deleting", "Starting Tunnel Deletion")
		cfDeployment := &appsv1.Deployment{}
		var bypass bool
		if err := r.Get(r.ctx, apitypes.NamespacedName{Name: r.tunnel.Name, Namespace: r.tunnel.Namespace}, cfDeployment); err != nil {
			r.log.Error(err, "Error in getting deployments, might already be deleted?")
			bypass = true
		}
		if !bypass && *cfDeployment.Spec.Replicas != 0 {
			r.log.Info("Scaling down cloudflared")
			r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Scaling", "Scaling down cloudflared")
			var size int32 = 0
			cfDeployment.Spec.Replicas = &size
			if err := r.Update(r.ctx, cfDeployment); err != nil {
				r.log.Error(err, "Failed to update Deployment", "Deployment.Namespace", cfDeployment.Namespace, "Deployment.Name", cfDeployment.Name)
				r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedScaling", "Failed to scale down cloudflared")
				return ctrl.Result{}, false, err
			}
			r.log.Info("Scaling down successful", "size", r.tunnel.Spec.Size)
			r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Scaled", "Scaling down cloudflared successful")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, false, nil
		}
		if bypass || *cfDeployment.Spec.Replicas == 0 {
			if err := r.cfAPI.DeleteCloudflareTunnel(); err != nil {
				r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedDeleting", "Tunnel deletion failed")
				return ctrl.Result{}, false, err
			}
			r.log.Info("Tunnel deleted", "tunnelID", r.tunnel.Status.TunnelId)
			r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Deleted", "Tunnel deletion successful")

			// Remove tunnelFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(r.tunnel, tunnelFinalizerAnnotation)
			err := r.Update(r.ctx, r.tunnel)
			if err != nil {
				r.log.Error(err, "unable to continue with tunnel deletion")
				r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedFinalizerUnset", "Unable to remove Tunnel Finalizer")
				return ctrl.Result{}, false, err
			}
			r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "FinalizerUnset", "Tunnel Finalizer removed")
			return ctrl.Result{}, true, nil
		}
	}
	return ctrl.Result{}, true, nil
}

func (r *TunnelReconciler) updateTunnelStatus() error {
	if r.tunnel.Labels == nil {
		r.tunnel.Labels = make(map[string]string)
	}
	for k, v := range labelsForTunnel(*r.tunnel) {
		r.tunnel.Labels[k] = v
	}
	if err := r.Update(r.ctx, r.tunnel); err != nil {
		return err
	}

	if err := r.cfAPI.ValidateAll(); err != nil {
		r.log.Error(err, "Failed to validate API credentials")
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "ErrSpecApi", "Error validating Cloudflare API credentials")
		return err
	}
	r.tunnel.Status.AccountId = r.cfAPI.ValidAccountId
	r.tunnel.Status.TunnelId = r.cfAPI.ValidTunnelId
	r.tunnel.Status.TunnelName = r.cfAPI.ValidTunnelName
	r.tunnel.Status.ZoneId = r.cfAPI.ValidZoneId
	if err := r.Status().Update(r.ctx, r.tunnel); err != nil {
		r.log.Error(err, "Failed to update Tunnel status", "Tunnel.Namespace", r.tunnel.Namespace, "Tunnel.Name", r.tunnel.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedStatusSet", "Failed to set Tunnel status required for operation")
		return err
	}
	r.log.Info("Tunnel status is set", "status", r.tunnel.Status)
	return nil
}

func (r *TunnelReconciler) createManagedSecret() error {
	managedSecret := &corev1.Secret{}
	if err := r.Get(r.ctx, apitypes.NamespacedName{Name: r.tunnel.Name, Namespace: r.tunnel.Namespace}, managedSecret); err != nil && apierrors.IsNotFound(err) {
		// Define a new Secret
		sec := r.secretForTunnel()
		r.log.Info("Creating a new Secret", "Secret.Namespace", sec.Namespace, "Secret.Name", sec.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "CreatingSecret", "Creating Tunnel Secret")
		err = r.Create(r.ctx, sec)
		if err != nil {
			r.log.Error(err, "Failed to create new Secret", "Deployment.Namespace", sec.Namespace, "Deployment.Name", sec.Name)
			r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedCreatingSecret", "Creating Tunnel Secret failed")
			return err
		}
		r.log.Info("Secret created", "Secret.Namespace", sec.Namespace, "Secret.Name", sec.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "CreatedSecret", "Created Tunnel Secret")
	} else if err != nil {
		r.log.Error(err, "Failed to get Secret")
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedCreatedSecret", "Reading Tunnel Secret failed")
		return err
	}
	return nil
}

func (r *TunnelReconciler) createManagedConfigMap() error {
	cfConfigMap := &corev1.ConfigMap{}
	if err := r.Get(r.ctx, apitypes.NamespacedName{Name: r.tunnel.Name, Namespace: r.tunnel.Namespace}, cfConfigMap); err != nil && apierrors.IsNotFound(err) {
		// Define a new ConfigMap
		cm := r.configMapForTunnel()
		r.log.Info("Creating a new ConfigMap", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Configuring", "Creating Tunnel ConfigMap")
		err = r.Create(r.ctx, cm)
		if err != nil {
			r.log.Error(err, "Failed to create new ConfigMap", "Deployment.Namespace", cm.Namespace, "Deployment.Name", cm.Name)
			r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedConfiguring", "Creating Tunnel ConfigMap failed")
			return err
		}
		r.log.Info("ConfigMap created", "ConfigMap.Namespace", cm.Namespace, "ConfigMap.Name", cm.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Configured", "Created Tunnel ConfigMap")
	} else if err != nil {
		r.log.Error(err, "Failed to get ConfigMap")
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedConfigured", "Reading Tunnel ConfigMap failed")
		return err
	}
	return nil
}

func (r *TunnelReconciler) createOrScaleManagedDeployment() (ctrl.Result, bool, error) {
	// Check if Deployment already exists, else create it
	cfDeployment := &appsv1.Deployment{}
	if res, err := r.createManagedDeployment(cfDeployment); err != nil || (res != ctrl.Result{}) {
		return res, false, err
	}

	// Ensure the Deployment size is the same as the spec
	if res, err := r.scaleManagedDeployment(cfDeployment); err != nil || (res != ctrl.Result{}) {
		return res, false, err
	}

	return ctrl.Result{}, true, nil
}

func (r *TunnelReconciler) createManagedDeployment(cfDeployment *appsv1.Deployment) (ctrl.Result, error) {
	if err := r.Get(r.ctx, apitypes.NamespacedName{Name: r.tunnel.Name, Namespace: r.tunnel.Namespace}, cfDeployment); err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForTunnel()
		r.log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Deploying", "Creating Tunnel Deployment")
		err = r.Create(r.ctx, dep)
		if err != nil {
			r.log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedDeploying", "Creating Tunnel Deployment failed")
			return ctrl.Result{}, err
		}
		r.log.Info("Deployment created", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Deployed", "Created Tunnel Deployment")
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		r.log.Error(err, "Failed to get Deployment")
		r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedDeployed", "Reading Tunnel Deployment failed")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) scaleManagedDeployment(cfDeployment *appsv1.Deployment) (ctrl.Result, error) {
	size := r.tunnel.Spec.Size
	if *cfDeployment.Spec.Replicas != size {
		r.log.Info("Updating deployment", "currentReplica", *cfDeployment.Spec.Replicas, "desiredSize", size)
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Scaling", "Scaling Tunnel Deployment")
		cfDeployment.Spec.Replicas = &size
		if err := r.Update(r.ctx, cfDeployment); err != nil {
			r.log.Error(err, "Failed to update Deployment", "Deployment.Namespace", cfDeployment.Namespace, "Deployment.Name", cfDeployment.Name)
			r.Recorder.Event(r.tunnel, corev1.EventTypeWarning, "FailedScaling", "Failed to scale Tunnel Deployment")
			return ctrl.Result{}, err
		}
		r.log.Info("Deployment updated")
		r.Recorder.Event(r.tunnel, corev1.EventTypeNormal, "Scaled", "Scaled Tunnel Deployment")
		// Ask to requeue after 1 minute in order to give enough time for the
		// pods be created on the cluster side and the operand be able
		// to do the next update step accurately.
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}
	return ctrl.Result{}, nil
}

func (r *TunnelReconciler) createManagedResources() (ctrl.Result, bool, error) {
	// Check if Secret already exists, else create it
	if err := r.createManagedSecret(); err != nil {
		return ctrl.Result{}, false, err
	}

	// Check if ConfigMap already exists, else create it
	if err := r.createManagedConfigMap(); err != nil {
		return ctrl.Result{}, false, err
	}

	// Create Deployment if does not exist and scale it
	if res, ok, err := r.createOrScaleManagedDeployment(); !ok {
		return res, false, err
	}

	return ctrl.Result{}, true, nil
}

// configMapForTunnel returns a tunnel ConfigMap object
func (r *TunnelReconciler) configMapForTunnel() *corev1.ConfigMap {
	ls := labelsForTunnel(*r.tunnel)
	initialConfigBytes, _ := yaml.Marshal(Configuration{
		TunnelId:     r.tunnel.Status.TunnelId,
		SourceFile:   "/etc/cloudflared/creds/credentials.json",
		Metrics:      "0.0.0.0:2000",
		NoAutoUpdate: true,
		Ingress: []UnvalidatedIngressRule{{
			Service: r.tunnel.Spec.FallbackTarget,
		}},
	})

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.tunnel.Name,
			Namespace: r.tunnel.Namespace,
			Labels:    ls,
		},
		Data: map[string]string{"config.yaml": string(initialConfigBytes)},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(r.tunnel, cm, r.Scheme)
	return cm
}

// secretForTunnel returns a tunnel Secret object
func (r *TunnelReconciler) secretForTunnel() *corev1.Secret {
	ls := labelsForTunnel(*r.tunnel)
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.tunnel.Name,
			Namespace: r.tunnel.Namespace,
			Labels:    ls,
		},
		StringData: map[string]string{"credentials.json": r.tunnelCreds},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(r.tunnel, sec, r.Scheme)
	return sec
}

// deploymentForTunnel returns a tunnel Deployment object
func (r *TunnelReconciler) deploymentForTunnel() *appsv1.Deployment {
	ls := labelsForTunnel(*r.tunnel)
	replicas := r.tunnel.Spec.Size

	args := []string{"tunnel", "--config", "/etc/cloudflared/config/config.yaml", "run"}
	volumes := []corev1.Volume{{
		Name: "creds",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: r.tunnel.Name},
		},
	}, {
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: r.tunnel.Name},
				Items: []corev1.KeyToPath{{
					Key:  "config.yaml",
					Path: "config.yaml",
				}},
			},
		},
	}}
	volumeMounts := []corev1.VolumeMount{{
		Name:      "config",
		MountPath: "/etc/cloudflared/config",
		ReadOnly:  true,
	}, {
		Name:      "creds",
		MountPath: "/etc/cloudflared/creds",
		ReadOnly:  true,
	}}
	if r.tunnel.Spec.NoTlsVerify {
		args = append(args, "--no-tls-verify")
	}
	if r.tunnel.Spec.OriginCaPool != "" {
		args = append(args, "--origin-ca-pool", "/etc/cloudflared/certs/tls.crt")
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "certs",
			MountPath: "/etc/cloudflared/certs",
			ReadOnly:  true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: r.tunnel.Spec.OriginCaPool},
			},
		})
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.tunnel.Name,
			Namespace: r.tunnel.Namespace,
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
						Image: r.tunnel.Spec.Image,
						Name:  "cloudflared",
						Args:  args,
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
						VolumeMounts: volumeMounts,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{"memory": resource.MustParse("30Mi"), "cpu": resource.MustParse("10m")},
							Limits:   corev1.ResourceList{"memory": resource.MustParse("256Mi"), "cpu": resource.MustParse("500m")},
						},
					}},
					Volumes: volumes,
				},
			},
		},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(r.tunnel, dep, r.Scheme)
	return dep
}

// SetupWithManager sets up the controller with the Manager.
func (r *TunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("cloudflare-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Tunnel{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
