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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

// AccessServiceReconciler reconciles a AccessService object
type AccessServiceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	access *networkingv1alpha1.AccessService
	log    logr.Logger
}

//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accessservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accessservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accessservices/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AccessService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *AccessServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = log.FromContext(ctx)

	// Lookup the AccessService resource
	r.access = &networkingv1alpha1.AccessService{}
	if err := r.Get(ctx, req.NamespacedName, r.access); err != nil {
		if apierrors.IsNotFound(err) {
			// AccessService object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Info("AccessService deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		r.log.Error(err, "unable to fetch AccessService")
		return ctrl.Result{}, err
	}

	if res, err := r.createAccessService(ctx); err != nil || (res != ctrl.Result{}) {
		return res, err
	}

	if res, err := r.createAccessDeployment(ctx); err != nil || (res != ctrl.Result{}) {
		return res, err
	}

	return ctrl.Result{}, nil
}

func (r *AccessServiceReconciler) createAccessService(ctx context.Context) (ctrl.Result, error) {
	acService := &corev1.Service{}
	acServiceName := apitypes.NamespacedName{Name: r.access.Spec.ServiceName, Namespace: r.access.Namespace}
	if err := r.Client.Get(ctx, acServiceName, acService); err != nil && apierrors.IsNotFound(err) {
		// Define a new service
		svc := r.serviceForAccess()
		r.log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deploying", "Creating AccessService Service")
		err = r.Client.Create(ctx, svc)
		if err != nil {
			r.log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			r.Recorder.Event(r.access, corev1.EventTypeWarning, "FailedDeploying", "Creating AccessService Service failed")
			return ctrl.Result{}, err
		}
		r.log.Info("Service created", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deployed", "Created AccessService Service")
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		r.log.Error(err, "Failed to get Service")
		r.Recorder.Event(r.access, corev1.EventTypeWarning, "FailedDeployed", "Reading AccessService Service failed")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *AccessServiceReconciler) getLabels() map[string]string {
	return map[string]string{
		"app":      "cloudflared",
		"access":   r.access.Name,
		"service":  r.access.Spec.ServiceName,
		"protocol": r.access.Spec.Protocol,
		"port":     fmt.Sprintf("%d", r.access.Spec.Port),
	}
}

// deploymentForService returns a service object
func (r *AccessServiceReconciler) serviceForAccess() *corev1.Service {
	ls := r.getLabels()

	if r.access.Spec.Protocol != "TCP" {
		r.log.Error(
			fmt.Errorf("ignoring Protocol, using TCP"),
			"Ignoring Protocol, using TCP",
			"AccessService.Name", r.access.Name,
			"AccessService.Namespace", r.access.Namespace,
			"protocol", r.access.Spec.Protocol,
		)
	}
	proto := corev1.ProtocolTCP

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.access.Spec.ServiceName,
			Namespace: r.access.Namespace,
			Labels:    ls,
		},
		Spec: corev1.ServiceSpec{
			Selector: ls,
			Ports: []corev1.ServicePort{
				{
					Port:       r.access.Spec.Port,
					TargetPort: intstr.FromString("access"),
					Protocol:   proto,
				},
			},
		},
	}
	// Set AccessService instance as the owner and controller
	ctrl.SetControllerReference(r.access, svc, r.Scheme)
	return svc
}

func (r *AccessServiceReconciler) createAccessDeployment(ctx context.Context) (ctrl.Result, error) {
	acDeploy := &appsv1.Deployment{}
	acDeployName := apitypes.NamespacedName{Name: r.access.Spec.ServiceName, Namespace: r.access.Namespace}
	if err := r.Client.Get(ctx, acDeployName, acDeploy); err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		dep := r.deploymentForAccess()
		r.log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deploying", "Creating AccessService Deployment")
		err = r.Client.Create(ctx, dep)
		if err != nil {
			r.log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			r.Recorder.Event(r.access, corev1.EventTypeWarning, "FailedDeploying", "Creating AccessService Deployment failed")
			return ctrl.Result{}, err
		}
		r.log.Info("Deployment created", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deployed", "Created AccessService Deployment")
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		r.log.Error(err, "Failed to get Deployment")
		r.Recorder.Event(r.access, corev1.EventTypeWarning, "FailedDeployed", "Reading AccessService Deployment failed")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// deploymentForAccess returns a deployment object
func (r *AccessServiceReconciler) deploymentForAccess() *appsv1.Deployment {
	replicas := r.access.Spec.Replicas
	ls := r.getLabels()

	if r.access.Spec.Protocol != "TCP" {
		r.log.Error(
			fmt.Errorf("ignoring Protocol, using TCP"),
			"Ignoring Protocol, using TCP",
			"AccessService.Name", r.access.Name,
			"AccessService.Namespace", r.access.Namespace,
			"protocol", r.access.Spec.Protocol,
		)
	}
	proto := corev1.ProtocolTCP

	url := fmt.Sprintf("0.0.0.0:%d", r.access.Spec.Port)
	args := []string{"access", r.access.Spec.Protocol, "--hostname", r.access.Spec.Hostname, "--url", url}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.access.Spec.ServiceName,
			Namespace: r.access.Namespace,
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
						Image: "cloudflare/cloudflared:latest",
						Name:  "cloudflared",
						Args:  args,
						Ports: []corev1.ContainerPort{
							{
								Name:          "access",
								ContainerPort: r.access.Spec.Port,
								Protocol:      proto,
							},
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{"memory": resource.MustParse("30Mi"), "cpu": resource.MustParse("10m")},
							Limits:   corev1.ResourceList{"memory": resource.MustParse("256Mi")},
						},
					}},
				},
			},
		},
	}
	// Set AccessService instance as the owner and controller
	ctrl.SetControllerReference(r.access, dep, r.Scheme)
	return dep
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("cloudflare-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.AccessService{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
