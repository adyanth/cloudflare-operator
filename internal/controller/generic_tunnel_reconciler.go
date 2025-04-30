package controller

import (
	"fmt"
	"time"

	networkingv1alpha1 "github.com/adyanth/cloudflare-operator/api/v1alpha1"
	"github.com/adyanth/cloudflare-operator/internal/k8s"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type GenericTunnelReconciler interface {
	k8s.GenericReconciler

	GetScheme() *runtime.Scheme
	GetTunnel() Tunnel
	GetCfAPI() *CloudflareAPI
	SetCfAPI(*CloudflareAPI)
	GetCfSecret() *corev1.Secret
	GetTunnelCreds() string
	SetTunnelCreds(string)
}

// labelsForTunnel returns the labels for selecting the resources
// belonging to the given Tunnel CR name.
func labelsForTunnel(cf Tunnel) map[string]string {
	return map[string]string{
		tunnelLabel:          cf.GetName(),
		tunnelAppLabel:       "cloudflared",
		tunnelIdLabel:        cf.GetStatus().TunnelId,
		tunnelNameLabel:      cf.GetStatus().TunnelName,
		tunnelDomainLabel:    cf.GetSpec().Cloudflare.Domain,
		isClusterTunnelLabel: "false",
	}
}

func nodeSelectorsForTunnel(cf Tunnel) map[string]string {
	return cf.GetSpec().NodeSelectors
}

func setupTunnel(r GenericTunnelReconciler) (ctrl.Result, bool, error) {
	okNewTunnel := r.GetTunnel().GetSpec().NewTunnel != networkingv1alpha1.NewTunnel{}
	okExistingTunnel := r.GetTunnel().GetSpec().ExistingTunnel != networkingv1alpha1.ExistingTunnel{}

	// If both are set (or neither are), we have a problem
	if okNewTunnel == okExistingTunnel {
		err := fmt.Errorf("spec ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		r.GetLog().Error(err, "spec ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "ErrSpecTunnel", "ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive")
		return ctrl.Result{}, false, err
	}

	if okExistingTunnel {
		// Existing Tunnel, Set tunnelId in status and get creds file
		if err := setupExistingTunnel(r); err != nil {
			return ctrl.Result{}, false, err
		}
	} else {
		// New tunnel, finalizer/cleanup logic + creation
		if r.GetTunnel().GetObject().GetDeletionTimestamp() != nil {
			if res, ok, err := cleanupTunnel(r); !ok {
				return res, false, err
			}
		} else {
			if err := setupNewTunnel(r); err != nil {
				return ctrl.Result{}, false, err
			}
		}
	}

	return ctrl.Result{}, true, nil
}

func setupExistingTunnel(r GenericTunnelReconciler) error {
	cfAPI := r.GetCfAPI()
	cfAPI.TunnelName = r.GetTunnel().GetSpec().ExistingTunnel.Name
	cfAPI.TunnelId = r.GetTunnel().GetSpec().ExistingTunnel.Id
	r.SetCfAPI(cfAPI)

	// Read secret for credentials file
	cfCredFileB64, okCredFile := r.GetCfSecret().Data[r.GetTunnel().GetSpec().Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE]
	cfSecretB64, okSecret := r.GetCfSecret().Data[r.GetTunnel().GetSpec().Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET]

	if !okCredFile && !okSecret {
		err := fmt.Errorf("neither key not found in secret")
		r.GetLog().Error(err, "neither key not found in secret", "secret", r.GetTunnel().GetSpec().Cloudflare.Secret, "key1", r.GetTunnel().GetSpec().Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_FILE, "key2", r.GetTunnel().GetSpec().Cloudflare.CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET)
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "ErrSpecSecret", "Neither Key found in Secret")
		return err
	}

	if okCredFile {
		r.SetTunnelCreds(string(cfCredFileB64))
	} else {
		creds, err := r.GetCfAPI().GetTunnelCreds(string(cfSecretB64))
		if err != nil {
			r.GetLog().Error(err, "error getting tunnel credentials from secret")
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "ErrSpecApi", "Error in getting Tunnel Credentials from Secret")
			return err
		}
		r.SetTunnelCreds(creds)
	}

	return nil
}

func setupNewTunnel(r GenericTunnelReconciler) error {
	// New tunnel, not yet setup, create on Cloudflare
	if r.GetTunnel().GetStatus().TunnelId == "" {
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "Creating", "Tunnel is being created")
		r.GetCfAPI().TunnelName = r.GetTunnel().GetSpec().NewTunnel.Name
		_, creds, err := r.GetCfAPI().CreateCloudflareTunnel()
		if err != nil {
			r.GetLog().Error(err, "unable to create Tunnel")
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "FailedCreate", "Unable to create Tunnel on Cloudflare")
			return err
		}
		r.GetLog().Info("Tunnel created on Cloudflare")
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "Created", "Tunnel created successfully on Cloudflare")
		r.SetTunnelCreds(creds)
	}

	// Add finalizer for tunnel
	if !controllerutil.ContainsFinalizer(r.GetTunnel().GetObject(), tunnelFinalizer) {
		controllerutil.AddFinalizer(r.GetTunnel().GetObject(), tunnelFinalizer)
		if err := r.GetClient().Update(r.GetContext(), r.GetTunnel().GetObject()); err != nil {
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "FailedFinalizerSet", "Failed to add Tunnel Finalizer")
			return err
		}
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "FinalizerSet", "Tunnel Finalizer added")
	}
	return nil
}

