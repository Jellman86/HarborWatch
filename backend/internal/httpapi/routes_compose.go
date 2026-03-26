package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/composeeditor"
	"github.com/Jellman86/HarborWatch/backend/internal/composesnapshots"
	"github.com/go-chi/chi/v5"
)

type composeProjectEditorResponse struct {
	ProjectKey         string                      `json:"projectKey"`
	ProjectName        string                      `json:"projectName"`
	WorkingDir         string                      `json:"workingDir,omitempty"`
	SourceStatus       composeSourceStatus         `json:"sourceStatus"`
	SourceVerified     bool                        `json:"sourceVerified"`
	SourceWritable     bool                        `json:"sourceWritable"`
	ComposeEditable    bool                        `json:"composeEditable"`
	EnvEditable        bool                        `json:"envEditable"`
	EnvCreatable       bool                        `json:"envCreatable"`
	EnvPath            string                      `json:"envPath,omitempty"`
	ComposeFiles       []composeeditor.FileState   `json:"composeFiles"`
	EnvFile            *composeeditor.FileState    `json:"envFile,omitempty"`
	ManagedEnvContent  string                      `json:"managedEnvContent,omitempty"`
	Members            []localComposeProjectMember `json:"members,omitempty"`
}

type composeProjectValidateRequest struct {
	ComposeFiles []composeeditor.DraftFile `json:"composeFiles"`
	EnvFile      *composeeditor.EnvDraft   `json:"envFile,omitempty"`
}

type composeProjectSaveRequest struct {
	ComposeFiles []composeeditor.DraftFile `json:"composeFiles"`
	EnvFile      *composeeditor.EnvDraft   `json:"envFile,omitempty"`
}

