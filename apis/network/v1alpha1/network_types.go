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

	infrabev1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkSpec defines the desired state of Network
type NetworkSpec struct {
	Topology string `json:"topology" yaml:"topology" protobuf:"bytes,1,opt,name=topology"`
	// BridgeDomains define a set of logical ports that share the same
	// flooding or broadcast characteristics. Like a virtual LAN (VLAN),
	// a bridge domain spans one or more ports of multiple devices.
	BridgeDomains []NetworkBridgeDomain `json:"bridgeDomains,omitempty" yaml:"bridgeDomains,omitempty" protobuf:"bytes,2,rep,name=bridgeDomains"`
	// RoutingTables defines a set of routes belonging to a given routing instance
	// Multiple routing tables are also called virtual routing instances. Each virtual
	// routing instance can hold overlapping IP information
	// A routing table supports both ipv4 and ipv6
	RoutingTables []NetworkRoutingTable `json:"routingTables,omitempty" yaml:"routingTables,omitempty" protobuf:"bytes,3,rep,name=routingTables"`
}

type NetworkBridgeDomain struct {
	// Name defines the name of the bridge domain
	Name string `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	// NetworkID defines the id of the bridge domain
	NetworkID int `json:"networkID,omitempty" yaml:"networkID,omitempty" protobuf:"bytes,2,opt,name=networkID"`
	// Interfaces defines the interfaces belonging to the bridge domain
	Interfaces []NetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
}

type NetworkRoutingTable struct {
	// Name defines the name of the routing table
	Name string `json:"name" yaml:"name"`
	// NetworkID defines the id of the bridge domain
	NetworkID int `json:"networkID,omitempty" yaml:"networkID,omitempty" protobuf:"bytes,2,opt,name=networkID"`
	// Interfaces defines the interfaces belonging to the routing table
	Interfaces []NetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
}

// Network defines the interface parameters
// An interface can be attached to a routingTable and a bridgeDomain.
// Dynamic or static assignments are possible
type NetworkInterface struct {
	// BridgeDomain defines the name of the bridgeDomain belonging to the interface
	// A BridgeDomain can only be attached to a routingTable and is mutualy exclusive with a
	// defined interface Identifier
	BridgeDomain *string `json:"bridgeDomain,omitempty" yaml:"bridgeDomain,omitempty" protobuf:"bytes,1,opt,name=bridgeDomain"`
	// EndpointID defines the name of the interface
	infrabev1alpha1.EndpointID `json:",inline" yaml:",inline" protobuf:"bytes,2,opt,name=endpointID"`
	// IPs define the list of IP addresses on the interface
	IPs []string `json:"ips,omitempty" yaml:"ips,omitempty" protobuf:"bytes,2,opt,name=ips"`
	// VLANID defines the VLAN ID on the interface
	VLANID *uint16 `json:"vlanID,omitempty" yaml:"vlanID,omitempty" protobuf:"bytes,2,opt,name=vlanID"`
	// Selector defines the selector criterias for the interface selection
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty"`
	// VLANTagging defines if the interface is vlanTagged or not
	VLANTagging bool `json:"vlanTagging,omitempty" yaml:"vlanTagging,,omitempty" protobuf:"bytes,5,opt,name=vlanTagging"`
	// Protocols define the protocols parameters for this interface
	Protocols *NetworkInterfaceProtocols `json:"protocols,omitempty" yaml:"protocols,,omitempty" protobuf:"bytes,4,opt,name=protocols"`
}

type NetworkInterfaceProtocols struct {
	BGP *NetworkConfigProtocolsBGP `json:"bgp,omitempty" yaml:"bgp,omitempty" protobuf:"bytes,4,opt,name=bgp"`
}

type NetworkConfigProtocolsBGP struct {
	LocalAS uint32 `json:"localAS,omitempty" yaml:"localAS,omitempty" protobuf:"bytes,1,opt,name=localAS"`
	PeerAS  uint32 `json:"peerAS,omitempty" yaml:"peerAS,omitempty" protobuf:"bytes,2,opt,name=peerAS"`
}

// NetworkStatus defines the observed state of Network
type NetworkStatus struct {
	// ConditionedStatus provides the status of the Network using conditions
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
// Network is the Network for the Network API
// +k8s:openapi-gen=true
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   NetworkSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status NetworkStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +kubebuilder:object:root=true
// NetworkClabList contains a list of NetworkClabs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Network `json:"items" yaml:"items" protobuf:"bytes,2,rep,name=items"`
}

var (
	NetworkKind = reflect.TypeOf(Network{}).Name()
)
