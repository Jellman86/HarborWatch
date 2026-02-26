package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func registerComposeRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/compose", func(r chi.Router) {
		r.Get("/projects", func(w http.ResponseWriter, r *http.Request) {
			if deps.dockerClient == nil {
				writeError(w, http.StatusServiceUnavailable, "docker_unavailable", "Docker client unavailable")
				return
			}
			containers, err := deps.dockerClient.ListContainers(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "docker_error", err.Error())
				return
			}
			snapshotRoot := ""
			if deps.settingsService != nil {
				if st, err := deps.settingsService.Get(r.Context()); err == nil {
					snapshotRoot = strings.TrimSpace(st.ComposeSnapshotRootPath)
				}
			}
			writeJSON(w, http.StatusOK, discoverLocalComposeProjectsWithSnapshotRoot(containers, snapshotRoot))
		})
	})
}
