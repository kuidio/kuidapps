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

	conditionv1alpha1 "github.com/kuidio/kuid/apis/condition/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkDeviceDeviceSpec defines the desired state of NetworkDevice
type NetworkDeviceSpec struct {
	Topology string `json:"topology" yaml:"topology" protobuf:"bytes,1,opt,name=topology"`
	// Provider defines the provider implementing this resource.
	Provider string `json:"provider" yaml:"provider" protobuf:"bytes,2,opt,name=provider"`
	// Interfaces defines the interfaces for the device config
	// +optional
	Interfaces []*NetworkDeviceInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty" protobuf:"bytes,3,rep,name=interfaces"`
	// NetworkInstances defines the network instances for the device config
	// +optional
	NetworkInstances []*NetworkDeviceNetworkInstance `json:"networkInstances,omitempty" yaml:"networkInstances,omitempty" protobuf:"bytes,4,rep,name=networkInstances"`
	// TunnelInterfaces defines the unnelInterfaces for the device config
	// +optional
	TunnelInterfaces []*NetworkDeviceTunnelInterface `json:"tunnelInterfaces,omitempty" yaml:"tunnelInterfaces,omitempty" protobuf:"bytes,5,rep,name=tunnelInterfaces"`
	// RoutingPolicies defines the routingPolicies for the device config
	// +optional
	RoutingPolicies []*NetworkDeviceRoutingPolicy `json:"routingPolicies,omitempty" yaml:"routingPolicies,omitempty" protobuf:"bytes,6,opt,name=routingPolicies"`
	// System defines the system parameters for the device config
	System *NetworkDeviceSystem `json:"system,omitempty" yaml:"system,omitempty" protobuf:"bytes,7,opt,name=system"`
}

type NetworkDeviceRoutingPolicy struct {
	Name         string   `json:"name,omitempty" yaml:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	IPv4Prefixes []string `json:"ipv4Prefixes,omitempty" yaml:"ipv4Prefixes,omitempty" protobuf:"bytes,2,rep,name=ipv4Prefixes"`
	IPv6Prefixes []string `json:"ipv6Prefixes,omitempty" yaml:"ipv6Prefixes,omitempty" protobuf:"bytes,3,rep,name=ipv6Prefixes"`
}

// TODO LAG, etc
type NetworkDeviceInterface struct {
	Name          string                                `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	SubInterfaces []*NetworkDeviceInterfaceSubInterface `json:"subInterfaces,omitempty" yaml:"subInterfaces,omitempty" protobuf:"bytes,3,rep,name=subInterfaces"`
	VLANTagging   bool                                  `json:"vlanTagging" yaml:"vlanTagging" protobuf:"bytes,4,opt,name=vlanTagging"`
	Speed         string                                `json:"speed" yaml:"speed" protobuf:"bytes,5,opt,name=speed"`
	LAGMember     bool                                  `json:"lagMember" yaml:"lagMember" protobuf:"bytes,6,opt,name=lagMember"`
}

