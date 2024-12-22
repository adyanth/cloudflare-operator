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
	"github.com/cloudflare/cloudflare-go"
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

type AccessConfig struct {
	// Enable handling of access configuration
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	Enabled bool `json:"enabled"`
	// Application type self_hosted,saas
	//+kubebuilder:validation:Optional
	//+kubebuilder:validation:Enum:="";"self_hosted";"saas"
	//+kubebuilder:default:="self_hosted"
	Type string `json:"type"`
	// List of access policies
	//+kubebuilder:validation:Optional
	AccessPolicies []AccessPolicy `json:"accessPolicies"`
	// Application settings
	//+kubebuilder:validation:Optional
	Settings AccessConfigSettings `json:"settings"`
}

type AccessConfigSettings struct {
	// Authentication settins
	//+kubebuilder:validation:Optional
	Authentication AccessConfigAuthentication `json:"authentication"`
	// Appearance settins
	//+kubebuilder:validation:Optional
	Appearance AccessConfigAppearance `json:"appearance"`
	// Cookie settings
	//+kubebuilder:validation:Optional
	Cookies AccessConfigCookies `json:"cookies"`
	// Additional settings
	//+kubebuilder:validation:Optional
	Additional AccessConfigAdditional `json:"additional"`
}

type AccessConfigAuthentication struct {
	// The list of identiy providers which application is allowed to use. If empty all idps are allowed
	//+kubebuilder:validation:Optional
	AllowedIdps []string `json:"allowedIdps"`
	// Skip identity provider selection if only one is configured
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	InstantAuth bool `json:"instantAuth"`
	// The amount of time that tokens issued for this application will be valid. Must be in the format 300ms or 2h45m. Valid time units are: ns, us (or µs), ms, s, m, h.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:="24h"
	SessionDuration string `json:"sessionDuration"`
	// The custom URL a user is redirected to when they are denied access to the application.
	//+kubebuilder:validation:Optional
	CustomDenyUrl string `json:"customDenyUrl"`
	// The custom error message shown to a user when they are denied access to the application.
	//+kubebuilder:validation:Optional
	CustomDenyMessage string `json:"customDenyMessage"`
}

type AccessConfigAppearance struct {
	// Wether to show app in the launcher. Defaults to true.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=true
	AppLauncherVisibility bool `json:"appLauncherVisibility"`
	// Custom logo url
	//+kubebuilder:validation:Optional
	CustomLogo string `json:"customLogo"`
}

type AccessConfigCookies struct {
	// Sets the SameSite cookie setting, which provides increased security against CSRF attacks. [none,strict,lax]
	//+kubebuilder:validation:Optional
	//+kubebuilder:validation:Enum:="";"none";"strict";"lax"
	SameSiteAttribute string `json:"sameSiteAttribute"`
	// Enables the HttpOnly cookie attribute, which increases security against XSS attacks.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=true
	EnableHttpOnly bool `json:"enableHttpOnly"`
	// Enables the binding cookie, which increases security against compromised authorization tokens and CSRF attacks.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	EnableBindingCookie bool `json:"enableBindingCookie"`
}

type AccessConfigAdditional struct {
	// Cloudflare will render an SSH terminal or VNC session for this application in a web browser. [ssh,vnc]
	//+kubebuilder:validation:Optional
	//+kubebuilder:validation:Enum:="";"vnc";"ssh"
	BrowserRendering string `json:"browserRendering"`
}

type AccessPolicy struct {
	// The name of the Access policy.
	//+kubebuilder:validation:Required
	Name string `json:"name"`
	// Decision if a policy is met
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Enum:="allow";"deny";"non_identity";"bypass"
	Action string `json:"action"`
	// Array of Access group names. Access groups are not managed by this operator
	//+kubebuilder:validation:Optional
	Include []string `json:"include"`
	// Array of Access group names. Access groups are not managed by this operator
	//+kubebuilder:validation:Optional
	Exclude []string `json:"exclude"`
	// Array of Access group names. Access groups are not managed by this operator
	//+kubebuilder:validation:Optional
	Require []string `json:"require"`
	// The amount of time that tokens issued for the application will be valid. Must be in the format 300ms or 2h45m. Valid time units are: ns, us (or µs), ms, s, m, h.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:="24h"
	// SessionDuration string `json:"sessionDuration"`
	// Require users to enter a justification when they log in to the application.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	PurposeJustificationRequired bool `json:"purposeJustificationRequired"`
	// A custom message that will appear on the purpose justification screen.
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:="Please enter a justification for entering this protected domain."
	PurposeJustificationPrompt string `json:"purposeJustificationPrompt"`
}

func (c *AccessConfig) NewAccessApplication(hostname string) cloudflare.AccessApplication {

	return cloudflare.AccessApplication{
		AllowedIdps:             c.Settings.Authentication.AllowedIdps,
		CustomDenyMessage:       c.Settings.Authentication.CustomDenyMessage,
		LogoURL:                 c.Settings.Appearance.CustomLogo,
		Domain:                  hostname,
		Type:                    cloudflare.AccessApplicationType(c.Type),
		SessionDuration:         c.Settings.Authentication.SessionDuration,
		SameSiteCookieAttribute: c.Settings.Cookies.SameSiteAttribute,
		CustomDenyURL:           c.Settings.Authentication.CustomDenyUrl,
		Name:                    hostname,
		AutoRedirectToIdentity:  &c.Settings.Authentication.InstantAuth,
		AppLauncherVisible:      &c.Settings.Appearance.AppLauncherVisibility,
		EnableBindingCookie:     &c.Settings.Cookies.EnableBindingCookie,
		HttpOnlyCookieAttribute: &c.Settings.Cookies.EnableHttpOnly,
	}
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
	//+kubebuilder:validation:Optional
	AccessConfig AccessConfig        `json:"accessConfig"`
	Status       TunnelBindingStatus `json:"status,omitempty"`
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
