package dockerengine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type NetworkTopologySnapshot struct {
	GeneratedAt int64                      `json:"generatedAt"`
	Networks    []NetworkTopologyNetwork   `json:"networks"`
	Containers  []NetworkTopologyContainer `json:"containers"`
	Edges       []NetworkTopologyEdge      `json:"edges"`
}

type NetworkTopologyNetwork struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Driver     string                     `json:"driver,omitempty"`
	Scope      string                     `json:"scope,omitempty"`
	Internal   bool                       `json:"internal,omitempty"`
	Attachable bool                       `json:"attachable,omitempty"`
	Ingress    bool                       `json:"ingress,omitempty"`
	IPAM       []NetworkTopologyIPAMBlock `json:"ipam,omitempty"`
	Labels     map[string]string          `json:"labels,omitempty"`
}

type NetworkTopologyIPAMBlock struct {
	Subnet  string `json:"subnet,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

type NetworkTopologyContainer struct {
	ID     string            `json:"id"`
	Name   string            `json:"name,omitempty"`
	Image  string            `json:"image,omitempty"`
	State  string            `json:"state,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type NetworkTopologyEdge struct {
	NetworkID    string `json:"networkId"`
	ContainerID  string `json:"containerId"`
	EndpointID   string `json:"endpointId,omitempty"`
	EndpointName string `json:"endpointName,omitempty"`
	MacAddress   string `json:"macAddress,omitempty"`
	IPv4Address  string `json:"ipv4Address,omitempty"`
	IPv6Address  string `json:"ipv6Address,omitempty"`
}

func (c *Client) GetNetworkTopology(ctx context.Context) (NetworkTopologySnapshot, error) {
	var list []networkListJSON
	if err := c.getJSON(ctx, "/networks", &list); err != nil {
		return NetworkTopologySnapshot{}, err
	}

	inspects := make([]networkInspectJSON, 0, len(list))
	for _, n := range list {
		if !isUserDefinedNetwork(n) {
			continue
		}
		var inspect networkInspectJSON
		if err := c.getJSON(ctx, "/networks/"+n.ID, &inspect); err != nil {
			return NetworkTopologySnapshot{}, fmt.Errorf("inspect network %s: %w", n.ID, err)
		}
		inspects = append(inspects, inspect)
	}

	containers, err := c.ListContainers(ctx)
	if err != nil {
		containers = nil // topology can still be built from network inspect attachment names
	}

	out := buildNetworkTopologyFromRaw(list, inspects, containers)
	out.GeneratedAt = time.Now().UTC().Unix()
	return out, nil
}

func buildNetworkTopologyFromRaw(list []networkListJSON, inspects []networkInspectJSON, containers []gen.ContainerSummary) NetworkTopologySnapshot {
	allowedUserDefinedIDs := map[string]struct{}{}
	for _, n := range list {
		if isUserDefinedNetwork(n) && strings.TrimSpace(n.ID) != "" {
			allowedUserDefinedIDs[strings.TrimSpace(n.ID)] = struct{}{}
		}
	}

	containerMeta := map[string]gen.ContainerSummary{}
	for _, c := range containers {
		if strings.TrimSpace(c.ID) == "" {
			continue
		}
		containerMeta[strings.TrimSpace(c.ID)] = c
	}

	networks := make([]NetworkTopologyNetwork, 0, len(inspects))
	edges := make([]NetworkTopologyEdge, 0, 16)
	containerNodes := map[string]NetworkTopologyContainer{}

	for _, inspect := range inspects {
		if strings.TrimSpace(inspect.ID) == "" {
			continue
		}
		if len(allowedUserDefinedIDs) > 0 {
			if _, ok := allowedUserDefinedIDs[strings.TrimSpace(inspect.ID)]; !ok {
				continue
			}
		}
		if !isUserDefinedNetwork(networkListJSON{Name: inspect.Name}) {
			continue
		}
		networks = append(networks, toNetworkNode(inspect))
		for containerID, ep := range inspect.Containers {
			id := strings.TrimSpace(containerID)
			if id == "" {
				continue
			}
			if _, ok := containerNodes[id]; !ok {
				node := NetworkTopologyContainer{
					ID:   id,
					Name: strings.TrimSpace(ep.Name),
				}
				if meta, ok := containerMeta[id]; ok {
					node.Name = trimContainerSummaryName(meta)
					node.Image = strings.TrimSpace(meta.Image)
					node.State = strings.TrimSpace(meta.State)
					if len(meta.Labels) > 0 {
						node.Labels = meta.Labels
					}
				}
				containerNodes[id] = node
			}
			edges = append(edges, NetworkTopologyEdge{
				NetworkID:    strings.TrimSpace(inspect.ID),
				ContainerID:  id,
				EndpointID:   strings.TrimSpace(ep.EndpointID),
				EndpointName: strings.TrimSpace(ep.Name),
				MacAddress:   strings.TrimSpace(ep.MacAddress),
				IPv4Address:  strings.TrimSpace(ep.IPv4Address),
				IPv6Address:  strings.TrimSpace(ep.IPv6Address),
			})
		}
	}

	containerList := make([]NetworkTopologyContainer, 0, len(containerNodes))
	for _, c := range containerNodes {
		containerList = append(containerList, c)
	}

	sort.Slice(networks, func(i, j int) bool {
		li := strings.ToLower(networks[i].Name)
		lj := strings.ToLower(networks[j].Name)
		if li == lj {
			return networks[i].ID < networks[j].ID
		}
		return li < lj
	})
	sort.Slice(containerList, func(i, j int) bool {
		li := strings.ToLower(containerList[i].Name)
		lj := strings.ToLower(containerList[j].Name)
		if li == lj {
			return containerList[i].ID < containerList[j].ID
		}
		return li < lj
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].NetworkID != edges[j].NetworkID {
			return edges[i].NetworkID < edges[j].NetworkID
		}
		return edges[i].ContainerID < edges[j].ContainerID
	})

	return NetworkTopologySnapshot{
		Networks:   networks,
		Containers: containerList,
		Edges:      edges,
	}
}

