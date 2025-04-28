/*
Copyright 2025 Adyanth H.

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

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/adyanth/cloudflare-operator/internal/utils/pointer"
	"github.com/go-logr/logr"
)

// AccessReconciler reconciles a Access object
type AccessReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme

	// Custom data for ease of (re)use

	ctx    context.Context
	log    logr.Logger
	access *networkingv1alpha1.Access
}

func cloudflaredDeploymentService(access *networkingv1alpha1.Access) (*appsv1.Deployment, *corev1.Service) {
	name := access.GetName()
	namespace := access.GetNamespace()
	fqdn := access.Target.Fqdn
	port := access.Target.Svc.Port
	image := access.Target.Image
	protocol := access.Target.Protocol
	corev1Protocol := corev1.ProtocolTCP
	if protocol == "udp" {
		corev1Protocol = corev1.ProtocolUDP
	}
	args := []string{}
	ls := map[string]string{"app": "cloudflared", "name": name, "fqdn": fqdn, "port": fmt.Sprint(port)}
	return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    ls,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: ls,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: ls,
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: pointer.To(true),
							SeccompProfile: &corev1.SeccompProfile{
								Type: corev1.SeccompProfileTypeRuntimeDefault,
							},
						},
						Containers: []corev1.Container{{
							Image: image,
							Name:  "cloudflared",
							Args:  args,
							Ports: []corev1.ContainerPort{
								{
									Name:          protocol,
									ContainerPort: port,
									Protocol:      corev1Protocol,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{"memory": resource.MustParse("30Mi"), "cpu": resource.MustParse("10m")},
								Limits:   corev1.ResourceList{"memory": resource.MustParse("256Mi"), "cpu": resource.MustParse("500m")},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: pointer.To(false),
								ReadOnlyRootFilesystem:   pointer.To(true),
								RunAsUser:                pointer.To(int64(1002)),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
							},
						}},
						Affinity: &corev1.Affinity{
							NodeAffinity: &corev1.NodeAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
									NodeSelectorTerms: []corev1.NodeSelectorTerm{
										{
											MatchExpressions: []corev1.NodeSelectorRequirement{
												{
													Key:      "kubernetes.io/arch",
													Operator: corev1.NodeSelectorOpIn,
													Values: []string{
														"amd64",
														"arm64",
													},
												},
												{
													Key:      "kubernetes.io/os",
													Operator: corev1.NodeSelectorOpIn,
													Values: []string{
														"linux",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    ls,
			},
			Spec: corev1.ServiceSpec{
				Selector: ls,
				Ports: []corev1.ServicePort{
					corev1.ServicePort{
						Name:       protocol,
						Protocol:   corev1Protocol,
						TargetPort: intstr.IntOrString{IntVal: port},
						Port:       port,
					},
				},
			},
		}
}

// +kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accesses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accesses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;create;update;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;create;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile the access object
func (r *AccessReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log = ctrllog.FromContext(ctx)

	// Fetch Access from API
	access := &networkingv1alpha1.Access{}
	if err := r.Get(ctx, req.NamespacedName, access); err != nil {
		if apierrors.IsNotFound(err) {
			// Access object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Info("Access deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		r.log.Error(err, "unable to fetch Access")
		return ctrl.Result{Requeue: true}, err
	}
	r.access = access

	// Create/Update deployment
	dep, svc := cloudflaredDeploymentService(r.access)
	r.log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
	r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deploying", "Creating Access Deployment")
	if err := r.Client.Update(ctx, dep); err != nil {
		r.log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		r.Recorder.Event(r.access, corev1.EventTypeWarning, "FailedDeploying", "Creating Access Deployment failed")
		return ctrl.Result{}, err
	}
	r.log.Info("Deployment created", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
	r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deployed", "Created Access Deployment")

	// Create/Update service
	r.log.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
	r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deploying", "Creating Access Service")
	if err := r.Client.Update(ctx, svc); err != nil {
		r.log.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		r.Recorder.Event(r.access, corev1.EventTypeWarning, "FailedDeploying", "Creating Access Service failed")
		return ctrl.Result{}, err
	}
	r.log.Info("Service created", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
	r.Recorder.Event(r.access, corev1.EventTypeNormal, "Deployed", "Created Access Service")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.Access{}).
		Complete(r)
}
