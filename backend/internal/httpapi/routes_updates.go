package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/go-chi/chi/v5"
)

func registerUpdateRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/updates", func(r chi.Router) {
		r.Post("/run", func(w http.ResponseWriter, r *http.Request) {
			if deps.updateService == nil {
				writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
				return
			}
			var req gen.UpdateStartRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON payload")
				return
			}
			req.ContainerID = strings.TrimSpace(req.ContainerID)
			req.TargetImage = strings.TrimSpace(req.TargetImage)
			req.ValidateURL = strings.TrimSpace(req.ValidateURL)
			var currentPortainerService PortainerClient
			if deps.currentPortainerState != nil {
				currentPortainerService = *deps.currentPortainerState
			}
			built, err := buildUpdateRequestForContainer(
				r.Context(),
				req.ContainerID,
				req.TargetImage,
				req.ValidateURL,
				req.ValidateMode,
				req.ValidateTimeoutSec,
				req.ValidateIntervalSec,
				req.BypassAI,
				req.SkipHealthCheck,
				deps.dockerClient,
				currentPortainerService,
				deps.rulesService,
				deps.settingsService,
				deps.intelService,
				deps.releaseService,
				deps.diagService,
				deps.gitOpsLookup,
				updateRequestBuildOptions{
					EnforceLocked: true,
				},
			)
			if err != nil {
				switch {
				case errors.Is(err, ErrUpdatePolicyLocked):
					writeError(w, http.StatusLocked, "update_policy_locked", err.Error())
				case errors.Is(err, ErrPortainerIntegrationRequired):
					writeError(w, http.StatusPreconditionFailed, "portainer_required", err.Error())
				case errors.Is(err, ErrComposeTargetDivergesFromSource):
					writeError(w, http.StatusPreconditionFailed, "compose_source_change_required", err.Error())
				case errors.Is(err, ErrComposeSourceDriftDetected):
					writeError(w, http.StatusPreconditionFailed, "compose_source_drift", err.Error())
				case errors.Is(err, ErrComposeSourceVerificationUnavailable):
					writeError(w, http.StatusPreconditionFailed, "compose_source_required", err.Error())
				default:
					writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				}
				return
			}

			resp, err := deps.updateService.StartUpdate(built.Request)
			if err != nil {
				writeError(w, http.StatusBadRequest, "update_start_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusAccepted, resp)
		})

		r.Get("/jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deps.updateService == nil {
				writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			run, err := deps.updateService.GetJob(ctx, chi.URLParam(r, "id"))
			if err != nil {
				writeError(w, http.StatusBadGateway, "update_read_failed", err.Error())
				return
			}
			if run == nil {
				writeError(w, http.StatusNotFound, "update_not_found", "Update job not found")
				return
			}
			writeJSON(w, http.StatusOK, run)
		})

		r.Get("/container/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deps.updateService == nil {
				writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
				return
			}
			id := strings.TrimSpace(chi.URLParam(r, "id"))
			if id == "" {
				writeError(w, http.StatusBadRequest, "invalid_request", "container id is required")
				return
			}
			limit := parseIntQuery(r.URL.Query().Get("limit"), 20, 1, 100)
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			runs, err := deps.updateService.ListContainerJobs(ctx, id, limit)
			if err != nil {
				writeError(w, http.StatusBadGateway, "update_read_failed", err.Error())
				return
			}
			if len(runs) == 0 && deps.dockerClient != nil {
				inspectCtx, inspectCancel := context.WithTimeout(r.Context(), 2*time.Second)
				if summary, inspectErr := deps.dockerClient.GetContainer(inspectCtx, id); inspectErr == nil {
					if name := strings.TrimSpace(trimContainerName(summary.Names)); name != "" {
						if byName, listErr := deps.updateService.ListContainerJobs(ctx, name, limit); listErr == nil && len(byName) > 0 {
							runs = byName
						}
					}
				}
				inspectCancel()
			}
			writeJSON(w, http.StatusOK, runs)
		})

		r.Get("/ai-blocked", func(w http.ResponseWriter, r *http.Request) {
			result := map[string]updateAIBlockedSignal{}
			if deps.updateService == nil || deps.dockerClient == nil {
				writeJSON(w, http.StatusOK, result)
				return
			}

			listCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			containers, err := deps.dockerClient.ListContainers(listCtx)
			if err != nil {
				writeError(w, http.StatusBadGateway, "container_list_failed", err.Error())
				return
			}

			for _, c := range containers {
				id := strings.TrimSpace(c.ID)
				if id == "" {
					continue
				}
				historyCtx, historyCancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
				runs, err := deps.updateService.ListContainerJobs(historyCtx, id, 10)
				historyCancel()
				if err != nil {
					continue
				}
				for _, run := range runs {
					if signal, ok := extractAIBlockedSignal(run); ok {
						result[id] = signal
						break
					}
				}
			}
			writeJSON(w, http.StatusOK, result)
		})

		r.Get("/events/{id}", func(w http.ResponseWriter, r *http.Request) {
			if deps.updateService == nil {
				writeError(w, http.StatusServiceUnavailable, "update_service_unavailable", "Update service unavailable")
				return
			}
			flusher, ok := w.(http.Flusher)
			if !ok {
				writeError(w, http.StatusInternalServerError, "stream_unsupported", "streaming unsupported by response writer")
				return
			}
			ch, cancel := deps.updateService.Subscribe(chi.URLParam(r, "id"))
			defer cancel()
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			for {
				select {
				case <-r.Context().Done():
					return
				case e, ok := <-ch:
					if !ok {
						return
					}
					payload, _ := json.Marshal(e)
					fmt.Fprintf(w, "event: update\ndata: %s\n\n", payload)
					flusher.Flush()
				}
			}
		})
	})
}