func cleanupTunnel(r GenericTunnelReconciler) (ctrl.Result, bool, error) {
	if controllerutil.ContainsFinalizer(r.GetTunnel().GetObject(), tunnelFinalizer) {
		// Run finalization logic. If the finalization logic fails,
		// don't remove the finalizer so that we can retry during the next reconciliation.

		r.GetLog().Info("starting deletion cycle", "size", r.GetTunnel().GetSpec().Size)
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "Deleting", "Starting Tunnel Deletion")
		cfDeployment := &appsv1.Deployment{}
		var bypass bool
		if err := r.GetClient().Get(r.GetContext(), apitypes.NamespacedName{Name: r.GetTunnel().GetName(), Namespace: r.GetTunnel().GetNamespace()}, cfDeployment); err != nil {
			r.GetLog().Error(err, "Error in getting deployments, might already be deleted?")
			bypass = true
		}
		if !bypass && *cfDeployment.Spec.Replicas != 0 {
			r.GetLog().Info("Scaling down cloudflared")
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "Scaling", "Scaling down cloudflared")
			var size int32 = 0
			cfDeployment.Spec.Replicas = &size
			if err := r.GetClient().Update(r.GetContext(), cfDeployment); err != nil {
				r.GetLog().Error(err, "Failed to update Deployment", "Deployment.Namespace", cfDeployment.Namespace, "Deployment.Name", cfDeployment.Name)
				r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "FailedScaling", "Failed to scale down cloudflared")
				return ctrl.Result{}, false, err
			}
			r.GetLog().Info("Scaling down successful", "size", r.GetTunnel().GetSpec().Size)
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "Scaled", "Scaling down cloudflared successful")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, false, nil
		}
		if bypass || *cfDeployment.Spec.Replicas == 0 {
			if err := r.GetCfAPI().DeleteCloudflareTunnel(); err != nil {
				r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "FailedDeleting", "Tunnel deletion failed")
				return ctrl.Result{}, false, err
			}
			r.GetLog().Info("Tunnel deleted", "tunnelID", r.GetTunnel().GetStatus().TunnelId)
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "Deleted", "Tunnel deletion successful")

			// Remove tunnelFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(r.GetTunnel().GetObject(), tunnelFinalizer)
			err := r.GetClient().Update(r.GetContext(), r.GetTunnel().GetObject())
			if err != nil {
				r.GetLog().Error(err, "unable to continue with tunnel deletion")
				r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "FailedFinalizerUnset", "Unable to remove Tunnel Finalizer")
				return ctrl.Result{}, false, err
			}
			r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeNormal, "FinalizerUnset", "Tunnel Finalizer removed")
			return ctrl.Result{}, true, nil
		}
	}
	return ctrl.Result{}, true, nil
}

func updateTunnelStatus(r GenericTunnelReconciler) error {
	labels := r.GetTunnel().GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range labelsForTunnel(r.GetTunnel()) {
		labels[k] = v
	}
	r.GetTunnel().SetLabels(labels)
	if err := r.GetClient().Update(r.GetContext(), r.GetTunnel().GetObject()); err != nil {
		return err
	}

	if err := r.GetCfAPI().ValidateAll(); err != nil {
		r.GetLog().Error(err, "Failed to validate API credentials")
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "ErrSpecApi", "Error validating Cloudflare API credentials")
		return err
	}
	status := r.GetTunnel().GetStatus()
	status.AccountId = r.GetCfAPI().ValidAccountId
	status.TunnelId = r.GetCfAPI().ValidTunnelId
	status.TunnelName = r.GetCfAPI().ValidTunnelName
	status.ZoneId = r.GetCfAPI().ValidZoneId
	r.GetTunnel().SetStatus(status)
	if err := r.GetClient().Status().Update(r.GetContext(), r.GetTunnel().GetObject()); err != nil {
		r.GetLog().Error(err, "Failed to update Tunnel status", "Tunnel.Namespace", r.GetTunnel().GetNamespace(), "Tunnel.Name", r.GetTunnel().GetName())
		r.GetRecorder().Event(r.GetTunnel().GetObject(), corev1.EventTypeWarning, "FailedStatusSet", "Failed to set Tunnel status required for operation")
		return err
	}
	r.GetLog().Info("Tunnel status is set", "status", r.GetTunnel().GetStatus())
	return nil
}

