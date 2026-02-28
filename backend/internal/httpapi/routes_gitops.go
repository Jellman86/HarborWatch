package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gitops"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// gitOpsResponse is a standardized envelope for API responses
type gitOpsResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type toggleDeploymentRequest struct {
	Enabled bool `json:"enabled"`
}

func registerGitOpsRoutes(r chi.Router, deps adminRouteDeps) {
	if deps.db == nil || deps.settingsService == nil {
		// Mocked out in tests, or database not available
		return
	}

	gitStore := gitops.NewStore(deps.db)
	gitService := gitops.NewService(gitStore, deps.settingsService)

	r.Route("/gitops", func(r chi.Router) {

		// SOURCES
		r.Route("/sources", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				sources, err := gitStore.ListSources(r.Context())
				if err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}
				// Mask secrets before returning
				for i := range sources {
					sources[i].AuthSecret = ""
				}
				writeJSON(w, http.StatusOK, sources)
			})

			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				var src gitops.GitSource
				if err := json.NewDecoder(req.Body).Decode(&src); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
					return
				}
				if err := gitops.NormalizeSource(&src); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
					return
				}
				if src.ID == "" {
					src.ID = uuid.NewString()
				}

				if err := gitStore.CreateSource(req.Context(), src); err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}

				// Return the created source, but mask secret
				src.AuthSecret = ""
				writeJSON(w, http.StatusOK, src)
			})

			r.Get("/{id}", func(w http.ResponseWriter, req *http.Request) {
				id := chi.URLParam(req, "id")
				src, err := gitStore.GetSource(req.Context(), id)
				if err != nil {
					writeError(w, http.StatusNotFound, "not_found", err.Error())
					return
				}
				src.AuthSecret = ""
				writeJSON(w, http.StatusOK, src)
			})

			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				id := chi.URLParam(req, "id")
				if err := gitStore.DeleteSource(req.Context(), id); err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, gitOpsResponse{OK: true})
			})

			r.Post("/{id}/sync", func(w http.ResponseWriter, req *http.Request) {
				id := chi.URLParam(req, "id")
				hash, changed, err := gitService.SyncSource(req.Context(), id)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "sync_failed", err.Error())
					return
				}

				// If it changed, auto-deploy its deployments
				if changed {
					// Launch deployment in background or synchronously based on preference.
					// We'll do it synchronously here for simpler feedback.
					deployErrs := gitService.DeployAllForSource(req.Context(), id)
					if len(deployErrs) > 0 {
						var errStrs []string
						for _, e := range deployErrs {
							errStrs = append(errStrs, e.Error())
						}
						writeError(w, http.StatusInternalServerError, "deploy_failed", strings.Join(errStrs, "; "))
						return
					}
				}

				writeJSON(w, http.StatusOK, map[string]any{
					"ok":      true,
					"changed": changed,
					"hash":    hash,
				})
			})
		})

		// DEPLOYMENTS
		r.Route("/deployments", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, req *http.Request) {
				sourceID := req.URL.Query().Get("sourceId")
				if sourceID == "" {
					writeError(w, http.StatusBadRequest, "invalid_request", "sourceId query parameter is required")
					return
				}
				deps, err := gitStore.ListDeploymentsForSource(req.Context(), sourceID)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, deps)
			})

			r.Post("/", func(w http.ResponseWriter, req *http.Request) {
				var dep gitops.GitDeployment
				if err := json.NewDecoder(req.Body).Decode(&dep); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
					return
				}
				if err := gitops.NormalizeDeployment(&dep); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
					return
				}
				if dep.ID == "" {
					dep.ID = uuid.NewString()
				}
				if err := gitStore.CreateDeployment(req.Context(), dep); err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, dep)
			})

			r.Delete("/{id}", func(w http.ResponseWriter, req *http.Request) {
				id := chi.URLParam(req, "id")
				if err := gitStore.DeleteDeployment(req.Context(), id); err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, gitOpsResponse{OK: true})
			})

			r.Patch("/{id}/enabled", func(w http.ResponseWriter, req *http.Request) {
				id := chi.URLParam(req, "id")
				var body toggleDeploymentRequest
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
					return
				}
				if err := gitStore.UpdateDeploymentEnabled(req.Context(), id, body.Enabled); err != nil {
					writeError(w, http.StatusInternalServerError, "db_error", err.Error())
					return
				}
				writeJSON(w, http.StatusOK, gitOpsResponse{OK: true})
			})

			r.Post("/{id}/deploy", func(w http.ResponseWriter, req *http.Request) {
				id := chi.URLParam(req, "id")

				// Need to lookup source ID from deployment
				var sourceID string
				// Quick lookup
				row := deps.db.QueryRowContext(req.Context(), "SELECT git_source_id FROM git_deployments WHERE id = ?", id)
				if err := row.Scan(&sourceID); err != nil {
					writeError(w, http.StatusNotFound, "not_found", "deployment not found")
					return
				}

				if err := gitService.DeployCompose(req.Context(), sourceID, id); err != nil {
					writeError(w, http.StatusInternalServerError, "deploy_failed", err.Error())
					return
				}

				writeJSON(w, http.StatusOK, gitOpsResponse{OK: true})
			})
		})
	})
}
