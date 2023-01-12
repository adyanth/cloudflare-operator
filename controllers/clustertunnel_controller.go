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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// ClusterTunnelReconciler reconciles a ClusterTunnel object
type ClusterTunnelReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	Namespace    string
	DefaultImage string

	// Custom data for ease of (re)use

	ctx         context.Context
	log         logr.Logger
	tunnel      Tunnel
	cfAPI       *CloudflareAPI
	cfSecret    *corev1.Secret
	tunnelCreds string
}

func (r *ClusterTunnelReconciler) GetClient() client.Client {
	return r.Client
}

func (r *ClusterTunnelReconciler) GetRecorder() record.EventRecorder {
	return r.Recorder
}

func (r *ClusterTunnelReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

func (r *ClusterTunnelReconciler) GetDefaultImage() string {
	return r.DefaultImage
}

func (r *ClusterTunnelReconciler) GetContext() context.Context {
	return r.ctx
}

func (r *ClusterTunnelReconciler) GetLog() logr.Logger {
	return r.log
}

func (r *ClusterTunnelReconciler) GetTunnel() Tunnel {
	return r.tunnel
}

func (r *ClusterTunnelReconciler) GetCfAPI() *CloudflareAPI {
	return r.cfAPI
}

func (r *ClusterTunnelReconciler) SetCfAPI(in *CloudflareAPI) {
	r.cfAPI = in
}

func (r *ClusterTunnelReconciler) GetCfSecret() *corev1.Secret {
	return r.cfSecret
}

func (r *ClusterTunnelReconciler) GetTunnelCreds() string {
	return r.tunnelCreds
}

func (r *ClusterTunnelReconciler) SetTunnelCreds(in string) {
	r.tunnelCreds = in
}

func (r *ClusterTunnelReconciler) initStruct(ctx context.Context, tunnel Tunnel) error {
	r.ctx = ctx
	r.tunnel = tunnel

	var err error

	if r.cfAPI, r.cfSecret, err = getAPIDetails(r.ctx, r.Client, r.log, r.tunnel.GetSpec(), r.tunnel.GetStatus(), r.tunnel.GetNamespace()); err != nil {
		r.log.Error(err, "unable to get API details")
		r.Recorder.Event(r.tunnel.GetObject(), corev1.EventTypeWarning, "ErrSpecSecret", "Error reading Secret to configure API")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=clustertunnels/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterTunnel object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ClusterTunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx)

	// Lookup the Tunnel resource
	tunnel := &networkingv1alpha1.ClusterTunnel{}
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

	if err := r.initStruct(ctx, ClusterTunnelAdapter{tunnel, r.Namespace, r.DefaultImage}); err != nil {
		return ctrl.Result{}, err
	}

	if res, ok, err := setupTunnel(r); !ok {
		return res, err
	}

	// Update status
	if err := updateTunnelStatus(r); err != nil {
		return ctrl.Result{}, err
	}

	// Create necessary resources
	if res, ok, err := createManagedResources(r); !ok {
		return res, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("cloudflare-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.ClusterTunnel{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
