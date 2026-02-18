package httpapi

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/portainer"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
)

type autoTaskDockerClient struct {
	containers []gen.ContainerSummary
	byID       map[string]gen.ContainerSummary
}

func (f autoTaskDockerClient) ListContainers(ctx context.Context) ([]gen.ContainerSummary, error) {
	return f.containers, nil
}

func (f autoTaskDockerClient) GetContainer(ctx context.Context, id string) (gen.ContainerSummary, error) {
	if c, ok := f.byID[id]; ok {
		return c, nil
	}
	return gen.ContainerSummary{}, errors.New("not found")
}

func (f autoTaskDockerClient) GetContainerLogs(ctx context.Context, id string, tail int, since time.Time, timestamps bool) (dockerengine.ContainerLogs, error) {
	return dockerengine.ContainerLogs{}, nil
}

func (f autoTaskDockerClient) GetContainerComposeConfig(ctx context.Context, id string, p *portainer.Client) (string, error) {
	return "", nil
}

func (f autoTaskDockerClient) ListImages(ctx context.Context) ([]gen.ImageSummary, error) {
	return []gen.ImageSummary{}, nil
}

func (f autoTaskDockerClient) OpenEventStream(ctx context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

type recordingUpdateService struct {
	started     []updates.Request
	listByID    map[string][]gen.UpdateJobStatus
	startErr    error
	startStatus string
}

func (s *recordingUpdateService) StartUpdate(req updates.Request) (gen.UpdateStartResponse, error) {
	if s.startErr != nil {
		return gen.UpdateStartResponse{}, s.startErr
	}
	s.started = append(s.started, req)
	status := s.startStatus
	if status == "" {
		status = "running"
	}
	return gen.UpdateStartResponse{
		JobID:  "job-" + req.ContainerID,
		Status: status,
	}, nil
}

func (s *recordingUpdateService) GetJob(ctx context.Context, jobID string) (*gen.UpdateJobStatus, error) {
	return nil, nil
}

func (s *recordingUpdateService) ListContainerJobs(ctx context.Context, containerID string, limit int) ([]gen.UpdateJobStatus, error) {
	if s.listByID == nil {
		return []gen.UpdateJobStatus{}, nil
	}
	return s.listByID[containerID], nil
}

func (s *recordingUpdateService) Subscribe(jobID string) (<-chan gen.UpdateStepEvent, func()) {
	ch := make(chan gen.UpdateStepEvent)
	close(ch)
	return ch, func() {}
}

type testRulesService struct {
	rule rules.ContainerRules
}

func (s testRulesService) Get(ctx context.Context, id string) (rules.ContainerRules, error) {
	out := s.rule
	if out.ContainerID == "" {
		out.ContainerID = id
	}
	return out, nil
}

func (s testRulesService) Save(ctx context.Context, r rules.ContainerRules) error {
	return nil
}

func TestAutomatedUpdateApplyTask_StartsAutoContainersWithUpdates(t *testing.T) {
	container := gen.ContainerSummary{
		ID:              "c-auto",
		Names:           []string{"/auto"},
		Image:           "ghcr.io/example/auto:v1",
		UpdateAvailable: true,
	}
	dockerClient := autoTaskDockerClient{
		containers: []gen.ContainerSummary{container},
		byID:       map[string]gen.ContainerSummary{container.ID: container},
	}
	updateSvc := &recordingUpdateService{}
	task := newAutomatedUpdateApplyTask(
		dockerClient,
		updateSvc,
		testRulesService{rule: rules.ContainerRules{
			UpdatePolicy: "auto",
			ValidateURL:  "http://localhost:8080/health",
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("run task: %v", err)
	}
	if len(updateSvc.started) != 1 {
		t.Fatalf("expected one auto-started update, got %d", len(updateSvc.started))
	}
	if updateSvc.started[0].ContainerID != container.ID {
		t.Fatalf("expected container %s, got %s", container.ID, updateSvc.started[0].ContainerID)
	}
}

func TestAutomatedUpdateApplyTask_SkipsWhenJobAlreadyRunning(t *testing.T) {
	container := gen.ContainerSummary{
		ID:              "c-running",
		Names:           []string{"/running"},
		Image:           "ghcr.io/example/running:v1",
		UpdateAvailable: true,
	}
	dockerClient := autoTaskDockerClient{
		containers: []gen.ContainerSummary{container},
		byID:       map[string]gen.ContainerSummary{container.ID: container},
	}
	updateSvc := &recordingUpdateService{
		listByID: map[string][]gen.UpdateJobStatus{
			container.ID: {{
				JobID:       "job-1",
				ContainerID: container.ID,
				TargetImage: container.Image,
				ValidateURL: "http://localhost:8080/health",
				Status:      "running",
				CreatedAt:   time.Now().UTC().Unix(),
				UpdatedAt:   time.Now().UTC().Unix(),
				Steps:       []gen.UpdateStepEvent{},
			}},
		},
	}
	task := newAutomatedUpdateApplyTask(
		dockerClient,
		updateSvc,
		testRulesService{rule: rules.ContainerRules{
			UpdatePolicy: "auto",
			ValidateURL:  "http://localhost:8080/health",
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("run task: %v", err)
	}
	if len(updateSvc.started) != 0 {
		t.Fatalf("expected running container to be skipped, got %d starts", len(updateSvc.started))
	}
}
