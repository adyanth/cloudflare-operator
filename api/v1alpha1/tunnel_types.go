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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type ExistingTunnel struct {
	//+kubebuilder:validation:Optional
	// Existing Tunnel ID to run on. Tunnel ID and Tunnel Name cannot be both empty. If both are provided, ID is used if valid, else falls back to Name.
	Id string `json:"id,omitempty"`

	//+kubebuilder:validation:Optional
	// Existing Tunnel name to run on. Tunnel Name and Tunnel ID cannot be both empty. If both are provided, ID is used if valid, else falls back to Name.
	Name string `json:"name,omitempty"`
}

type NewTunnel struct {
	//+kubebuilder:validation:Required
	// Tunnel name to create on Cloudflare.
	Name string `json:"name,omitempty"`
}

type CloudflareDetails struct {
	//+kubebuilder:validation:Required
	// Cloudflare Domain to which this tunnel belongs to
	Domain string `json:"domain,omitempty"`

	//+kubebuilder:validation:Required
	// Secret containing Cloudflare API key
	Secret string `json:"secret,omitempty"`

	//+kubebuilder:validation:Optional
	// Account Name in Cloudflare. AccountName and AccountId cannot be both empty. If both are provided, Account ID is used if valid, else falls back to Account Name.
	AccountName string `json:"accountName,omitempty"`

	//+kubebuilder:validation:Optional
	// Account ID in Cloudflare. AccountId and AccountName cannot be both empty. If both are provided, Account ID is used if valid, else falls back to Account Name.
	AccountId string `json:"accountId,omitempty"`

	//+kubebuilder:validation:Optional
	// Email to use along with API Key for Delete operations for new tunnels only, or as an alternate to API Token
	Email string `json:"email,omitempty"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=CLOUDFLARE_API_KEY
	// Key in the secret to use for Cloudflare API Key, defaults to CLOUDFLARE_API_KEY. Needs Email also to be provided.
	// For Delete operations for new tunnels only, or as an alternate to API Token
	CLOUDFLARE_API_KEY string `json:"CLOUDFLARE_API_KEY,omitempty"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=CLOUDFLARE_API_TOKEN
	// Key in the secret to use for Cloudflare API token, defaults to CLOUDFLARE_API_TOKEN
	CLOUDFLARE_API_TOKEN string `json:"CLOUDFLARE_API_TOKEN,omitempty"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=CLOUDFLARE_TUNNEL_CREDENTIAL_FILE
	// Key in the secret to use as credentials.json for the tunnel, defaults to CLOUDFLARE_TUNNEL_CREDENTIAL_FILE
	CLOUDFLARE_TUNNEL_CREDENTIAL_FILE string `json:"CLOUDFLARE_TUNNEL_CREDENTIAL_FILE,omitempty"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET
	// Key in the secret to use as credentials.json for the tunnel, defaults to CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET
	CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET string `json:"CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET,omitempty"`
}

// TunnelSpec defines the desired state of Tunnel
type TunnelSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:default:=1
	//+kubebuilder:validation:Optional
	// Number of Daemon pods to run for this tunnel
	Size int32 `json:"size,omitempty"`

	//+kubebuilder:validation:Required
	// Cloudflare Credentials
	Cloudflare CloudflareDetails `json:"cloudflare,omitempty"`

	//+kubebuilder:validation:Optional
	// Existing tunnel object.
	// ExistingTunnel and NewTunnel cannot be both empty and are mutually exclusive.
	ExistingTunnel ExistingTunnel `json:"existingTunnel,omitempty"`

	//+kubebuilder:validation:Optional
	// New tunnel object.
	// NewTunnel and ExistingTunnel cannot be both empty and are mutually exclusive.
	NewTunnel NewTunnel `json:"newTunnel,omitempty"`
}

// TunnelStatus defines the observed state of Tunnel
type TunnelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	TunnelId   string `json:"tunnelId"`
	TunnelName string `json:"tunnelName"`
	AccountId  string `json:"accountId"`
	ZoneId     string `json:"zoneId"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Tunnel is the Schema for the tunnels API
type Tunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TunnelSpec   `json:"spec,omitempty"`
	Status TunnelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TunnelList contains a list of Tunnel
type TunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Tunnel{}, &TunnelList{})
}
