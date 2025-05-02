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

package v1alpha1

import (
	"log"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/yaml"

	networkingv1alpha2 "github.com/adyanth/cloudflare-operator/api/v1alpha2"
)

// Patch conversion type
type v1alpha1Tov1alpha2PatchSpecTemplateSpecContainer struct {
	Name   string         `json:"name,omitempty"`
	Image  string         `json:"image,omitempty"`
	Extras map[string]any `json:",inline,omitempty"`
}

type v1alpha1Tov1alpha2PatchSpecTemplateSpec struct {
	Tolerations  []corev1.Toleration                                `json:"tolerations,omitempty"`
	NodeSelector map[string]string                                  `json:"nodeSelector,omitempty"`
	Containers   []v1alpha1Tov1alpha2PatchSpecTemplateSpecContainer `json:"containers,omitempty"`
	Extras       map[string]any                                     `json:",inline,omitempty"`
}

type v1alpha1Tov1alpha2PatchSpecTemplate struct {
	Spec   *v1alpha1Tov1alpha2PatchSpecTemplateSpec `json:"spec,omitempty"`
	Extras map[string]any                           `json:",inline,omitempty"`
}

type v1alpha1Tov1alpha2PatchSpec struct {
	Replicas int32                                `json:"replicas,omitempty"`
	Template *v1alpha1Tov1alpha2PatchSpecTemplate `json:"template,omitempty"`
	Extras   map[string]any                       `json:",inline,omitempty"`
}

type v1alpha1Tov1alpha2Patch struct {
	Spec   *v1alpha1Tov1alpha2PatchSpec `json:"spec,omitempty"`
	Extras map[string]any               `json:",inline,omitempty"`
}

// ConvertTo converts this Tunnel (v1alpha1) to the Hub version (v1alpha2).
func (src *Tunnel) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*networkingv1alpha2.Tunnel)
	log.Printf("ConvertTo: Converting Tunnel from Spoke version v1alpha1 to Hub version v1alpha2;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// Implement conversion logic from v1alpha1 to v1alpha2
	dst.ObjectMeta = src.ObjectMeta
	if err := src.Spec.ConvertTo(&dst.Spec); err != nil {
		return err
	}
	if err := src.Status.ConvertTo(&dst.Status); err != nil {
		return err
	}
	return nil
}

func (src TunnelSpec) ConvertTo(dst *networkingv1alpha2.TunnelSpec) error {

	if (src.NewTunnel != NewTunnel{}) {
		dst.NewTunnel = (*networkingv1alpha2.NewTunnel)(&src.NewTunnel)
	}
	if (src.ExistingTunnel != ExistingTunnel{}) {
		dst.ExistingTunnel = (*networkingv1alpha2.ExistingTunnel)(&src.ExistingTunnel)
	}
	dst.Cloudflare = networkingv1alpha2.CloudflareDetails(src.Cloudflare)
	dst.FallbackTarget = src.FallbackTarget
	dst.Protocol = src.Protocol
	dst.OriginCaPool = src.OriginCaPool
	dst.NoTlsVerify = src.NoTlsVerify

	patch := &v1alpha1Tov1alpha2Patch{}

	if src.Size != 0 && src.Size != 1 {
		if patch.Spec == nil {
			patch.Spec = &v1alpha1Tov1alpha2PatchSpec{}
		}
		patch.Spec.Replicas = src.Size
	}

	if len(src.NodeSelectors) != 0 || len(src.Tolerations) != 0 || src.Image != "" {
		if patch.Spec == nil {
			patch.Spec = &v1alpha1Tov1alpha2PatchSpec{}
		}
		if patch.Spec.Template == nil {
			patch.Spec.Template = &v1alpha1Tov1alpha2PatchSpecTemplate{}
		}
		if patch.Spec.Template.Spec == nil {
			patch.Spec.Template.Spec = &v1alpha1Tov1alpha2PatchSpecTemplateSpec{}
		}

		if len(src.NodeSelectors) != 0 {
			patch.Spec.Template.Spec.NodeSelector = src.NodeSelectors
		}
		if len(src.Tolerations) != 0 {
			patch.Spec.Template.Spec.Tolerations = src.Tolerations
		}
		if src.Image != "" {
			patch.Spec.Template.Spec.Containers = []v1alpha1Tov1alpha2PatchSpecTemplateSpecContainer{{
				Name:  "cloudflared",
				Image: src.Image,
			}}
		}
	}

	patchBytes, err := yaml.Marshal(patch)
	if err != nil {
		return err
	}
	patchBytes, err = yaml.YAMLToJSON(patchBytes)
	if err != nil {
		return err
	}

	dst.DeployPatch = string(patchBytes)
	return nil
}

