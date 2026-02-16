package scheduler

import (
	"context"
	"fmt"
	"log"

	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
)

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

// SecurityScanTask (Placeholder for future implementation)
type SecurityScanTask struct {
	NameStr string
	Action  func(ctx context.Context) error
}

func (t *SecurityScanTask) Name() string { return t.NameStr }
func (t *SecurityScanTask) Run(ctx context.Context) error { return t.Action(ctx) }
