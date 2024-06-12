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
	// Bridges define a set of logical ports that share the same
	// flooding or broadcast characteristics. Like a virtual LAN (VLAN),
	// a bridge can span one or more ports of multiple devices.
	Bridges []*NetworkBridge `json:"bridges,omitempty" yaml:"bridges,omitempty" protobuf:"bytes,2,rep,name=bridges"`
	// Routers defines a set of routes belonging to a given routing instance
	// A Router can also be called a virtual routing instances. Each virtual
	// routing instance can hold overlapping IP information
	// A router supports both ipv4 and ipv6
	Routers []*NetworkRouter `json:"routers,omitempty" yaml:"routers,omitempty" protobuf:"bytes,3,rep,name=routers"`
}

type NetworkBridge struct {
	// Name defines the name of the bridge domain
	Name string `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	// NetworkID defines the id of the bridge domain
	NetworkID int `json:"networkID,omitempty" yaml:"networkID,omitempty" protobuf:"bytes,2,opt,name=networkID"`
	// Interfaces defines the interfaces belonging to the bridge domain
	Interfaces []*NetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty"`
}

type NetworkRouter struct {
	// Name defines the name of the routing table
	Name string `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	// NetworkID defines the id of router
	NetworkID int `json:"networkID,omitempty" yaml:"networkID,omitempty" protobuf:"bytes,2,opt,name=networkID"`
	// Interfaces defines the interfaces belonging to the routing table
	Interfaces []*NetworkInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty" protobuf:"bytes,3,opt,name=interfaces"`
}

// NetworkInterface defines the interface parameters
// An interface can be attached to a Router and/or a Bridge.
// Dynamic or static assignments are possible
type NetworkInterface struct {
	// Bridge defines the name of the bridge belonging to the interface
	// A bridge can only be attached to a router and is mutualy exclusive with a
	// defined Endpoint
	Bridge *string `json:"bridge,omitempty" yaml:"bridge,omitempty" protobuf:"bytes,1,opt,name=bridge"`
	// Endpoint
	EndPoint *string `json:"endpoint,omitempty" yaml:"endpoint,omitempty" protobuf:"bytes,2,opt,name=endpoint"`
	// NodeID defines the node identifier
	// if omitted only possible with a bridge
	*infrabev1alpha1.NodeID `json:",inline" yaml:",inline" protobuf:"bytes,3,opt,name=nodeID"`
	// IPs define the list of IP addresses on the interface
	Addresses []*NetworkInterfaceAddress `json:"addresses,omitempty" yaml:"addresses,omitempty" protobuf:"bytes,4,opt,name=ipsaddresses"`
	// VLANID defines the VLAN ID on the interface
	VLANID *uint32 `json:"vlanID,omitempty" yaml:"vlanID,omitempty" protobuf:"bytes,5,opt,name=vlanID"`
	// Selector defines the selector criterias for the interface selection
	// Used for dynamic interface selection
	Selector *metav1.LabelSelector `json:"selector,omitempty" yaml:"selector,omitempty" protobuf:"bytes,6,opt,name=selector"`
	// VLANTagging defines if the interface is vlanTagged or not
	// Used for dynamic interface selection
	VLANTagging bool `json:"vlanTagging,omitempty" yaml:"vlanTagging,,omitempty" protobuf:"bytes,7,opt,name=vlanTagging"`
	// Protocols define the protocols parameters for this interface
	Protocols *NetworkInterfaceProtocols `json:"protocols,omitempty" yaml:"protocols,,omitempty" protobuf:"bytes,8,opt,name=protocols"`
}

type NetworkInterfaceAddress struct {
	Address   string  `json:"address" yaml:"address" protobuf:"bytes,1,opt,name=address"`
	Attribute *string `json:"attribute,omitempty" yaml:"address,omitempty" protobuf:"bytes,2,opt,name=attribute"`
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

	DevicesConfigStatus []*NetworkStatusDeviceStatus `json:"devicesConfigStatus,omitempty" yaml:"devicesConfigStatus,omitempty" protobuf:"bytes,2,opt,name=devicesConfigStatus"`
	DevicesDeployStatus []*NetworkStatusDeviceStatus `json:"devicesDeployStatus,omitempty" yaml:"devicesDeployStatus,omitempty" protobuf:"bytes,3,opt,name=devicesDeployStatus"`

	// UsedReferences track the resource used to determine if a change to the resources was identified
	// If a change is detected a reconcile will be triggered and the child status will be reset
	UsedReferences *NetworkStatusUsedReferences `json:"usedReferences,omitempty" yaml:"usedReferences,omitempty" protobuf:"bytes,4,opt,name=usedReferences"`
}

type NetworkStatusDeviceStatus struct {
	Node   string  `json:"node" yaml:"node" protobuf:"bytes,1,opt,name=node"`
	Ready  bool    `json:"ready" yaml:"ready" protobuf:"bytes,2,opt,name=ready"`
	Reason *string `json:"reason,omitempty" yaml:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`
}

type NetworkStatusUsedReferences struct {
	NetworkSpecHash       string `json:"networkSpecHash" yaml:"networkSpecHash" protobuf:"bytes,1,opt,name=networkSpecHash"`
	NetworkDesignResourceVersion string `json:"networkDesignResourceVersion" yaml:"networkDesignResourceVersion" protobuf:"bytes,2,opt,name=networkDesignResourceVersion"`
}

// +kubebuilder:object:root=true
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="PARAM-READY",type="string",JSONPath=".status.conditions[?(@.type=='NetworkPararmReady')].status"
// +kubebuilder:printcolumn:name="DEVICES-READY",type="string",JSONPath=".status.conditions[?(@.type=='NetworkDeviceReady')].status"
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
