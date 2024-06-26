package clab

import (
	"context"
	"fmt"
	"strings"

	"github.com/henderiw/logger/log"
	"github.com/srl-labs/clabernetes/util/containerlab"

	"github.com/kuidio/kuid/apis/backend"
	infrav1alpha1 "github.com/kuidio/kuid/apis/backend/infra/v1alpha1"
	"github.com/kuidio/kuid/apis/common/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func NewClabKuid(siteID *infrav1alpha1.SiteID, yamlString string) (Clab, error) {
	cfg, err := containerlab.LoadContainerlabConfig(yamlString)
	if err != nil {
		return nil, err
	}

	return &clabkuid{
		siteID: siteID,
		cfg:    cfg,
	}, nil

}

type clabkuid struct {
	siteID *infrav1alpha1.SiteID
	cfg    *containerlab.Config
}

func (r *clabkuid) GetNodes(ctx context.Context) []backend.GenericObject {
	nodes := make([]backend.GenericObject, 0, len(r.cfg.Topology.Nodes))
	for nodeName, n := range r.cfg.Topology.Nodes {
		nodeKind, nodeType := r.cfg.Topology.GetNodeKindType(nodeName)
		nodeGroupNodeID := r.getNodeGroupNodeID(nodeName, n.Labels)

		if len(n.Labels) != 0 {
			if _, ok := n.Labels[backend.KuidINVExclude]; ok {
				continue
			}
		}
		labels := map[string]string{
			backend.KuidINVNodeTypeKey: nodeType,
		}
		for k, v := range n.Labels {
			labels[k] = v
		}

		nodes = append(nodes, infrav1alpha1.BuildNode(
			metav1.ObjectMeta{
				Name:      nodeGroupNodeID.KuidString(),
				Namespace: "default",
			},
			&infrav1alpha1.NodeSpec{
				NodeGroupNodeID: nodeGroupNodeID,
				Rack:            r.getRack(n.Labels),
				Position:        r.getPosition(n.Labels),
				Location:        r.getLocation(n.Labels),
				Provider:        r.getProvider(nodeKind),
				UserDefinedLabels: v1alpha1.UserDefinedLabels{
					Labels: labels,
				},
			},
			nil,
		))
	}
	return nodes
}

func (r *clabkuid) GetLinks(ctx context.Context) []backend.GenericObject {
	log := log.FromContext(ctx)
	links := make([]backend.GenericObject, 0, len(r.cfg.Topology.Links))
	for _, l := range r.cfg.Topology.Links {
		if len(l.Labels) != 0 {
			if _, ok := l.Labels[backend.KuidINVExclude]; ok {
				continue
			}
		}
		epSpecs := r.getEndpoints(ctx, l)
		if epSpecs == nil {
			return nil
		}
		if len(epSpecs) != 2 {
			log.Error("cannot create link if len endpoints != 2", "endpoints", len(epSpecs))
			return nil
		}

		links = append(links, infrav1alpha1.BuildLink(
			metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%s", epSpecs[0].NodeGroupEndpointID.KuidString(), epSpecs[1].NodeGroupEndpointID.KuidString()),
				Namespace: "default",
			},
			&infrav1alpha1.LinkSpec{
				Endpoints: []*infrav1alpha1.NodeGroupEndpointID{
					&epSpecs[0].NodeGroupEndpointID,
					&epSpecs[1].NodeGroupEndpointID,
				},
				UserDefinedLabels: v1alpha1.UserDefinedLabels{
					Labels: l.Labels,
				},
			},
			nil,
		))
	}
	return links
}

