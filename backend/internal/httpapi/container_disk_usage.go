package httpapi

import (
	"context"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func collectContainerDiskUsage(ctx context.Context, containerID string) (*gen.ContainerDiskUsage, error) {
	raw, err := dockerengine.NewRawClient()
	if err != nil {
		return nil, err
	}
	defer raw.Close()

	inspectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	inspect, _, err := raw.ContainerInspectWithRaw(inspectCtx, containerID, true)
	if err != nil {
		return nil, err
	}

	writable := int64(0)
	rootFS := int64(0)
	if inspect.SizeRw != nil {
		writable = *inspect.SizeRw
	}
	if inspect.SizeRootFs != nil {
		rootFS = *inspect.SizeRootFs
	}

	mountCount := len(inspect.Mounts)
	return &gen.ContainerDiskUsage{
		WritableBytes: writable,
		RootFsBytes:   rootFS,
		MountCount:    mountCount,
	}, nil
}
