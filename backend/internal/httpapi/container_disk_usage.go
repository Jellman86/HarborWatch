package httpapi

import (
	"context"
	"syscall"
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
	hostTotal, hostAvailable, hostUsed := hostDiskCapacity()
	return &gen.ContainerDiskUsage{
		WritableBytes:      writable,
		RootFsBytes:        rootFS,
		MountCount:         mountCount,
		HostTotalBytes:     hostTotal,
		HostAvailableBytes: hostAvailable,
		HostUsedBytes:      hostUsed,
	}, nil
}

func hostDiskCapacity() (total int64, available int64, used int64) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err != nil {
		return 0, 0, 0
	}
	total = int64(fs.Blocks) * int64(fs.Bsize)
	available = int64(fs.Bavail) * int64(fs.Bsize)
	used = total - available
	if used < 0 {
		used = 0
	}
	return total, available, used
}
