package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
)

// ScannerService is an interface to avoid circular dependencies with scanning package
type ScannerService interface {
	StartScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScanPath(targetLabel, scanPath string, cleanup bool) (gen.ScanStartResponse, error)
	UpdateClamAVSignatures(ctx context.Context) (string, error)
	ListImages(ctx context.Context) ([]gen.ImageSummary, error)
}

type ContainerAutomationPolicy func(ctx context.Context, containerID string) bool
type MalwareMountPolicy func(ctx context.Context, containerID, sourcePath string) bool

// DockerPruneTask cleans up dangling images and stopped containers.
type DockerPruneTask struct {
	docker *client.Client
	logger Logger
}

func NewDockerPruneTask(cli *client.Client) *DockerPruneTask {
	return &DockerPruneTask{docker: cli}
}

func (t *DockerPruneTask) WithLogger(logger Logger) *DockerPruneTask {
	t.logger = logger
	return t
}

func (t *DockerPruneTask) log(level, message string) {
	log.Printf("%s", message)
	if t.logger != nil {
		t.logger.Log(level, "Scheduler", message)
	}
}

func (t *DockerPruneTask) Name() string { return "docker_system_prune" }

func (t *DockerPruneTask) Run(ctx context.Context) error {
	t.log("INFO", "Starting Docker system prune task...")

	// Prune Images
	report, err := t.docker.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))
	if err != nil {
		t.log("ERROR", fmt.Sprintf("Image prune failed: %v", err))
		return fmt.Errorf("image prune failed: %w", err)
	}
	t.log("INFO", fmt.Sprintf("Pruned %d images, space reclaimed: %d bytes", len(report.ImagesDeleted), report.SpaceReclaimed))

	// Prune Containers
	cReport, err := t.docker.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		t.log("ERROR", fmt.Sprintf("Container prune failed: %v", err))
		return fmt.Errorf("container prune failed: %w", err)
	}
	t.log("INFO", fmt.Sprintf("Pruned %d containers, space reclaimed: %d bytes", len(cReport.ContainersDeleted), cReport.SpaceReclaimed))

	return nil
}

// TrivySweepTask scans all running containers for vulnerabilities.
type TrivySweepTask struct {
	docker  *client.Client
	scanner ScannerService
	allow   ContainerAutomationPolicy
	logger  Logger
}

func NewTrivySweepTask(cli *client.Client, s ScannerService, allow ...ContainerAutomationPolicy) *TrivySweepTask {
	task := &TrivySweepTask{docker: cli, scanner: s}
	if len(allow) > 0 {
		task.allow = allow[0]
	}
	return task
}

func (t *TrivySweepTask) Name() string { return "security_sweep_trivy" }

func (t *TrivySweepTask) WithLogger(logger Logger) *TrivySweepTask {
	t.logger = logger
	return t
}

func (t *TrivySweepTask) log(level, message string) {
	log.Printf("%s", message)
	if t.logger != nil {
		t.logger.Log(level, "Scheduler", message)
	}
}

func (t *TrivySweepTask) Run(ctx context.Context) error {
	t.log("INFO", "Starting Trivy security sweep task...")
	
	// Fetch all images to get unique primary references
	images, err := t.scanner.ListImages(ctx)
	if err != nil {
		return fmt.Errorf("list images for sweep: %w", err)
	}

	queued := 0
	for _, img := range images {
		// Use the first tag or ID as target
		target := img.ID
		if len(img.RepoTags) > 0 {
			target = img.RepoTags[0]
		}
		
		if target == "" || strings.HasPrefix(target, "<none>") {
			continue
		}

		t.log("INFO", fmt.Sprintf("Queueing Trivy scan for %s", target))
		if _, err := t.scanner.StartScan(target); err != nil {
			t.log("ERROR", fmt.Sprintf("Failed to queue Trivy scan for %s: %v", target, err))
			continue
		}
		queued++
	}
	t.log("INFO", fmt.Sprintf("Trivy security sweep queued %d unique image scans", queued))
	return nil
}

