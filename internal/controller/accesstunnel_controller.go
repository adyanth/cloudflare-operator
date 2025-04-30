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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/adyanth/cloudflare-operator/internal/k8s"
	"github.com/go-logr/logr"
)

const CONTAINER_PORT int32 = 8000

// AccessTunnelReconciler reconciles a Access object
type AccessTunnelReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme

	// Custom data for ease of (re)use

	ctx          context.Context
	log          logr.Logger
	accessTunnel *networkingv1alpha1.AccessTunnel
}

func (r *AccessTunnelReconciler) GetLog() logr.Logger {
	return r.log
}
func (r *AccessTunnelReconciler) GetRecorder() record.EventRecorder {
	return r.Recorder
}
func (r *AccessTunnelReconciler) GetClient() client.Client {
	return r.Client
}
func (r *AccessTunnelReconciler) GetReconciledObject() client.Object {
	return r.accessTunnel
}
func (r *AccessTunnelReconciler) GetContext() context.Context {
	return r.ctx
}
func (r *AccessTunnelReconciler) GetReconcilerName() string {
	return "AccessTunnel"
}

var _ k8s.GenericReconciler = &AccessTunnelReconciler{}

func cloudflaredDeploymentService(accessTunnel *networkingv1alpha1.AccessTunnel, secret *corev1.Secret) (*appsv1.Deployment, *corev1.Service) {
	svcName := accessTunnel.GetName()
	if accessTunnel.Target.Svc.Name != "" {
		svcName = accessTunnel.Target.Svc.Name
	}
	port := accessTunnel.Target.Svc.Port
	if port == 0 {
		port = CONTAINER_PORT
	}
	namespace := accessTunnel.GetNamespace()
	image := accessTunnel.Target.Image
	fqdn := accessTunnel.Target.Fqdn
	protocol := accessTunnel.Target.Protocol
	corev1Protocol := corev1.ProtocolTCP
	if protocol == "udp" {
		corev1Protocol = corev1.ProtocolUDP
	}

	// Args for cloudflared
	args := []string{"access", protocol, "--listener", fmt.Sprintf("0.0.0.0:%d", CONTAINER_PORT), "--hostname", fqdn}
	if accessTunnel.ServiceToken != nil && secret != nil {
		id := secret.Data[accessTunnel.ServiceToken.CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID]
		token := secret.Data[accessTunnel.ServiceToken.CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN]
		args = append(args, "--service-token-id", string(id), "--service-token-secret", string(token))
	}

	// Instead of using: ctrl.SetControllerReference(accessTunnel, &appsv1.Deployment{}, scheme)
	ownerRefs := []metav1.OwnerReference{{
		APIVersion:         accessTunnel.APIVersion,
		Kind:               accessTunnel.Kind,
		Name:               accessTunnel.Name,
		UID:                accessTunnel.UID,
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}}
	ls := map[string]string{"app": "cloudflared", "name": accessTunnel.Name}
	return &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            accessTunnel.Name,
				Namespace:       namespace,
				Labels:          ls,
				OwnerReferences: ownerRefs,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To(int32(1)),
				Selector: &metav1.LabelSelector{
					MatchLabels: ls,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: ls,
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							RunAsNonRoot: ptr.To(true),
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
									Name:          accessTunnel.Name,
									ContainerPort: CONTAINER_PORT,
									Protocol:      corev1Protocol,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{"memory": resource.MustParse("30Mi"), "cpu": resource.MustParse("10m")},
								Limits:   corev1.ResourceList{"memory": resource.MustParse("256Mi"), "cpu": resource.MustParse("500m")},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
								RunAsUser:                ptr.To(int64(1002)),
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
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            svcName,
				Namespace:       namespace,
				Labels:          ls,
				OwnerReferences: ownerRefs,
			},
			Spec: corev1.ServiceSpec{
				Selector: ls,
				Ports: []corev1.ServicePort{{
					Name:       protocol,
					Protocol:   corev1Protocol,
					TargetPort: intstr.FromInt32(CONTAINER_PORT),
					Port:       port,
				}},
			},
		}
}

// +kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accesstunnels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.cfargotunnel.com,resources=accesstunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;create;update;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;create;update;patch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile the access object
func (r *AccessTunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ctx = ctx
	r.log = ctrllog.FromContext(ctx)

	// Fetch Access from API
	accessTunnel := &networkingv1alpha1.AccessTunnel{}
	if err := r.Get(ctx, req.NamespacedName, accessTunnel); err != nil {
		if apierrors.IsNotFound(err) {
			// Access object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Info("AccessTunnel deleted, nothing to do")
			return ctrl.Result{}, nil
		}
		r.log.Error(err, "unable to fetch AccessTunnel")
		return ctrl.Result{Requeue: true}, err
	}
	r.accessTunnel = accessTunnel

	// Fetch secret if needed
	secret := &corev1.Secret{}
	if accessTunnel.ServiceToken != nil {
		if err := r.Get(ctx, types.NamespacedName{Namespace: accessTunnel.Namespace, Name: accessTunnel.ServiceToken.SecretRef}, secret); err != nil {
			r.log.Error(err, "unable to fetch Secret")
			return ctrl.Result{Requeue: true}, err
		}
		if _, ok := secret.Data[accessTunnel.ServiceToken.CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID]; !ok {
			err := fmt.Errorf("secret does not contain the token ID key %s", accessTunnel.ServiceToken.CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID)
			r.log.Error(err, "invalid secret")
			r.Recorder.Event(accessTunnel, corev1.EventTypeWarning, "InvalidSecret", "Secret Invalid, no token ID key")
			return ctrl.Result{}, err
		}
		if _, ok := secret.Data[accessTunnel.ServiceToken.CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN]; !ok {
			err := fmt.Errorf("secret does not contain the token token key %s", accessTunnel.ServiceToken.CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN)
			r.log.Error(err, "invalid secret")
			r.Recorder.Event(accessTunnel, corev1.EventTypeWarning, "InvalidSecret", "Secret Invalid, no token token key")
			return ctrl.Result{}, err
		}
	}

	// Create/Update deployment
	dep, svc := cloudflaredDeploymentService(accessTunnel, secret)

	if err := k8s.Apply(r, dep); err != nil {
		return ctrl.Result{}, err
	}
	if err := k8s.Apply(r, svc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccessTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("cloudflare-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1alpha1.AccessTunnel{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
