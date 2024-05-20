/*
Copyright 2024 Nokia.

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
	"reflect"

	ipambev1alpha1 "github.com/kuidio/kuid/apis/backend/ipam/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkConfigSpec defines the desired state of NetworkConfig
type NetworkConfigSpec struct {
	Topology string `json:"topology" yaml:"topology" protobuf:"bytes,1,opt,name=topology"`
	// Prefixes defines the prefixes belonging to this network config
	// prefixLength would be indicated by a label
	Prefixes []ipambev1alpha1.Prefix `json:"prefixes,omitempty" yaml:"prefixes,omitempty" protobuf:"bytes,2,rep,name=prefixes"`
	// Addressing defines the addressing used in this network
	// +kubebuilder:validation:Enum=dualstack;ipv4only;ipv6only
	Addressing *string `json:"addressing,omitempty" yaml:"addressing,omitempty" protobuf:"bytes,3,opt,name=addressing"`
	// Protocols define the network wide protocol parameters
	Protocols *NetworkConfigProtocols `json:"protocols,omitempty" yaml:"protocols,,omitempty" protobuf:"bytes,4,opt,name=protocols"`
	// VLANTagging defines if VLAN tagging should be used or not
	VLANTagging bool `json:"vlanTagging,omitempty" yaml:"vlanTagging,,omitempty" protobuf:"bytes,5,opt,name=vlanTagging"`
}

type NetworkConfigProtocols struct {
	OSPF  *NetworkConfigProtocolsOSPF  `json:"ospf,omitempty" yaml:"ospf,omitempty" protobuf:"bytes,1,opt,name=ospf"`
	ISIS  *NetworkConfigProtocolsISIS  `json:"isis,omitempty" yaml:"isis,omitempty" protobuf:"bytes,2,opt,name=isis"`
	IBGP  *NetworkConfigProtocolsIBGP  `json:"ibgp,omitempty" yaml:"ibgp,omitempty" protobuf:"bytes,3,opt,name=ibgp"`
	EBGP  *NetworkConfigProtocolsEBGP  `json:"ebgp,omitempty" yaml:"ebgp,omitempty" protobuf:"bytes,4,opt,name=ebgp"`
	EVPN  *NetworkConfigProtocolsEVPN  `json:"evpn,omitempty" yaml:"evpn,omitempty" protobuf:"bytes,5,opt,name=evpn"`
	IPVPN *NetworkConfigProtocolsIPVPN `json:"ipvpn,omitempty" yaml:"ipvpn,omitempty" protobuf:"bytes,6,opt,name=ipvpn"`
}

type NetworkConfigProtocolsOSPF struct {
}

type NetworkConfigProtocolsISIS struct {
}

type NetworkConfigProtocolsIBGP struct {
	AS              *uint32  `json:"as,omitempty" yaml:"as,omitempty" protobuf:"bytes,1,opt,name=as"`
	LocalAS         bool     `json:"localAS,omitempty" yaml:"localAS,omitempty" protobuf:"bytes,2,opt,name=localAS"`
	RouteReflectors []string `json:"routeReflectors,omitempty" yaml:"routeReflectors,omitempty" protobuf:"bytes,3,opt,name=routeReflectors"`
}

type NetworkConfigProtocolsEBGP struct {
	ASPool *string `json:"asPool,omitempty" yaml:"asPool,omitempty" protobuf:"bytes,3,opt,name=asPool"`
}

type NetworkConfigProtocolsEVPN struct {
}

type NetworkConfigProtocolsIPVPN struct {
}

// NetworkConfigStatus defines the observed state of NetworkConfig
type NetworkConfigStatus struct {
	// ConditionedStatus provides the status of the NetworkConfig using conditions
	// - a ready condition indicates the overall status of the resource
	conditionv1alpha1.ConditionedStatus `json:",inline" yaml:",inline" protobuf:"bytes,1,opt,name=conditionedStatus"`
}

// +kubebuilder:object:root=true
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:resource:categories={kuid, net}
// NetworkConfig is the NetworkConfig for the NetworkConfig API
// +k8s:openapi-gen=true
type NetworkConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   NetworkConfigSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status NetworkConfigStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +kubebuilder:object:root=true
// NetworkConfigClabList contains a list of NetworkConfigClabs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkConfigList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []NetworkConfig `json:"items" yaml:"items" protobuf:"bytes,2,rep,name=items"`
}

var (
	NetworkConfigKind = reflect.TypeOf(NetworkConfig{}).Name()
)