// ClamAVSweepTask scans all container host mounts for malware.
type ClamAVSweepTask struct {
	docker     *client.Client
	scanner    ScannerService
	allow      ContainerAutomationPolicy
	allowMount MalwareMountPolicy
	logger     Logger
}

func NewClamAVSweepTask(cli *client.Client, s ScannerService, allow ...ContainerAutomationPolicy) *ClamAVSweepTask {
	task := &ClamAVSweepTask{docker: cli, scanner: s}
	if len(allow) > 0 {
		task.allow = allow[0]
	}
	return task
}

func (t *ClamAVSweepTask) WithMountPolicy(policy MalwareMountPolicy) *ClamAVSweepTask {
	t.allowMount = policy
	return t
}

func (t *ClamAVSweepTask) WithLogger(logger Logger) *ClamAVSweepTask {
	t.logger = logger
	return t
}

func (t *ClamAVSweepTask) Name() string { return "malware_sweep_clamav" }

func (t *ClamAVSweepTask) log(level, message string) {
	log.Printf("%s", message)
	if t.logger != nil {
		t.logger.Log(level, "Scheduler", message)
	}
}

func (t *ClamAVSweepTask) Run(ctx context.Context) error {
	t.log("INFO", "Starting ClamAV malware sweep task...")
	containers, err := t.docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	queued := 0
	skippedPolicy := 0
	skippedMount := 0
	queueErrors := 0

	for _, c := range containers {
		if t.allow != nil && !t.allow(ctx, c.ID) {
			skippedPolicy++
			continue
		}
		inspect, err := t.docker.ContainerInspect(ctx, c.ID)
		if err != nil {
			t.log("WARN", fmt.Sprintf("Skipping ClamAV sweep for %s: inspect failed: %v", c.ID, err))
			continue
		}

		for _, m := range inspect.Mounts {
			source := strings.TrimSpace(m.Source)
			if source == "" {
				continue
			}
			if t.allowMount != nil && !t.allowMount(ctx, c.ID, source) {
				skippedMount++
				continue
			}
			dest := strings.TrimSpace(m.Destination)
			if dest == "" {
				dest = source
			}
			targetLabel := fmt.Sprintf("container:%s:mount:%s", strings.TrimSpace(c.ID), dest)
			t.log("INFO", fmt.Sprintf("Queueing ClamAV scan for %s (source=%s)", targetLabel, source))
			if _, err := t.scanner.StartMalwareScanPath(targetLabel, source, false); err != nil {
				queueErrors++
				t.log("ERROR", fmt.Sprintf("Failed to queue ClamAV scan for %s: %v", targetLabel, err))
				continue
			}
			queued++
		}
	}
	t.log("INFO", fmt.Sprintf("ClamAV malware sweep queued=%d skipped_policy=%d skipped_mount=%d queue_errors=%d", queued, skippedPolicy, skippedMount, queueErrors))
	return nil
}

// ClamAVSignatureUpdateTask refreshes local ClamAV signatures.
type ClamAVSignatureUpdateTask struct {
	scanner ScannerService
}

func NewClamAVSignatureUpdateTask(s ScannerService) *ClamAVSignatureUpdateTask {
	return &ClamAVSignatureUpdateTask{scanner: s}
}

func (t *ClamAVSignatureUpdateTask) Name() string { return "clamav_signature_update" }

func (t *ClamAVSignatureUpdateTask) Run(ctx context.Context) error {
	log.Printf("Automated Security Sweep: Triggering ClamAV signature update")
	summary, err := t.scanner.UpdateClamAVSignatures(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(summary) != "" {
		log.Printf("ClamAV signature update summary: %s", summary)
	}
	return nil
}

// GenericTask is a helper to wrap a simple function as a Task.
type GenericTask struct {
	name string
	run  func(ctx context.Context) error
}

func NewGenericTask(name string, run func(ctx context.Context) error) *GenericTask {
	return &GenericTask{name: name, run: run}
}

func (t *GenericTask) Name() string { return t.name }
func (t *GenericTask) Run(ctx context.Context) error {
	return t.run(ctx)
}