func createManagedResources(r GenericTunnelReconciler) (ctrl.Result, error) {
	// Check if Secret already exists, else create it
	if err := k8s.Apply(r, secretForTunnel(r)); err != nil {
		return ctrl.Result{}, err
	}

	// Check if ConfigMap already exists, else create it
	if err := k8s.MergeOrApply(r, configMapForTunnel(r)); err != nil {
		return ctrl.Result{}, err
	}

	if err := k8s.Apply(r, deploymentForTunnel(r)); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// configMapForTunnel returns a tunnel ConfigMap object
func configMapForTunnel(r GenericTunnelReconciler) *corev1.ConfigMap {
	ls := labelsForTunnel(r.GetTunnel())
	noTlsVerify := r.GetTunnel().GetSpec().NoTlsVerify
	originRequest := OriginRequestConfig{
		NoTLSVerify: &noTlsVerify,
	}
	if r.GetTunnel().GetSpec().OriginCaPool != "" {
		defaultCaPool := "/etc/cloudflared/certs/tls.crt"
		originRequest.CAPool = &defaultCaPool
	}
	initialConfigBytes, _ := yaml.Marshal(Configuration{
		TunnelId:      r.GetTunnel().GetStatus().TunnelId,
		SourceFile:    "/etc/cloudflared/creds/credentials.json",
		Metrics:       "0.0.0.0:2000",
		NoAutoUpdate:  true,
		OriginRequest: originRequest,
		Ingress: []UnvalidatedIngressRule{{
			Service: r.GetTunnel().GetSpec().FallbackTarget,
		}},
	})

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.GetTunnel().GetName(),
			Namespace: r.GetTunnel().GetNamespace(),
			Labels:    ls,
		},
		Data: map[string]string{"config.yaml": string(initialConfigBytes)},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(r.GetTunnel().GetObject(), cm, r.GetScheme())
	return cm
}

// secretForTunnel returns a tunnel Secret object
func secretForTunnel(r GenericTunnelReconciler) *corev1.Secret {
	ls := labelsForTunnel(r.GetTunnel())
	sec := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.GetTunnel().GetName(),
			Namespace: r.GetTunnel().GetNamespace(),
			Labels:    ls,
		},
		StringData: map[string]string{"credentials.json": r.GetTunnelCreds()},
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(r.GetTunnel().GetObject(), sec, r.GetScheme())
	return sec
}

// deploymentForTunnel returns a tunnel Deployment object
func deploymentForTunnel(r GenericTunnelReconciler) *appsv1.Deployment {
	ls := labelsForTunnel(r.GetTunnel())
	replicas := r.GetTunnel().GetSpec().Size
	nodeSelector := nodeSelectorsForTunnel(r.GetTunnel())
	tolerations := r.GetTunnel().GetSpec().Tolerations
	protocol := r.GetTunnel().GetSpec().Protocol

	args := []string{"tunnel", "--protocol", protocol, "--config", "/etc/cloudflared/config/config.yaml", "--metrics", "0.0.0.0:2000", "run"}
	volumes := []corev1.Volume{{
		Name: "creds",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  r.GetTunnel().GetName(),
				DefaultMode: ptr.To(int32(420)),
			},
		},
	}, {
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: r.GetTunnel().GetName()},
				Items: []corev1.KeyToPath{{
					Key:  "config.yaml",
					Path: "config.yaml",
				}},
				DefaultMode: ptr.To(int32(420)),
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
	if r.GetTunnel().GetSpec().OriginCaPool != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "certs",
			MountPath: "/etc/cloudflared/certs",
			ReadOnly:  true,
		})
		volumes = append(volumes, corev1.Volume{
			Name: "certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  r.GetTunnel().GetSpec().OriginCaPool,
					DefaultMode: ptr.To(int32(420)),
				},
			},
		})
	}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.GetTunnel().GetName(),
			Namespace: r.GetTunnel().GetNamespace(),
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
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image: r.GetTunnel().GetSpec().Image,
						Name:  "cloudflared",
						Args:  args,
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/ready",
									Port: intstr.IntOrString{IntVal: 2000},
								},
							},
							FailureThreshold:    1,
							InitialDelaySeconds: 10,
							PeriodSeconds:       10,
						},
						Ports: []corev1.ContainerPort{
							{
								Name:          "metrics",
								ContainerPort: 2000,
								Protocol:      corev1.ProtocolTCP,
							},
						},
						VolumeMounts: volumeMounts,
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
					Volumes:      volumes,
					NodeSelector: nodeSelector,
					Tolerations:  tolerations,
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
	}
	// Set Tunnel instance as the owner and controller
	ctrl.SetControllerReference(r.GetTunnel().GetObject(), dep, r.GetScheme())
	return dep
}
