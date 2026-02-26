package dockerengine

import (
	"testing"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func TestBuildNetworkTopologyFromRaw_FiltersDefaultNetworksAndBuildsEdges(t *testing.T) {
	containers := []gen.ContainerSummary{
		{
			ID:    "c1",
			Names: []string{"/web"},
			Image: "ghcr.io/acme/web:latest",
			State: "running",
		},
		{
			ID:    "c2",
			Names: []string{"/db"},
			Image: "postgres:16",
			State: "running",
		},
	}

	list := []networkListJSON{
		{ID: "n-bridge", Name: "bridge", Driver: "bridge", Scope: "local"},
		{ID: "n-app", Name: "app_net", Driver: "bridge", Scope: "local"},
	}
	inspects := []networkInspectJSON{
		{
			ID:         "n-bridge",
			Name:       "bridge",
			Driver:     "bridge",
			Scope:      "local",
			Containers: map[string]networkInspectEndpointJSON{"c1": {Name: "web", IPv4Address: "172.17.0.2/16"}},
		},
		{
			ID:         "n-app",
			Name:       "app_net",
			Driver:     "bridge",
			Scope:      "local",
			Attachable: true,
			IPAM: networkInspectIPAMJSON{
				Config: []networkInspectIPAMConfigJSON{{Subnet: "172.20.0.0/16", Gateway: "172.20.0.1"}},
			},
			Containers: map[string]networkInspectEndpointJSON{
				"c1": {Name: "web", EndpointID: "ep1", MacAddress: "02:42:ac:14:00:02", IPv4Address: "172.20.0.2/16"},
				"c2": {Name: "db", EndpointID: "ep2", MacAddress: "02:42:ac:14:00:03", IPv4Address: "172.20.0.3/16"},
			},
		},
	}

	got := buildNetworkTopologyFromRaw(list, inspects, containers)
	if len(got.Networks) != 1 {
		t.Fatalf("expected 1 user-defined network, got %d", len(got.Networks))
	}
	if got.Networks[0].Name != "app_net" {
		t.Fatalf("expected app_net, got %q", got.Networks[0].Name)
	}
	if len(got.Networks[0].IPAM) != 1 || got.Networks[0].IPAM[0].Subnet != "172.20.0.0/16" {
		t.Fatalf("expected subnet metadata to be present, got %#v", got.Networks[0].IPAM)
	}
	if len(got.Containers) != 2 {
		t.Fatalf("expected 2 deduped containers, got %d", len(got.Containers))
	}
	if len(got.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(got.Edges))
	}
	for _, e := range got.Edges {
		if e.NetworkID != "n-app" {
			t.Fatalf("unexpected network edge target: %#v", e)
		}
	}
}

func TestBuildNetworkTopologyFromRaw_EmptyInputReturnsEmptySlices(t *testing.T) {
	got := buildNetworkTopologyFromRaw(nil, nil, nil)
	if len(got.Networks) != 0 || len(got.Containers) != 0 || len(got.Edges) != 0 {
		t.Fatalf("expected empty topology slices, got %#v", got)
	}
}
