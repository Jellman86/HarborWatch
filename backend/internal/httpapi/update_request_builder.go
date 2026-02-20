package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/updates"
)

var (
	ErrUpdatePolicyLocked          = errors.New("update policy is locked")
	ErrUpdatePolicyNotAuto         = errors.New("update policy is not auto")
	ErrPortainerIntegrationRequired = errors.New("portainer integration required for this container")
)

type updateRequestBuildOptions struct {
	RequireAutoPolicy bool
	EnforceLocked     bool
}

type updateRequestBuildResult struct {
	Request updates.Request
	Rules   rules.ContainerRules
	Summary gen.ContainerSummary
}

func buildUpdateRequestForContainer(
	ctx context.Context,
	containerID string,
	targetImage string,
	validateURL string,
	validateMode string,
	validateTimeoutSec int,
	validateIntervalSec int,
	dockerClient DockerClient,
	portainerService PortainerClient,
	rulesService RulesService,
	settingsService SettingsService,
	intelService ContainerIntelService,
	releaseService ReleaseService,
	diagService DiagService,
	opts updateRequestBuildOptions,
) (updateRequestBuildResult, error) {
	out := updateRequestBuildResult{}
	containerID = strings.TrimSpace(containerID)
	targetImage = strings.TrimSpace(targetImage)
	validateURL = strings.TrimSpace(validateURL)
	if containerID == "" {
		return out, errors.New("containerId is required")
	}

	out.Summary = gen.ContainerSummary{ID: containerID}
	if dockerClient != nil {
		containerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if c, err := dockerClient.GetContainer(containerCtx, containerID); err == nil {
			out.Summary = c
		}
		cancel()
	}

	if targetImage == "" {
		targetImage = strings.TrimSpace(out.Summary.Image)
	}

	effectiveRules := rules.ContainerRules{
		ContainerID:           containerID,
		UpdatePolicy:          "manual",
		ValidateMode:          "both",
		ValidateTimeoutSec:    45,
		ValidateIntervalSec:   2,
		AIValidateLogs:        false,
		AutoRollback:          true,
		InheritAutomation:     true,
		UpgradesAutomation:    true,
		MaintenanceAutomation: true,
		SecurityAutomation:    true,
	}
	if rulesService != nil {
		rulesCtx, rulesCancel := context.WithTimeout(ctx, 3*time.Second)
		if loaded, err := rulesService.Get(rulesCtx, containerID); err == nil {
			effectiveRules = loaded
		}
		rulesCancel()
	}
	// Respect explicit request overrides so we do not perform unnecessary auto-derivation work.
	if validateURL != "" {
		effectiveRules.ValidateURL = validateURL
	}
	if validateMode != "" {
		effectiveRules.ValidateMode = validateMode
	}
	if validateTimeoutSec > 0 {
		effectiveRules.ValidateTimeoutSec = validateTimeoutSec
	}
	if validateIntervalSec > 0 {
		effectiveRules.ValidateIntervalSec = validateIntervalSec
	}
	effectiveRules = effectiveContainerRules(ctx, out.Summary, effectiveRules, settingsService, diagService)
	effectiveRules.UpdatePolicy = normalizeUpdatePolicy(effectiveRules.UpdatePolicy)
	if effectiveRules.UpdatePolicy == "" {
		effectiveRules.UpdatePolicy = "manual"
	}
	out.Rules = effectiveRules

	if opts.EnforceLocked && effectiveRules.UpdatePolicy == "locked" {
		return out, fmt.Errorf("%w for container %s", ErrUpdatePolicyLocked, containerID)
	}
	if opts.RequireAutoPolicy && effectiveRules.UpdatePolicy != "auto" {
		return out, fmt.Errorf("%w for container %s", ErrUpdatePolicyNotAuto, containerID)
	}

	if validateURL == "" {
		validateURL = strings.TrimSpace(effectiveRules.ValidateURL)
	}
	if targetImage == "" || validateURL == "" {
		return out, errors.New("targetImage and validateUrl could not be auto-derived; provide explicit values")
	}

	aiBlockRiskThreshold := -1
	if settingsService != nil {
		settingsCtx, settingsCancel := context.WithTimeout(ctx, 2*time.Second)
		if st, err := settingsService.Get(settingsCtx); err == nil {
			if st.AIBlockRiskThreshold >= 0 && st.AIBlockRiskThreshold <= 100 {
				aiBlockRiskThreshold = st.AIBlockRiskThreshold
			}
		}
		settingsCancel()
	}

	repoURL := deriveRepositoryURL(out.Summary)
	changelogURL := deriveChangelogURL(out.Summary, repoURL)
	if intelService != nil {
		intelCtx, intelCancel := context.WithTimeout(ctx, 3*time.Second)
		if ov, err := intelService.Get(intelCtx, containerID); err == nil {
			merged := effectiveContainerIntel(out.Summary, ov, portainerService)
			repoURL = firstNonEmpty(merged.EffectiveRepositoryURL, repoURL)
			changelogURL = firstNonEmpty(merged.EffectiveChangelogURL, changelogURL)
		}
		intelCancel()
	}

	releaseContext := ""
	if releaseService != nil {
		repoRef := firstNonEmpty(repoURL, changelogURL)
		currentTag := imageTagFromRef(out.Summary.Image)
		targetTag := imageTagFromRef(targetImage)
		releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		releaseContext = buildReleaseContext(releaseCtx, releaseService, repoRef, currentTag, targetTag)
		cancel()
	}

	isPortainer := false
	stackID := 0
	endpointID := 0
	if out.Summary.Labels != nil {
		if sid, ok := out.Summary.Labels["io.portainer.stack_id"]; ok {
			if portainerService == nil {
				return out, fmt.Errorf("%w: container %s is managed by a Portainer stack", ErrPortainerIntegrationRequired, containerID)
			}
			if parsed, err := strconv.Atoi(sid); err == nil {
				isPortainer = true
				stackID = parsed
				// Try to find the endpoint ID. It might be in labels or we query Portainer.
				if eid, ok := out.Summary.Labels["io.portainer.endpoint_id"]; ok {
					if parsedE, err := strconv.Atoi(eid); err == nil {
						endpointID = parsedE
					}
				}
				if endpointID == 0 {
					// Fallback: Query stacks list to find this stack's endpoint
					pCtx, pCancel := context.WithTimeout(ctx, 5*time.Second)
					if stacks, err := portainerService.ListStacks(pCtx); err == nil {
						for _, s := range stacks {
							if s.ID == stackID {
								endpointID = s.EndpointID
								break
							}
						}
					}
					pCancel()
				}
			}
		}
	}

	out.Request = updates.Request{
		ContainerID:          containerID,
		TargetImage:          targetImage,
		ValidateURL:          validateURL,
		CurrentImage:         strings.TrimSpace(out.Summary.Image),
		ContainerName:        trimContainerName(out.Summary.Names),
		Labels:               out.Summary.Labels,
		RepositoryURL:        repoURL,
		ChangelogURL:         changelogURL,
		ReleaseContext:       releaseContext,
		ValidateMode:         effectiveRules.ValidateMode,
		ValidateTimeoutSec:   effectiveRules.ValidateTimeoutSec,
		ValidateIntervalSec:  effectiveRules.ValidateIntervalSec,
		AIValidateLogs:       effectiveRules.AIValidateLogs,
		AIBlockRiskThreshold: aiBlockRiskThreshold,
		IsPortainerManaged:   isPortainer,
		PortainerStackID:     stackID,
		PortainerEndpointID:  endpointID,
	}
	return out, nil
}
