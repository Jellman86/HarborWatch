package httpapi

import (
	"context"
	"sort"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type imageIntelligenceRow struct {
	ID                    string   `json:"id"`
	RepoTags              []string `json:"repoTags"`
	Size                  int64    `json:"size"`
	PrimaryRef            string   `json:"primaryRef,omitempty"`
	InUse                 bool     `json:"inUse"`
	Outdated              bool     `json:"outdated"`
	PruneCandidate        bool     `json:"pruneCandidate"`
	VulnerabilityTotal    int      `json:"vulnerabilityTotal"`
	VulnerabilityCritical int      `json:"vulnerabilityCritical"`
	VulnerabilityHigh     int      `json:"vulnerabilityHigh"`
	MalwareInfected       bool     `json:"malwareInfected"`
	MalwareThreatCount    int      `json:"malwareThreatCount"`
	SecurityScannedAt     int64    `json:"securityScannedAt,omitempty"`
}

func filteredRepoTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" || tag == "<none>:<none>" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func imagePrimaryRef(img gen.ImageSummary) string {
	tags := filteredRepoTags(img.RepoTags)
	if len(tags) > 0 {
		return tags[0]
	}
	return strings.TrimSpace(img.ID)
}

func buildImageIntelligence(ctx context.Context, dockerClient DockerClient, scanService ScanService) ([]imageIntelligenceRow, error) {
	images, err := dockerClient.ListImages(ctx)
	if err != nil {
		return nil, err
	}

	containers, err := dockerClient.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	inUseByImage := map[string]bool{}
	outdatedByImage := map[string]bool{}
	for _, c := range containers {
		key := strings.TrimSpace(c.Image)
		if key == "" {
			continue
		}
		inUseByImage[key] = true
		if c.UpdateAvailable {
			outdatedByImage[key] = true
		}
	}

	out := make([]imageIntelligenceRow, 0, len(images))
	for _, img := range images {
		tags := filteredRepoTags(img.RepoTags)
		primaryRef := imagePrimaryRef(img)

		inUse := false
		outdated := false
		for _, tag := range tags {
			if inUseByImage[tag] {
				inUse = true
			}
			if outdatedByImage[tag] {
				outdated = true
			}
		}
		if !inUse && inUseByImage[primaryRef] {
			inUse = true
		}
		if !outdated && outdatedByImage[primaryRef] {
			outdated = true
		}

		row := imageIntelligenceRow{
			ID:             img.ID,
			RepoTags:       img.RepoTags,
			Size:           img.Size,
			PrimaryRef:     primaryRef,
			InUse:          inUse,
			Outdated:       outdated,
			PruneCandidate: !inUse,
		}

		if scanService != nil && primaryRef != "" {
			if summary, err := scanService.LatestSummaryForTarget(ctx, primaryRef); err == nil && summary != nil {
				row.VulnerabilityTotal = summary.Total
				row.VulnerabilityCritical = summary.Critical
				row.VulnerabilityHigh = summary.High
				row.SecurityScannedAt = summary.ScannedAt
			}
			if malware, err := scanService.MalwareSummaries(ctx, primaryRef); err == nil && len(malware) > 0 {
				latest := malware[0]
				row.MalwareInfected = latest.Infected
				row.MalwareThreatCount = len(latest.ThreatsFound)
				if latest.ScannedAt > row.SecurityScannedAt {
					row.SecurityScannedAt = latest.ScannedAt
				}
			}
		}

		out = append(out, row)
	}

	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(out[i].PrimaryRef))
		right := strings.ToLower(strings.TrimSpace(out[j].PrimaryRef))
		if left == right {
			return out[i].ID < out[j].ID
		}
		return left < right
	})

	return out, nil
}
