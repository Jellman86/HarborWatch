package httpapi

import (
	"context"
	"sort"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

func taglessImageRef(ref string) string {
	raw := strings.TrimSpace(ref)
	if raw == "" {
		return ""
	}
	if at := strings.Index(raw, "@"); at > 0 {
		raw = raw[:at]
	}
	lastSlash := strings.LastIndex(raw, "/")
	lastColon := strings.LastIndex(raw, ":")
	if lastColon > lastSlash {
		return raw[:lastColon]
	}
	return raw
}

func imageRefVariants(ref string) []string {
	raw := strings.TrimSpace(ref)
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := []string{}
	add := func(value string) {
		v := strings.TrimSpace(value)
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}

	add(raw)
	base := taglessImageRef(raw)
	add(base)
	if !strings.Contains(raw, "@") {
		lastSlash := strings.LastIndex(raw, "/")
		lastColon := strings.LastIndex(raw, ":")
		if lastColon <= lastSlash {
			add(raw + ":latest")
		}
	}
	if strings.Contains(raw, "@") {
		add(taglessImageRef(raw))
	}
	return out
}

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

type imageScanLookup interface {
	LatestSummaryForTarget(ctx context.Context, target string) (*gen.ScanSummary, error)
	MalwareSummaries(ctx context.Context, target string) ([]gen.MalwareScanSummary, error)
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

func buildImageIntelligence(ctx context.Context, dockerClient DockerClient, scanService imageScanLookup) ([]imageIntelligenceRow, error) {
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
		for _, key := range imageRefVariants(c.Image) {
			k := strings.ToLower(strings.TrimSpace(key))
			if k == "" {
				continue
			}
			inUseByImage[k] = true
			if c.UpdateAvailable {
				outdatedByImage[k] = true
			}
		}
	}

	out := make([]imageIntelligenceRow, 0, len(images))
	for _, img := range images {
		tags := filteredRepoTags(img.RepoTags)
		primaryRef := imagePrimaryRef(img)

		lookupCandidates := []string{}
		seenCandidate := map[string]struct{}{}
		addCandidate := func(ref string) {
			for _, v := range imageRefVariants(ref) {
				key := strings.ToLower(strings.TrimSpace(v))
				if key == "" {
					continue
				}
				if _, ok := seenCandidate[key]; ok {
					continue
				}
				seenCandidate[key] = struct{}{}
				lookupCandidates = append(lookupCandidates, v)
			}
		}
		for _, tag := range tags {
			addCandidate(tag)
		}
		addCandidate(primaryRef)

		inUse := false
		outdated := false
		for _, candidate := range lookupCandidates {
			key := strings.ToLower(strings.TrimSpace(candidate))
			if inUseByImage[key] {
				inUse = true
			}
			if outdatedByImage[key] {
				outdated = true
			}
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

		if scanService != nil {
			for _, candidate := range lookupCandidates {
				summary, err := scanService.LatestSummaryForTarget(ctx, candidate)
				if err != nil || summary == nil {
					continue
				}
				row.VulnerabilityTotal = summary.Total
				row.VulnerabilityCritical = summary.Critical
				row.VulnerabilityHigh = summary.High
				row.SecurityScannedAt = summary.ScannedAt
				break
			}
			for _, candidate := range lookupCandidates {
				malware, err := scanService.MalwareSummaries(ctx, candidate)
				if err != nil || len(malware) == 0 {
					continue
				}
				latest := malware[0]
				row.MalwareInfected = latest.Infected
				row.MalwareThreatCount = len(latest.ThreatsFound)
				if latest.ScannedAt > row.SecurityScannedAt {
					row.SecurityScannedAt = latest.ScannedAt
				}
				break
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
