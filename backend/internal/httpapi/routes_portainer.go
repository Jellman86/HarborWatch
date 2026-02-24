package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/jobs"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/go-chi/chi/v5"
)

func registerPortainerRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/portainer/stacks", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if deps.currentPortainerState == nil || *deps.currentPortainerState == nil {
				writeError(w, http.StatusServiceUnavailable, "portainer_unavailable", "Portainer integration not configured")
				return
			}
			stacks, err := (*deps.currentPortainerState).ListStacks(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "portainer_error", err.Error())
				return
			}
			if stacks == nil {
				stacks = []portainer.Stack{}
			}
			writeJSON(w, http.StatusOK, stacks)
		})

		r.Post("/{id}/redeploy", func(w http.ResponseWriter, r *http.Request) {
			if deps.currentPortainerState == nil || *deps.currentPortainerState == nil {
				writeError(w, http.StatusServiceUnavailable, "portainer_unavailable", "Portainer integration not configured")
				return
			}

			idStr := chi.URLParam(r, "id")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_id", "Stack ID must be an integer")
				return
			}

			portainerClient := *deps.currentPortainerState

			stacks, err := portainerClient.ListStacks(r.Context())
			if err != nil {
				writeError(w, http.StatusBadGateway, "portainer_error", "Failed to list stacks: "+err.Error())
				return
			}
			var targetStack *portainer.Stack
			for _, s := range stacks {
				if s.ID == id {
					targetStack = &s
					break
				}
			}
			if targetStack == nil {
				writeError(w, http.StatusNotFound, "stack_not_found", "Stack not found in Portainer")
				return
			}

			jobID := "redeploy-" + idStr + "-" + strconv.FormatInt(time.Now().Unix(), 10)
			lockID := "portainer-stack-" + idStr

			job := &jobs.Job{
				ID:         jobID,
				Type:       jobs.JobTypeRedeploy,
				Target:     lockID,
				TargetName: "Stack: " + targetStack.Name,
				Status:     "queued",
				Message:    "Waiting for concurrency slot",
				StartedAt:  time.Now().UTC().Unix(),
			}
			if existingID, duplicate := deps.jobManager.RegisterJobIfNoDuplicate(job); duplicate {
				writeError(w, http.StatusConflict, "redeploy_already_running", fmt.Sprintf("A redeploy for this stack is already queued or running (job=%s)", existingID))
				return
			}

			go func(client PortainerClient, stack portainer.Stack) {
				defer deps.jobManager.FinishJob(jobID)

				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
				defer cancel()

				if err := deps.jobManager.AcquireSlot(ctx, jobID, lockID); err != nil {
					if deps.diagService != nil {
						deps.diagService.Log("ERROR", "Portainer", fmt.Sprintf("Stack %d redeploy failed to acquire slot: %v", id, err))
					}
					return
				}
				defer deps.jobManager.ReleaseSlot(jobID, lockID)

				deps.jobManager.UpdateJob(jobID, 10, "running", "Fetching stack file")
				yaml, err := client.GetStackFile(ctx, id)
				if err != nil {
					deps.jobManager.UpdateJob(jobID, 100, "failed", "Fetch failed: "+err.Error())
					return
				}

				deps.jobManager.UpdateJob(jobID, 30, "running", "Triggering Portainer update (with pull)")
				if err := client.UpdateStack(ctx, id, stack.EndpointID, yaml, stack.Env, true, true); err != nil {
					deps.jobManager.UpdateJob(jobID, 100, "failed", "Redeploy failed: "+err.Error())
					return
				}

				deps.jobManager.UpdateJob(jobID, 100, "completed", "Redeploy triggered successfully")
			}(portainerClient, *targetStack)

			writeJSON(w, http.StatusAccepted, map[string]string{
				"status":  "accepted",
				"message": "Stack redeploy queued",
				"jobId":   jobID,
			})
		})
	})
}
