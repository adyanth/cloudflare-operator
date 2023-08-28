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

// AccessServiceSpec defines the desired state of AccessService
type AccessServiceSpec struct {
	// FQDN to connect to for the TCP tunnel
	//+kubebuilder:validation:Required
	Hostname string `json:"hostname"`

	// Protocol defines the protocol to use, only TCP for now, default
	//+kubebuilder:validation:Enum:="tcp";"udp"
	//+kubebuilder:default="tcp"
	Protocol string `json:"protocol"`

	// Port defines the port for the service to listen on
	//+kubebuilder:validation:Minimum:=1
	//+kubebuilder:validation:Maximum:=65535
	Port int32 `json:"port"`

	// ServiceName defines the name of the service for this port to be exposed on
	//+kubebuilder:validation:Required
	ServiceName string `json:"serviceName"`

	// Replicas defines the number of cloudflared access replicas to run
	Replicas int32 `json:"replicas"`
}

// AccessServiceStatus defines the observed state of AccessService
type AccessServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AccessService is the Schema for the accessservices API
type AccessService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessServiceSpec   `json:"spec,omitempty"`
	Status AccessServiceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AccessServiceList contains a list of AccessService
type AccessServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AccessService{}, &AccessServiceList{})
}
