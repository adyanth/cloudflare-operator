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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AccessTunnelServiceConfig defines the Service created to Access
type AccessTunnelServiceConfig struct {
	// Name of the new service to create
	// Defaults to the name of the Access object
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	// Service port to expose with
	// Defaults to 8000
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=8000
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:validation:Maximum:=65535
	Port int32 `json:"port,omitempty"`
}

// AccessTunnelTarget defines the desired state of Access
type AccessTunnelTarget struct {
	// cloudflared image to use
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="cloudflare/cloudflared:2025.4.0"
	Image string `json:"image,omitempty"`

	// Fqdn specifies the DNS name to access
	// This is not validated and used as provided
	// +kubebuilder:validation:Required
	Fqdn string `json:"fqdn,omitempty"`

	// Protocol to forward, better to use TCP?
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=tcp
	// +kubebuilder:validation:Enum:="tcp";"rdp";"smb";"ssh"
	Protocol string `json:"protocol,omitempty"`

	// Service Config
	// +kubebuilder:validation:Optional
	Svc AccessTunnelServiceConfig `json:"svc,omitempty"`
}

// AccessTunnelServiceToken defines the access auth if needed
type AccessTunnelServiceToken struct {
	// Access Service Token Secret
	// +kubebuilder:validation:Required
	SecretRef string `json:"secretRef,omitempty"`

	// Key in the secret to use for Access Service Token ID, defaults to CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID
	CLOUDFLARE_ACCESS_SERVICE_TOKEN_ID string `json:"id,omitempty"`
	// Key in the secret to use for Access Service Token Token, defaults to CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN
	CLOUDFLARE_ACCESS_SERVICE_TOKEN_TOKEN string `json:"token,omitempty"`
}

// AccessTunnelStatus defines the observed state of Access
type AccessTunnelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.target.fqdn`

// AccessTunnel is the Schema for the accesstunnels API
type AccessTunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Target       AccessTunnelTarget        `json:"target,omitempty"`
	ServiceToken *AccessTunnelServiceToken `json:"serviceToken,omitempty"`
	Status       AccessTunnelStatus        `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccessTunnelList contains a list of AccessTunnel
type AccessTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessTunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessTunnel{}, &AccessTunnelList{})
}