func (src TunnelStatus) ConvertTo(dst *networkingv1alpha2.TunnelStatus) error {
	dst = (*networkingv1alpha2.TunnelStatus)(&src)
	return nil
}

// ConvertFrom converts the Hub version (v1alpha2) to this Tunnel (v1alpha1).
func (dst *Tunnel) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*networkingv1alpha2.Tunnel)
	log.Printf("ConvertFrom: Converting Tunnel from Hub version v1alpha2 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// Implement conversion logic from v1alpha2 to v1alpha1
	dst.ObjectMeta = src.ObjectMeta
	if err := dst.Spec.ConvertFrom(src.Spec); err != nil {
		return err
	}
	if err := dst.Status.ConvertFrom(src.Status); err != nil {
		return err
	}
	return nil
}

func (dst *TunnelSpec) ConvertFrom(src networkingv1alpha2.TunnelSpec) error {
	if src.NewTunnel != nil {
		dst.NewTunnel = NewTunnel(*src.NewTunnel)
	}
	if src.ExistingTunnel != nil {
		dst.ExistingTunnel = ExistingTunnel(*src.ExistingTunnel)
	}
	dst.Cloudflare = CloudflareDetails(src.Cloudflare)
	dst.FallbackTarget = src.FallbackTarget
	dst.Protocol = src.Protocol
	dst.OriginCaPool = src.OriginCaPool
	dst.NoTlsVerify = src.NoTlsVerify

	patch := &v1alpha1Tov1alpha2Patch{}
	if err := yaml.Unmarshal([]byte(src.DeployPatch), patch); err != nil {
		return err
	}

	if spec := patch.Spec; spec != nil {
		dst.Size = spec.Replicas

		if template := spec.Template; template != nil {
			if spec := template.Spec; spec != nil {
				if len(spec.NodeSelector) != 0 {
					dst.NodeSelectors = spec.NodeSelector
				}
				if len(spec.Tolerations) != 0 {
					dst.Tolerations = spec.Tolerations
				}
				if len(spec.Containers) != 0 {
					for _, container := range spec.Containers {
						if container.Name == "cloudflared" {
							dst.Image = container.Image
							break
						}

						if len(container.Extras) != 0 {
							log.Printf("Extras found in patch spec template spec containers, not converted: %+v", spec.Extras)
						}
					}
				}

				if len(spec.Extras) != 0 {
					log.Printf("Extras found in patch spec template spec, not converted: %+v", spec.Extras)
				}
			}

			if len(template.Extras) != 0 {
				log.Printf("Extras found in patch spec.template, not converted: %+v", template.Extras)
			}
		}

		if len(spec.Extras) != 0 {
			log.Printf("Extras found in patch spec, not converted: %+v", spec.Extras)
		}
	}
	if len(patch.Extras) != 0 {
		log.Printf("Extras found in patch, not converted: %+v", patch.Extras)
	}

	return nil
}

func (dst *TunnelStatus) ConvertFrom(src networkingv1alpha2.TunnelStatus) error {
	dst = (*TunnelStatus)(&src)
	return nil
}