type NetworkDeviceTunnelInterface struct {
	Name          string                                      `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	SubInterfaces []*NetworkDeviceTunnelInterfaceSubInterface `json:"subInterfaces,omitempty" yaml:"subInterfaces,omitempty" protobuf:"bytes,2,rep,name=subInterfaces"`
}

type NetworkDeviceInterfaceSubInterface struct {
	PeerName string `json:"peerName" yaml:"peerName" protobuf:"bytes,1,opt,name=peerName"`
	ID       uint32 `json:"id" yaml:"id" protobuf:"bytes,2,opt,name=id"`
	// routed or bridged
	Type SubInterfaceType                        `json:"type" yaml:"type" protobuf:"bytes,3,opt,name=type"`
	VLAN *uint32                                 `json:"vlan,omitempty" yaml:"vlan,omitempty" protobuf:"bytes,4,opt,name=vlan"`
	IPv4 *NetworkDeviceInterfaceSubInterfaceIPv4 `json:"ipv4,omitempty" yaml:"ipv4,omitempty" protobuf:"bytes,5,rep,name=ipv4"`
	IPv6 *NetworkDeviceInterfaceSubInterfaceIPv6 `json:"ipv6,omitempty" yaml:"ipv6,omitempty" protobuf:"bytes,6,rep,name=ipv6"`
}

type NetworkDeviceTunnelInterfaceSubInterface struct {
	ID uint32 `json:"id" yaml:"id" protobuf:"bytes,1,opt,name=id"`
	// routed or bridged
	Type SubInterfaceType `json:"type" yaml:"type" protobuf:"bytes,2,opt,name=type"`
}

type NetworkDeviceInterfaceSubInterfaceIPv4 struct {
	Addresses []string `json:"addresses" yaml:"addresses" protobuf:"bytes,1,opt,name=addresses"`
}

type NetworkDeviceInterfaceSubInterfaceIPv6 struct {
	Addresses []string `json:"addresses" yaml:"addresses" protobuf:"bytes,1,opt,name=addresses"`
}

type NetworkDeviceNetworkInstance struct {
	Name string `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	// mac-vrf, ip-vrf
	Type           NetworkInstanceType                      `json:"type" yaml:"type" protobuf:"bytes,2,opt,name=type"`
	Protocols      *NetworkDeviceNetworkInstanceProtocols   `json:"protocols,omitempty" yaml:"protocols,omitempty" protobuf:"bytes,3,opt,name=protocols"`
	Interfaces     []*NetworkDeviceNetworkInstanceInterface `json:"interfaces,omitempty" yaml:"interfaces,omitempty" protobuf:"bytes,4,opt,name=interfaces"`
	VXLANInterface *NetworkDeviceNetworkInstanceInterface   `json:"vxlanInterface,omitempty" yaml:"vxlanInterface,omitempty" protobuf:"bytes,5,opt,name=vxlanInterface"`
}

