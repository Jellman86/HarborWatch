package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerNetworkRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/networks", func(r chi.Router) {
		r.Get("/topology", func(w http.ResponseWriter, r *http.Request) {
			if deps.dockerClient == nil {
				writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker client unavailable")
				return
			}
			topology, err := deps.dockerClient.GetNetworkTopology(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "docker_network_topology_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, topology)
		})
	})
}
