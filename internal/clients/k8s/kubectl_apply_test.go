package k8s_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/adyanth/cloudflare-operator/internal/clients/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	TUNNEL_NAME string = "testing-tunnel"
)

func TestEmptyPatch(t *testing.T) {
	dep := sampleDeployment()
	newDep := &appsv1.Deployment{}
	if err := k8s.StrategicPatch(dep, "{}", newDep); err != nil {
		t.Error(err)
	}
	old, _ := json.Marshal(dep.Spec)
	new, _ := json.Marshal(newDep.Spec)
	if !slices.Equal(old, new) {
		t.Logf("original: \n%s\nnew: \n%s", string(old), string(new))
		t.Fail()
	}
}

func TestStrategicPatch(t *testing.T) {
	dep := sampleDeployment()
	patchSpec := samplePatch()
	newDep := &appsv1.Deployment{}
	if err := k8s.StrategicPatch(dep, patchSpec, newDep); err != nil {
		t.Error(err)
	}
	if *newDep.Spec.Replicas != 3 {
		t.Fail()
	}
	if len(newDep.Spec.Template.Spec.Containers) != 1 {
		t.Fail()
	}
	if newDep.Spec.Template.Spec.Containers[0].Image != "cloudflared:new" {
		t.Fail()
	}
}

func samplePatch() string {
	return `
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: cloudflared
        image: cloudflared:new
`
}

func sampleDeployment() *appsv1.Deployment {
	ls := map[string]string{"app": "test"}
	args := []string{"tunnel", "--config", "/etc/cloudflared/config/config.yaml", "--metrics", "0.0.0.0:2000", "run"}
	volumes := []corev1.Volume{{
		Name: "creds",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  TUNNEL_NAME,
				DefaultMode: ptr.To(int32(420)),
			},
		},
	}, {
		Name: "config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: TUNNEL_NAME},
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

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      TUNNEL_NAME,
			Namespace: TUNNEL_NAME,
			Labels:    ls,
		},
		Spec: appsv1.DeploymentSpec{
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
						Image: "cloudflared:old",
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
					Volumes: volumes,
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
	return dep
}