func registerComposeRoutes(r chi.Router, deps adminRouteDeps) {
	r.Route("/compose", func(r chi.Router) {
		r.Get("/projects", func(w http.ResponseWriter, r *http.Request) {
			projects, _, err := listLocalComposeProjects(r.Context(), deps)
			if err != nil {
				writeError(w, http.StatusBadGateway, "docker_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, projects)
		})

		r.Get("/projects/{projectKey}", func(w http.ResponseWriter, r *http.Request) {
			project, err := resolveComposeProject(r.Context(), deps, chi.URLParam(r, "projectKey"))
			if err != nil {
				writeError(w, http.StatusNotFound, "compose_project_not_found", "Compose project not found")
				return
			}
			if !project.SourceVerified {
				writeError(w, http.StatusConflict, "compose_source_unverified", "Compose source is not readable from this appliance")
				return
			}
			svc := composeeditor.NewService()
			composeFiles, envFile, err := svc.LoadProjectFiles(composeeditor.ProjectDescriptor{
				ProjectName:     project.ProjectName,
				WorkingDir:      project.WorkingDir,
				ConfigFiles:     project.ConfigFiles,
				ComposeEditable: project.ComposeEditable,
				EnvEditable:     project.EnvEditable,
				EnvCreatable:    project.EnvCreatable,
				EnvPath:         project.EnvPath,
			})
			if err != nil {
				writeError(w, http.StatusBadGateway, "compose_source_read_failed", err.Error())
				return
			}

			// Resolve the HarborWatch-managed env content for GitOps-backed projects.
			// This is the env that is actually used at deploy time and may not be present
			// as a regular .env file on disk.
			managedEnvContent := ""
			if deps.db != nil && deps.settingsService != nil && deps.gitOpsLookup != nil {
				if st, stErr := deps.settingsService.Get(r.Context()); stErr == nil {
					content, overrideFiles, _ := resolveGitOpsOverrideEnvFiles(
						r.Context(), project.WorkingDir, project.ConfigFiles,
						st.GitOpsMasterDirectory, deps.gitOpsLookup,
					)
					if content != "" {
						managedEnvContent = content
					} else if len(overrideFiles) > 0 {
						// envFilePath case: content is on disk at the resolved path
						if raw, readErr := os.ReadFile(overrideFiles[0]); readErr == nil {
							managedEnvContent = string(raw)
						}
					}
				}
			}

			writeJSON(w, http.StatusOK, composeProjectEditorResponse{
				ProjectKey:        project.ProjectKey,
				ProjectName:       project.ProjectName,
				WorkingDir:        project.WorkingDir,
				SourceStatus:      project.SourceStatus,
				SourceVerified:    project.SourceVerified,
				SourceWritable:    project.SourceWritable,
				ComposeEditable:   project.ComposeEditable,
				EnvEditable:       project.EnvEditable,
				EnvCreatable:      project.EnvCreatable,
				EnvPath:           project.EnvPath,
				ComposeFiles:      composeFiles,
				EnvFile:           envFile,
				ManagedEnvContent: managedEnvContent,
				Members:           project.Members,
			})
		})

		r.Post("/projects/{projectKey}/validate", func(w http.ResponseWriter, r *http.Request) {
			project, err := resolveComposeProject(r.Context(), deps, chi.URLParam(r, "projectKey"))
			if err != nil {
				writeError(w, http.StatusNotFound, "compose_project_not_found", "Compose project not found")
				return
			}
			var req composeProjectValidateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
				return
			}
			svc := composeeditor.NewService()
			validation := svc.ValidateDraft(r.Context(), composeeditor.ProjectDescriptor{
				ProjectName:     project.ProjectName,
				WorkingDir:      project.WorkingDir,
				ConfigFiles:     project.ConfigFiles,
				ComposeEditable: project.ComposeEditable,
				EnvEditable:     project.EnvEditable,
				EnvCreatable:    project.EnvCreatable,
				EnvPath:         project.EnvPath,
			}, req.ComposeFiles, req.EnvFile)
			if !validation.OK {
				writeJSON(w, http.StatusBadRequest, validation)
				return
			}
			writeJSON(w, http.StatusOK, validation)
		})

		r.Post("/projects/{projectKey}/prettify", func(w http.ResponseWriter, r *http.Request) {
			if _, err := resolveComposeProject(r.Context(), deps, chi.URLParam(r, "projectKey")); err != nil {
				writeError(w, http.StatusNotFound, "compose_project_not_found", "Compose project not found")
				return
			}
			var req composeProjectValidateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
				return
			}
			svc := composeeditor.NewService()
			formatted, err := svc.PrettifyComposeDrafts(req.ComposeFiles)
			if err != nil {
				writeError(w, http.StatusBadRequest, "compose_prettify_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"composeFiles": formatted,
			})
		})

		r.Put("/projects/{projectKey}", func(w http.ResponseWriter, r *http.Request) {
			project, err := resolveComposeProject(r.Context(), deps, chi.URLParam(r, "projectKey"))
			if err != nil {
				writeError(w, http.StatusNotFound, "compose_project_not_found", "Compose project not found")
				return
			}
			var req composeProjectSaveRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
				return
			}

			svc := composeeditor.NewService()
			validation := svc.ValidateDraft(r.Context(), composeeditor.ProjectDescriptor{
				ProjectName:     project.ProjectName,
				WorkingDir:      project.WorkingDir,
				ConfigFiles:     project.ConfigFiles,
				ComposeEditable: project.ComposeEditable,
				EnvEditable:     project.EnvEditable,
				EnvCreatable:    project.EnvCreatable,
				EnvPath:         project.EnvPath,
			}, req.ComposeFiles, req.EnvFile)
			if !validation.OK {
				writeJSON(w, http.StatusBadRequest, validation)
				return
			}

			if err := preflightComposeHashConflicts(project, req); err != nil {
				if errors.Is(err, composeeditor.ErrHashConflict) {
					writeError(w, http.StatusConflict, "compose_hash_conflict", err.Error())
					return
				}
				writeError(w, http.StatusBadGateway, "compose_preflight_failed", err.Error())
				return
			}

			snapshotRoot := ""
			if deps.settingsService != nil {
				if st, err := deps.settingsService.Get(r.Context()); err == nil {
					snapshotRoot = strings.TrimSpace(st.ComposeSnapshotRootPath)
				}
			}
			if composeDraftsHaveChanges(project, req.ComposeFiles) {
				if _, err := composesnapshots.CreateProjectSnapshot(composesnapshots.CreateSnapshotInput{
					RootDir:      snapshotRoot,
					ProjectName:  project.ProjectName,
					WorkingDir:   project.WorkingDir,
					ConfigFiles:  project.ConfigFiles,
					CreatedAtUTC: time.Now().UTC(),
				}); err != nil {
					writeError(w, http.StatusBadGateway, "compose_snapshot_failed", err.Error())
					return
				}
			}

			saveResult, err := svc.SaveDraft(composeeditor.ProjectDescriptor{
				ProjectName:     project.ProjectName,
				WorkingDir:      project.WorkingDir,
				ConfigFiles:     project.ConfigFiles,
				ComposeEditable: project.ComposeEditable,
				EnvEditable:     project.EnvEditable,
				EnvCreatable:    project.EnvCreatable,
				EnvPath:         project.EnvPath,
			}, req.ComposeFiles, req.EnvFile)
			if err != nil {
				var saveErr *composeeditor.SaveError
				if errors.As(err, &saveErr) {
					switch {
					case errors.Is(saveErr.Err, composeeditor.ErrHashConflict):
						writeError(w, http.StatusConflict, "compose_hash_conflict", saveErr.Error())
						return
					case errors.Is(saveErr.Err, composeeditor.ErrPathNotAllowed):
						writeError(w, http.StatusBadRequest, "compose_path_not_allowed", saveErr.Error())
						return
					case errors.Is(saveErr.Err, composeeditor.ErrFileNotWritable):
						writeError(w, http.StatusConflict, "compose_file_not_writable", saveErr.Error())
						return
					}
				}
				writeError(w, http.StatusBadGateway, "compose_save_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, saveResult)
		})
	})
}

func listLocalComposeProjects(ctx context.Context, deps adminRouteDeps) ([]localComposeProject, string, error) {
	if deps.dockerClient == nil {
		return nil, "", errors.New("docker client unavailable")
	}
	containers, err := deps.dockerClient.ListContainers(ctx)
	if err != nil {
		return nil, "", err
	}
	snapshotRoot := ""
	gitOpsRoot := ""
	if deps.settingsService != nil {
		if st, err := deps.settingsService.Get(ctx); err == nil {
			snapshotRoot = strings.TrimSpace(st.ComposeSnapshotRootPath)
			gitOpsRoot = strings.TrimSpace(st.GitOpsMasterDirectory)
		}
	}
	return discoverLocalComposeProjectsWithRoots(containers, snapshotRoot, gitOpsRoot), snapshotRoot, nil
}

func resolveComposeProject(ctx context.Context, deps adminRouteDeps, projectKey string) (*localComposeProject, error) {
	projects, _, err := listLocalComposeProjects(ctx, deps)
	if err != nil {
		return nil, err
	}
	key := strings.TrimSpace(projectKey)
	for i := range projects {
		if strings.EqualFold(strings.TrimSpace(projects[i].ProjectKey), key) {
			return &projects[i], nil
		}
	}
	return nil, errors.New("project not found")
}

func preflightComposeHashConflicts(project *localComposeProject, req composeProjectSaveRequest) error {
	for _, draft := range req.ComposeFiles {
		path := strings.TrimSpace(draft.Path)
		expected := strings.TrimSpace(draft.ExpectedSHA256)
		if expected == "" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read compose file %s: %w", path, err)
		}
		sum := sha256.Sum256(raw)
		actual := hex.EncodeToString(sum[:])
		if actual != expected {
			return fmt.Errorf("%w: compose file hash mismatch for %s", composeeditor.ErrHashConflict, path)
		}
	}

	if req.EnvFile != nil {
		expected := strings.TrimSpace(req.EnvFile.ExpectedSHA256)
		if expected != "" {
			path := strings.TrimSpace(req.EnvFile.Path)
			if path == "" {
				path = strings.TrimSpace(project.EnvPath)
			}
			if path == "" {
				path = filepath.Join(strings.TrimSpace(project.WorkingDir), ".env")
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read env file %s: %w", path, err)
			}
			sum := sha256.Sum256(raw)
			actual := hex.EncodeToString(sum[:])
			if actual != expected {
				return fmt.Errorf("%w: env file hash mismatch for %s", composeeditor.ErrHashConflict, path)
			}
		}
	}
	return nil
}

func composeDraftsHaveChanges(project *localComposeProject, drafts []composeeditor.DraftFile) bool {
	byPath := make(map[string]string, len(drafts))
	for _, draft := range drafts {
		path := strings.TrimSpace(draft.Path)
		if path == "" {
			continue
		}
		byPath[path] = draft.Content
	}
	for _, path := range project.ConfigFiles {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		content, ok := byPath[path]
		if !ok {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return true
		}
		if string(raw) != content {
			return true
		}
	}
	return false
}