func toNetworkNode(in networkInspectJSON) NetworkTopologyNetwork {
	out := NetworkTopologyNetwork{
		ID:         strings.TrimSpace(in.ID),
		Name:       strings.TrimSpace(in.Name),
		Driver:     strings.TrimSpace(in.Driver),
		Scope:      strings.TrimSpace(in.Scope),
		Internal:   in.Internal,
		Attachable: in.Attachable,
		Ingress:    in.Ingress,
	}
	if len(in.Labels) > 0 {
		out.Labels = in.Labels
	}
	if len(in.IPAM.Config) > 0 {
		out.IPAM = make([]NetworkTopologyIPAMBlock, 0, len(in.IPAM.Config))
		for _, cfg := range in.IPAM.Config {
			if strings.TrimSpace(cfg.Subnet) == "" && strings.TrimSpace(cfg.Gateway) == "" {
				continue
			}
			out.IPAM = append(out.IPAM, NetworkTopologyIPAMBlock{
				Subnet:  strings.TrimSpace(cfg.Subnet),
				Gateway: strings.TrimSpace(cfg.Gateway),
			})
		}
	}
	return out
}

func trimContainerSummaryName(c gen.ContainerSummary) string {
	if len(c.Names) == 0 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(c.Names[0]), "/")
}

func isUserDefinedNetwork(n networkListJSON) bool {
	name := strings.ToLower(strings.TrimSpace(n.Name))
	switch name {
	case "", "bridge", "host", "none", "ingress", "docker_gwbridge":
		return false
	}
	return true
}

type networkListJSON struct {
	ID         string            `json:"Id"`
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Scope      string            `json:"Scope"`
	Internal   bool              `json:"Internal"`
	Attachable bool              `json:"Attachable"`
	Ingress    bool              `json:"Ingress"`
	Labels     map[string]string `json:"Labels"`
}

type networkInspectJSON struct {
	ID         string                                `json:"Id"`
	Name       string                                `json:"Name"`
	Driver     string                                `json:"Driver"`
	Scope      string                                `json:"Scope"`
	Internal   bool                                  `json:"Internal"`
	Attachable bool                                  `json:"Attachable"`
	Ingress    bool                                  `json:"Ingress"`
	Labels     map[string]string                     `json:"Labels"`
	IPAM       networkInspectIPAMJSON                `json:"IPAM"`
	Containers map[string]networkInspectEndpointJSON `json:"Containers"`
}

type networkInspectIPAMJSON struct {
	Config []networkInspectIPAMConfigJSON `json:"Config"`
}

type networkInspectIPAMConfigJSON struct {
	Subnet  string `json:"Subnet"`
	Gateway string `json:"Gateway"`
}

type networkInspectEndpointJSON struct {
	Name        string `json:"Name"`
	EndpointID  string `json:"EndpointID"`
	MacAddress  string `json:"MacAddress"`
	IPv4Address string `json:"IPv4Address"`
	IPv6Address string `json:"IPv6Address"`
}
