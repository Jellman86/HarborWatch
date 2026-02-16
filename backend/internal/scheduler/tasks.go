package scheduler

import (
	"context"
	"fmt"
	"log"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
)

// ScannerService is an interface to avoid circular dependencies with scanning package
type ScannerService interface {
	StartScan(target string) (gen.ScanStartResponse, error)
	StartMalwareScan(target string) (gen.ScanStartResponse, error)
}

// DockerPruneTask cleans up dangling images and stopped containers.
type DockerPruneTask struct {
	docker *client.Client
}

func NewDockerPruneTask(cli *client.Client) *DockerPruneTask {
	return &DockerPruneTask{docker: cli}
}

func (t *DockerPruneTask) Name() string { return "docker_system_prune" }

func (t *DockerPruneTask) Run(ctx context.Context) error {
	log.Println("Starting automated Docker system prune...")
	
	// Prune Images
	report, err := t.docker.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))
	if err != nil {
		return fmt.Errorf("image prune failed: %w", err)
	}
	log.Printf("Pruned %d images, space reclaimed: %d bytes", len(report.ImagesDeleted), report.SpaceReclaimed)

	// Prune Containers
	cReport, err := t.docker.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		return fmt.Errorf("container prune failed: %w", err)
	}
	log.Printf("Pruned %d containers, space reclaimed: %d bytes", len(cReport.ContainersDeleted), cReport.SpaceReclaimed)

	return nil
}

// TrivySweepTask scans all running containers for vulnerabilities.
type TrivySweepTask struct {
	docker  *client.Client
	scanner ScannerService
}

func NewTrivySweepTask(cli *client.Client, s ScannerService) *TrivySweepTask {
	return &TrivySweepTask{docker: cli, scanner: s}
}

func (t *TrivySweepTask) Name() string { return "security_sweep_trivy" }

func (t *TrivySweepTask) Run(ctx context.Context) error {
	containers, err := t.docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	for _, c := range containers {
		log.Printf("Automated Security Sweep: Triggering Trivy scan for %s", c.Image)
		if _, err := t.scanner.StartScan(c.Image); err != nil {
			log.Printf("ERROR: Failed to start automated Trivy scan for %s: %v", c.Image, err)
		}
	}
	return nil
}

// ClamAVSweepTask scans all container host mounts for malware.
type ClamAVSweepTask struct {
	docker  *client.Client
	scanner ScannerService
}

func NewClamAVSweepTask(cli *client.Client, s ScannerService) *ClamAVSweepTask {
	return &ClamAVSweepTask{docker: cli, scanner: s}
}

func (t *ClamAVSweepTask) Name() string { return "malware_sweep_clamav" }

func (t *ClamAVSweepTask) Run(ctx context.Context) error {
	containers, err := t.docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	// Track paths to avoid duplicate scanning of the same host volume
	seenPaths := make(map[string]bool)

	for _, c := range containers {
		inspect, err := t.docker.ContainerInspect(ctx, c.ID)
		if err != nil {
			continue
		}

		for _, m := range inspect.Mounts {
			if m.Source != "" && !seenPaths[m.Source] {
				log.Printf("Automated Security Sweep: Triggering ClamAV scan for path %s", m.Source)
				if _, err := t.scanner.StartMalwareScan(m.Source); err != nil {
					log.Printf("ERROR: Failed to start automated ClamAV scan for %s: %v", m.Source, err)
				}
				seenPaths[m.Source] = true
			}
		}
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
