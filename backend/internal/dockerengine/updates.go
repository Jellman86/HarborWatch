package dockerengine

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/moby/moby/client"
)

type UpdateStore struct {
	mu      sync.RWMutex
	updates map[string]bool
}

var globalUpdateStore = &UpdateStore{
	updates: make(map[string]bool),
}

func (s *UpdateStore) Set(imageName string, available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates[imageName] = available
}

func (s *UpdateStore) Get(imageName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updates[imageName]
}

// RefreshUpdateStatus checks for updates for all unique images in the provided container list.
func RefreshUpdateStatus(ctx context.Context, cli *client.Client, imageNames []string) {
	for _, img := range imageNames {
		available, err := CheckImageUpdate(ctx, cli, img)
		if err != nil {
			log.Printf("UpdateCheck: Failed for %s: %v", img, err)
			continue
		}
		globalUpdateStore.Set(img, available)
	}
}

// CheckImageUpdate performs a robust check for a new version of the given image.
// It uses 'docker pull --dry-run' style logic via DistributionInspect or comparable API calls.
func CheckImageUpdate(ctx context.Context, cli *client.Client, imageName string) (bool, error) {
	// 1. Get local image digest
	inspect, _, err := cli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return false, fmt.Errorf("inspect local: %w", err)
	}

	localDigest := ""
	if len(inspect.RepoDigests) > 0 {
		localDigest = inspect.RepoDigests[0]
	}

	// 2. Get remote image digest
	// Note: DistributionInspect requires registry credentials if private.
	// For public images, it works without auth in most Docker Engine configurations.
	dist, err := cli.DistributionInspect(ctx, imageName, "")
	if err != nil {
		// If registry doesn't support DistributionInspect or auth fails, we can't be sure.
		return false, nil 
	}

	remoteDigest := string(dist.Descriptor.Digest)
	
	// If we have a local digest, compare it.
	// RepoDigests usually looks like "repo/image@sha256:..."
	if localDigest != "" {
		for _, d := range inspect.RepoDigests {
			if fmt.Sprintf("%s@%s", imageName, remoteDigest) == d {
				return false, nil
			}
			// Alternative: just check if remoteDigest is contained in any local digest string
			if d != "" && (d == remoteDigest || (len(d) > 7 && d[len(d)-len(remoteDigest):] == remoteDigest)) {
				return false, nil
			}
		}
	}

	// If we got here and have digests to compare, and they differ, update is likely available.
	return remoteDigest != "" && !containsDigest(inspect.RepoDigests, remoteDigest), nil
}

func containsDigest(digests []string, target string) bool {
	for _, d := range digests {
		// RepoDigest is usually "image@sha256:digest"
		if d == target {
			return true
		}
		// Match the sha256: part
		if i := byteIndex(d, '@'); i != -1 {
			if d[i+1:] == target {
				return true
			}
		}
	}
	return false
}

func byteIndex(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// UpdateCheckTask is a scheduler task to refresh update information.
type UpdateCheckTask struct {
	docker *client.Client
}

func NewUpdateCheckTask(cli *client.Client) *UpdateCheckTask {
	return &UpdateCheckTask{docker: cli}
}

func (t *UpdateCheckTask) Name() string { return "container_update_check" }

func (t *UpdateCheckTask) Run(ctx context.Context) error {
	log.Println("Starting automated update check for containers...")
	
	// Get all images from running containers
	containers, err := t.docker.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	images := make(map[string]bool)
	for _, c := range containers {
		images[c.Image] = true
	}

	uniqueImages := make([]string, 0, len(images))
	for img := range images {
		uniqueImages = append(uniqueImages, img)
	}

	RefreshUpdateStatus(ctx, t.docker, uniqueImages)
	return nil
}
