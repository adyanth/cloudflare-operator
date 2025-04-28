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

// AccessServiceConfig defines the Service created to Access
type AccessServiceConfig struct {
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
	Port uint `json:"port,omitempty"`
}

// AccessTarget defines the desired state of Access
type AccessTarget struct {
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
	Svc AccessServiceConfig `json:"svc,omitempty"`
}

// AccessServiceToken defines the access auth if needed
type AccessServiceToken struct {
	// Access Service Token ID
	// +kubebuilder:validation:Required
	Id string `json:"id,omitempty"`
	// Access Service Token
	// +kubebuilder:validation:Required
	Token string `json:"token,omitempty"`
}

// AccessStatus defines the observed state of Access
type AccessStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=`.target.fqdn`

// Access is the Schema for the accesses API
type Access struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Target       AccessTarget       `json:"target,omitempty"`
	ServiceToken AccessServiceToken `json:"serviceToken,omitempty"`
	Status       AccessStatus       `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccessList contains a list of Access
type AccessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Access `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Access{}, &AccessList{})
}
