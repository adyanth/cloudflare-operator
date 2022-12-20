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

// ExistingTunnel spec needs either a Tunnel Id or a Name to find it on Cloudflare.
type ExistingTunnel struct {
	//+kubebuilder:validation:Optional
	// Existing Tunnel ID to run on. Tunnel ID and Tunnel Name cannot be both empty. If both are provided, ID is used if valid, else falls back to Name.
	Id string `json:"id,omitempty"`

	//+kubebuilder:validation:Optional
	// Existing Tunnel name to run on. Tunnel Name and Tunnel ID cannot be both empty. If both are provided, ID is used if valid, else falls back to Name.
	Name string `json:"name,omitempty"`
}

// NewTunnel spec needs a name to create a Tunnel on Cloudflare.
type NewTunnel struct {
	//+kubebuilder:validation:Required
	// Tunnel name to create on Cloudflare.
	Name string `json:"name,omitempty"`
}

// CloudflareDetails spec contains all the necessary parameters needed to connect to the Cloudflare API.
type CloudflareDetails struct {
	//+kubebuilder:validation:Required
	// Cloudflare Domain to which this tunnel belongs to
	Domain string `json:"domain,omitempty"`

	//+kubebuilder:validation:Required
	// Secret containing Cloudflare API key/token
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
	// Key in the secret to use as credentials.json for an existing tunnel, defaults to CLOUDFLARE_TUNNEL_CREDENTIAL_FILE
	CLOUDFLARE_TUNNEL_CREDENTIAL_FILE string `json:"CLOUDFLARE_TUNNEL_CREDENTIAL_FILE,omitempty"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET
	// Key in the secret to use as tunnel secret for an existing tunnel, defaults to CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET
	CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET string `json:"CLOUDFLARE_TUNNEL_CREDENTIAL_SECRET,omitempty"`
}

// TunnelSpec defines the desired state of Tunnel
type TunnelSpec struct {
	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:default:=1
	//+kubebuilder:validation:Optional
	// Size defines the number of Daemon pods to run for this tunnel
	Size int32 `json:"size,omitempty"`

	//+kubebuilder:default:="cloudflare/cloudflared:2022.12.1"
	//+kubebuilder:validation:Optional
	// Image sets the Cloudflared Image to use. Defaults to the image set during the release of the operator.
	Image string `json:"image,omitempty"`

	//+kubebuilder:default:=false
	//+kubebuilder:validation:Optional
	// NoTlsVerify disables origin TLS certificate checks when the endpoint is HTTPS.
	NoTlsVerify bool `json:"noTlsVerify,omitempty"`

	//+kubebuilder:validation:Optional
	// OriginCaPool speficies the secret with tls.crt (and other certs as needed to be referred in the service annotation) of the Root CA to be trusted when sending traffic to HTTPS endpoints
	OriginCaPool string `json:"originCaPool,omitempty"`

	//+kubebuilder:validation:Optional
	//+kubebuilder:default:="http_status:404"
	// FallbackTarget speficies the target for requests that do not match an ingress. Defaults to http_status:404
	FallbackTarget string `json:"fallbackTarget,omitempty"`

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
	TunnelId   string `json:"tunnelId"`
	TunnelName string `json:"tunnelName"`
	AccountId  string `json:"accountId"`
	ZoneId     string `json:"zoneId"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="TunnelID",type=string,JSONPath=`.status.tunnelId`

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
