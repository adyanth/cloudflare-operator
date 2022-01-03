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

// TunnelSpec defines the desired state of Tunnel
type TunnelSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of CloudflareTunnel. Edit cloudflaretunnel_types.go to remove/update
	// Foo  string `json:"foo,omitempty"`

	//+kubebuilder:validation:Minimum=0
	//+kubebuilder:default:=1
	//+kubebuilder:validation:Optional
	// Number of Daemon pods to run for this tunnel
	Size int32 `json:"size,omitempty"`

	//+kubebuilder:validation:Required
	// Cloudflare Domain to which this tunnel belongs to
	Domain string `json:"domain,omitempty"`

	//+kubebuilder:validation:Required
	// Tunnel name to give in Cloudflare
	TunnelName string `json:"tunnelName"`

	//+kubebuilder:validation:Optional
	// Existing Tunnel ID to run on
	TunnelId string `json:"tunnelId,omitempty"`
	// TODO: Further attributes to take credentials
}

// TunnelStatus defines the observed state of Tunnel
type TunnelStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Pods      []string `json:"pods"`
	Ingresses []string `json:"ingresses"`
	TunnelId  string   `json:"tunnelId"`
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
