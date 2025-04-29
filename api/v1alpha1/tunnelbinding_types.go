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

// TunnelBindingSubject defines the subject TunnelBinding connects to the Tunnel
type TunnelBindingSubject struct {
	// Kind can be Service
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:="Service"
	Kind string `json:"kind"`
	//+kubebuilder:validation:Required
	Name string `json:"name"`
	//+kubebuilder:validation:Optional
	Spec TunnelBindingSubjectSpec `json:"spec"`
}

type TunnelBindingSubjectSpec struct {
	// Fqdn specifies the DNS name to access this service from.
	// Defaults to the service.metadata.name + tunnel.spec.domain.
	// If specifying this, make sure to use the same domain that the tunnel belongs to.
	// This is not validated and used as provided
	//+kubebuilder:validation:Optional
	Fqdn string `json:"fqdn,omitempty"`

	// Protocol specifies the protocol for the service. Should be one of http, https, tcp, udp, ssh or rdp.
	// Defaults to http, with the exceptions of https for 443, smb for 139 and 445, rdp for 3389 and ssh for 22 if the service has a TCP port.
	// The only available option for a UDP port is udp, which is default.
	//+kubebuilder:validation:Optional
	Protocol string `json:"protocol,omitempty"`

	// Path specifies a regular expression for to match on the request for http/https services
	// If a rule does not specify a path, all paths will be matched.
	//+kubebuilder:validation:Optional
	Path string `json:"path,omitempty"`

	// Target specified where the tunnel should proxy to.
	// Defaults to the form of <protocol>://<service.metadata.name>.<service.metadata.namespace>.svc:<port>
	//+kubebuilder:validation:Optional
	Target string `json:"target,omitempty"`

	// CaPool trusts the CA certificate referenced by the key in the secret specified in tunnel.spec.originCaPool.
	// tls.crt is trusted globally and does not need to be specified. Only useful if the protocol is HTTPS.
	//+kubebuilder:validation:Optional
	CaPool string `json:"caPool,omitempty"`

	// NoTlsVerify disables TLS verification for this service.
	// Only useful if the protocol is HTTPS.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	NoTlsVerify bool `json:"noTlsVerify"`

	// Http2Origin makes the service attempt to connect to origin using HTTP2.
	// Origin must be configured as https.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	Http2Origin bool `json:"http2Origin"`

	// cloudflared starts a proxy server to translate HTTP traffic into TCP when proxying, for example, SSH or RDP.

	// ProxyAddress configures the listen address for that proxy
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:="127.0.0.1"
	//+kubebuilder:validation:Pattern="((^((([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]))$)|(^(([0-9a-fA-F]{1,4}:){7,7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:))$))"
	ProxyAddress string `json:"proxyAddress,omitempty"`

	// ProxyPort configures the listen port for that proxy
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=0
	//+kubebuilder:validation:Minimum:=0
	//+kubebuilder:validation:Maximum:=65535
	ProxyPort uint `json:"proxyPort,omitempty"`

	// ProxyType configures the proxy type.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=""
	//+kubebuilder:validation:Enum:="";"socks"
	ProxyType string `json:"proxyType,omitempty"`
}

// TunnelRef defines the Tunnel TunnelBinding connects to
type TunnelRef struct {
	// Kind can be Tunnel or ClusterTunnel
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Enum:="ClusterTunnel";"Tunnel"
	Kind string `json:"kind"`
	// Name of the tunnel resource
	//+kubebuilder:validation:Required
	Name string `json:"name"`

	//+kubebuilder:validation:Optional
	// DisableDNSUpdates disables the DNS updates on Cloudflare, just managing the configs. Assumes the DNS entries are manually added.
	DisableDNSUpdates bool `json:"disableDNSUpdates"`
}

// ServiceInfo stores the Hostname and Target for each service
type ServiceInfo struct {
	// FQDN of the service
	Hostname string `json:"hostname"`
	// Target for cloudflared
	Target string `json:"target"`
}

// TunnelBindingStatus defines the observed state of TunnelBinding
type TunnelBindingStatus struct {
	// To show on the kubectl cli
	Hostnames string        `json:"hostnames"`
	Services  []ServiceInfo `json:"services"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="FQDNs",type=string,JSONPath=`.status.hostnames`

// TunnelBinding is the Schema for the tunnelbindings API
type TunnelBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Subjects  []TunnelBindingSubject `json:"subjects"`
	TunnelRef TunnelRef              `json:"tunnelRef"`
	Status    TunnelBindingStatus    `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TunnelBindingList contains a list of TunnelBinding
type TunnelBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TunnelBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TunnelBinding{}, &TunnelBindingList{})
}
