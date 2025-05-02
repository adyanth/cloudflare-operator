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

	"sigs.k8s.io/controller-runtime/pkg/conversion"

	networkingv1alpha2 "github.com/adyanth/cloudflare-operator/api/v1alpha2"
)

// ConvertTo converts this ClusterTunnel (v1alpha1) to the Hub version (v1alpha2).
func (src *ClusterTunnel) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*networkingv1alpha2.ClusterTunnel)
	log.Printf("ConvertTo: Converting ClusterTunnel from Spoke version v1alpha1 to Hub version v1alpha2;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// TODO(user): Implement conversion logic from v1alpha1 to v1alpha2
	return nil
}

// ConvertFrom converts the Hub version (v1alpha2) to this ClusterTunnel (v1alpha1).
func (dst *ClusterTunnel) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*networkingv1alpha2.ClusterTunnel)
	log.Printf("ConvertFrom: Converting ClusterTunnel from Hub version v1alpha2 to Spoke version v1alpha1;"+
		"source: %s/%s, target: %s/%s", src.Namespace, src.Name, dst.Namespace, dst.Name)

	// TODO(user): Implement conversion logic from v1alpha2 to v1alpha1
	return nil
}