type NetworkDeviceNetworkInstanceInterface struct {
	Name string `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	ID   uint32 `json:"id" yaml:"id" protobuf:"bytes,2,opt,name=id"`
}

type NetworkDeviceNetworkInstanceProtocols struct {
	BGP     *NetworkDeviceNetworkInstanceProtocolBGP     `json:"bgp,omitempty" yaml:"bgp,omitempty" protobuf:"bytes,1,opt,name=bgp"`
	BGPEVPN *NetworkDeviceNetworkInstanceProtocolBGPEVPN `json:"bgpEVPN,omitempty" yaml:"bgpEVPN,omitempty" protobuf:"bytes,2,opt,name=bgpEVPN"`
	BGPVPN  *NetworkDeviceNetworkInstanceProtocolBGPVPN  `json:"bgpVPN,omitempty" yaml:"bgpVPN,omitempty" protobuf:"bytes,2,opt,name=bgpVPN"`
}

type NetworkDeviceNetworkInstanceProtocolBGP struct {
	AS               uint32                                                   `json:"as" yaml:"as" protobuf:"bytes,1,opt,name=as"`
	RouterID         string                                                   `json:"routerID" yaml:"routerID" protobuf:"bytes,2,opt,name=routerID"`
	PeerGroups       []*NetworkDeviceNetworkInstanceProtocolBGPPeerGroup      `json:"peerGroups,omitempty" yaml:"peerGroups,omitempty" protobuf:"bytes,3,opt,name=peerGroups"`
	Neighbors        []*NetworkDeviceNetworkInstanceProtocolBGPNeighbor       `json:"neighbors,omitempty" yaml:"neighbors,omitempty" protobuf:"bytes,4,opt,name=neighbors"`
	DynamicNeighbors *NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors `json:"dynamicNeighbors,omitempty" yaml:"dynamicNeighbors,omitempty" protobuf:"bytes,5,opt,name=dynamicNeighbors"`
}

type NetworkDeviceNetworkInstanceProtocolBGPEVPN struct {
	EVI            uint32 `json:"evi" yaml:"evi" protobuf:"bytes,1,opt,name=evi"`
	ECMP           uint32 `json:"ecmp" yaml:"ecmp" protobuf:"bytes,2,opt,name=ecmp"`
	VXLANInterface string `json:"vxlanInterface" yaml:"vxlanInterface" protobuf:"bytes,2,opt,name=vxlanInterface"`
}

type NetworkDeviceNetworkInstanceProtocolBGPVPN struct {
	ImportRouteTarget string `json:"importRouteTarget" yaml:"importRouteTarget" protobuf:"bytes,1,opt,name=importRouteTarget"`
	ExportRouteTarget string `json:"exportRouteTarget" yaml:"exportRouteTarget" protobuf:"bytes,2,opt,name=exportRouteTarget"`
}

type NetworkDeviceNetworkInstanceProtocolBGPPeerGroup struct {
	Name            string                                                          `json:"name" yaml:"name" protobuf:"bytes,1,opt,name=name"`
	AddressFamilies []string                                                        `json:"addressFamilies,omitempty" yaml:"addressFamilies,omitempty" protobuf:"bytes,2,rep,name=addressFamilies"`
	RouteReflector  *NetworkDeviceNetworkInstanceProtocolBGPPeerGroupRouteReflector `json:"routeReflector,omitempty" yaml:"routeReflector,omitempty" protobuf:"bytes,3,opt,name=routeReflector"`
}

type NetworkDeviceNetworkInstanceProtocolBGPPeerGroupRouteReflector struct {
	ClusterID string `json:"clusterID" yaml:"clusterID" protobuf:"bytes,1,opt,name=clusterID"`
}

type NetworkDeviceNetworkInstanceProtocolBGPNeighbor struct {
	PeerAddress  string `json:"peerAddress" yaml:"peerAddress" protobuf:"bytes,1,opt,name=peerAddress"`
	PeerAS       uint32 `json:"peerAS" yaml:"peerAS" protobuf:"bytes,2,opt,name=peerAS"`
	PeerGroup    string `json:"peerGroup" yaml:"peerGroup" protobuf:"bytes,3,opt,name=peerGroup"`
	LocalAS      uint32 `json:"localAS" yaml:"localAS" protobuf:"bytes,4,opt,name=localAS"`
	LocalAddress string `json:"localAddress" yaml:"localAddress" protobuf:"bytes,5,opt,name=localAddress"`
}

type NetworkDeviceNetworkInstanceProtocolBGPDynamicNeighbors struct {
	PeerPrefixes []string `json:"peerPrefixes" yaml:"peerPrefixes" protobuf:"bytes,1,opt,name=peerPrefixes"`
	PeerAS       uint32   `json:"peerAS" yaml:"peerAS" protobuf:"bytes,2,opt,name=peerAS"`
	PeerGroup    string   `json:"peerGroup" yaml:"peerGroup" protobuf:"bytes,3,opt,name=peerGroup"`
}

type NetworkDeviceSystem struct {
	Protocols *NetworkDeviceSystemProtocols `json:"protocols,omitempty" yaml:"protocols,omitempty" protobuf:"bytes,7,opt,name=protocols"`
}

type NetworkDeviceSystemProtocols struct {
	BGPVPN  *NetworkDeviceSystemProtocolsBGPVPN  `json:"bgpVPN,omitempty" yaml:"bgpVPN,omitempty" protobuf:"bytes,7,opt,name=bgpVPN"`
	BGPEVPN *NetworkDeviceSystemProtocolsBGPEVPN `json:"bgpEVPN,omitempty" yaml:"bgpEVPN,omitempty" protobuf:"bytes,7,opt,name=bgpEVPN"`
}

type NetworkDeviceSystemProtocolsBGPVPN struct {
}

type NetworkDeviceSystemProtocolsBGPEVPN struct {
}

// NetworkDeviceStatus defines the observed state of NetworkDevice
type NetworkDeviceStatus struct {
	// ConditionedStatus provides the status of the NetworkDevice using conditions
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
// +kubebuilder:printcolumn:name="PROVIDER",type="string",JSONPath=".spec.provider"
// +kubebuilder:resource:categories={kuid, net}
// NetworkDevice is the NetworkDevice for the NetworkDevice API
// +k8s:openapi-gen=true
type NetworkDevice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   NetworkDeviceSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status NetworkDeviceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +kubebuilder:object:root=true
// NetworkDeviceClabList contains a list of NetworkDeviceClabs
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkDeviceList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []NetworkDevice `json:"items" yaml:"items" protobuf:"bytes,2,rep,name=items"`
}

var (
	NetworkDeviceKind = reflect.TypeOf(NetworkDevice{}).Name()
)