func (r *clabkuid) GetEndpoints(ctx context.Context) []backend.GenericObject {
	log := log.FromContext(ctx)
	endpoints := make([]backend.GenericObject, 0, 2*len(r.cfg.Topology.Links))
	for _, l := range r.cfg.Topology.Links {
		if len(l.Labels) != 0 {
			if _, ok := l.Labels[backend.KuidINVExclude]; ok {
				continue
			}
		}
		eps := r.getEndpoints(ctx, l)
		if eps == nil {
			return nil
		}
		if len(eps) != 2 {
			log.Error("cannot create link if len endpoints != 2", "endpoints", len(eps))
			return nil
		}

		for _, epSpec := range eps {
			endpoints = append(endpoints, infrav1alpha1.BuildEndpoint(
				metav1.ObjectMeta{
					Name:      epSpec.KuidString(),
					Namespace: "default",
				},
				epSpec,
				nil,
			))
		}

	}
	return endpoints
}

func (r *clabkuid) getEndpoints(ctx context.Context, l *containerlab.LinkDefinition) []*infrav1alpha1.EndpointSpec {
	log := log.FromContext(ctx)
	endpoints := make([]*infrav1alpha1.EndpointSpec, 0, 2)
	if len(l.Endpoints) != 2 {
		return nil
	}

	for _, nodeEPName := range l.Endpoints {
		parts := strings.Split(nodeEPName, ":")
		if len(parts) != 2 {
			log.Error("cannot get endpoints, wrong nodeEPName, expecting <nodeName>:<epName>", "got", nodeEPName)
			return nil
		}
		nodeName := parts[0]
		epName := parts[1]

		n, ok := r.cfg.Topology.Nodes[nodeName]
		if !ok {
			log.Error("cannot get endpoints, nodeName not found in topology", "nodeName", nodeName)
			return nil
		}

		nodeKind, _ := r.cfg.Topology.GetNodeKindType(nodeName)

		nodeGroupNodeID := r.getNodeGroupNodeID(nodeName, n.Labels)

		endpoints = append(endpoints, &infrav1alpha1.EndpointSpec{
			Provider: r.getProvider(nodeKind),
			NodeGroupEndpointID: infrav1alpha1.NodeGroupEndpointID{
				NodeGroup: r.cfg.Name,
				EndpointID: infrav1alpha1.EndpointID{
					NodeID:   nodeGroupNodeID.NodeID,
					Endpoint: epName,
				},
			},
		})

	}
	return endpoints
}

func (r *clabkuid) getNodeGroupNodeID(nodeName string, labels map[string]string) infrav1alpha1.NodeGroupNodeID {
	return infrav1alpha1.NodeGroupNodeID{
		NodeGroup: r.cfg.Name, // topologyName
		NodeID: infrav1alpha1.NodeID{
			SiteID: infrav1alpha1.SiteID{
				Region: r.getRegion(labels),
				Site:   r.getSite(labels),
			},
			Node: nodeName,
		},
	}
}

func (r *clabkuid) getSite(labels map[string]string) string {
	site, ok := labels[backend.KuidINVSiteKey]
	if ok {
		return site
	}
	return r.siteID.Site
}

func (r *clabkuid) getRegion(labels map[string]string) string {
	region, ok := labels[backend.KuidINVRegionKey]
	if ok {
		return region
	}
	return r.siteID.Region
}

func (r *clabkuid) getRack(labels map[string]string) *string {
	rack, ok := labels[backend.KuidINVRegionKey]
	if ok {
		return ptr.To[string](rack)
	}
	return nil
}

func (r *clabkuid) getPosition(labels map[string]string) *string {
	position, ok := labels[backend.KuidINVPositionKey]
	if ok {
		return ptr.To[string](position)
	}
	return nil
}

func (r *clabkuid) getLocation(labels map[string]string) *infrav1alpha1.Location {
	location, ok := labels[backend.KuidINVLocationKey]
	if ok {
		parts := strings.Split(location, ":")
		if len(parts) != 2 {
			return nil
		}
		return &infrav1alpha1.Location{
			Longitude: parts[0],
			Latitude:  parts[1],
		}
	}
	return nil
}

func (r *clabkuid) getProvider(nodeKind string) string {
	switch nodeKind {
	case "nokia_srlinux":
		return "srlinux.nokia.com"
	case "nokia_sros":
		return "sros.nokia.com"
	}
	return ""
}
