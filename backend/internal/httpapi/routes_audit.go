package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func registerAuditRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/audit", func(r chi.Router) {
		r.Get("/jobs", func(w http.ResponseWriter, r *http.Request) {
			if deps.auditService == nil {
				writeError(w, http.StatusServiceUnavailable, "audit_service_unavailable", "Audit service not initialized")
				return
			}
			jobs, err := deps.auditService.ListAuditJobs(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "audit_query_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, jobs)
		})

		r.Get("/jobs/{id}/steps", func(w http.ResponseWriter, r *http.Request) {
			if deps.auditService == nil {
				writeError(w, http.StatusServiceUnavailable, "audit_service_unavailable", "Audit service not initialized")
				return
			}
			steps, err := deps.auditService.GetAuditJobSteps(r.Context(), chi.URLParam(r, "id"))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "audit_steps_query_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, steps)
		})
	})
}
