package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"gopkg.in/yaml.v3"
)

var (
	ErrComposeSourceVerificationUnavailable = errors.New("compose source verification unavailable")
	ErrComposeSourceDriftDetected           = errors.New("runtime image diverges from compose source")
	ErrComposeTargetDivergesFromSource      = errors.New("target image diverges from compose source")
)

func enforceComposeSourceAuthorityForAuto(
	ctx context.Context,
	summary gen.ContainerSummary,
	targetImage string,
	portainerService PortainerClient,
) error {
	if !isComposeManagedContainer(summary) {
		return nil
	}

	declared, err := resolveDeclaredComposeImageRef(ctx, summary, portainerService)
	if err != nil {
		return err
	}
	if !imageRefsEquivalent(summary.Image, declared) {
		return fmt.Errorf("%w: runtime=%q declared=%q", ErrComposeSourceDriftDetected, strings.TrimSpace(summary.Image), declared)
	}
	if !imageRefsEquivalent(targetImage, declared) {
		return fmt.Errorf("%w: target=%q declared=%q", ErrComposeTargetDivergesFromSource, strings.TrimSpace(targetImage), declared)
	}
	return nil
}

func isComposeManagedContainer(summary gen.ContainerSummary) bool {
	if summary.Labels == nil {
		return false
	}
	project := strings.TrimSpace(summary.Labels["com.docker.compose.project"])
	service := strings.TrimSpace(summary.Labels["com.docker.compose.service"])
	return project != "" && service != ""
}

func resolveDeclaredComposeImageRef(ctx context.Context, summary gen.ContainerSummary, portainerService PortainerClient) (string, error) {
	if !isComposeManagedContainer(summary) {
		return "", nil
	}
	service := strings.TrimSpace(summary.Labels["com.docker.compose.service"])
	if service == "" {
		return "", fmt.Errorf("%w: compose service label missing", ErrComposeSourceVerificationUnavailable)
	}

	stackID, ok := portainerStackIDFromLabels(summary.Labels)
	if !ok {
		return "", fmt.Errorf("%w: compose-managed container is not backed by a verifiable Portainer stack source", ErrComposeSourceVerificationUnavailable)
	}
	if portainerService == nil {
		return "", fmt.Errorf("%w: portainer integration unavailable", ErrComposeSourceVerificationUnavailable)
	}

	yamlText, err := portainerService.GetStackFile(ctx, stackID)
	if err != nil {
		return "", fmt.Errorf("%w: fetch stack file: %v", ErrComposeSourceVerificationUnavailable, err)
	}
	imageRef, err := extractComposeServiceImageRef(yamlText, service)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrComposeSourceVerificationUnavailable, err)
	}
	return imageRef, nil
}

func portainerStackIDFromLabels(labels map[string]string) (int, bool) {
	if labels == nil {
		return 0, false
	}
	if raw := strings.TrimSpace(labels["io.portainer.stack_id"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n, true
		}
	}
	path := strings.TrimSpace(firstNonEmpty(
		labels["com.docker.compose.project.config_files"],
		labels["com.docker.compose.project.working_dir"],
	))
	if strings.HasPrefix(path, "/data/compose/") {
		parts := strings.Split(strings.TrimPrefix(path, "/data/compose/"), "/")
		if len(parts) > 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && n > 0 {
				return n, true
			}
		}
	}
	return 0, false
}

func extractComposeServiceImageRef(stackYAML, serviceName string) (string, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "", errors.New("missing compose service name")
	}
	var root map[string]any
	if err := yaml.Unmarshal([]byte(stackYAML), &root); err != nil {
		return "", fmt.Errorf("parse stack yaml: %w", err)
	}
	services, ok := root["services"].(map[string]any)
	if !ok || len(services) == 0 {
		return "", errors.New("services map not found")
	}
	serviceDef, ok := services[serviceName]
	if !ok {
		return "", fmt.Errorf("service %q not found in compose source", serviceName)
	}
	serviceMap, ok := serviceDef.(map[string]any)
	if !ok {
		return "", fmt.Errorf("service %q definition is invalid", serviceName)
	}
	imageRef := strings.TrimSpace(fmt.Sprint(serviceMap["image"]))
	if imageRef == "" || imageRef == "<nil>" {
		return "", fmt.Errorf("service %q has no image field", serviceName)
	}
	return imageRef, nil
}

func imageRefsEquivalent(a, b string) bool {
	left, okL := normalizeSourceAuthorityRef(a)
	right, okR := normalizeSourceAuthorityRef(b)
	if !okL || !okR {
		return false
	}
	if left.repo != right.repo {
		return false
	}
	if left.digest != "" || right.digest != "" {
		return left.digest != "" && left.digest == right.digest
	}
	return left.tag == right.tag
}

type normalizedSourceRef struct {
	repo   string
	tag    string
	digest string
}

func normalizeSourceAuthorityRef(ref string) (normalizedSourceRef, bool) {
	raw := strings.TrimSpace(ref)
	if raw == "" {
		return normalizedSourceRef{}, false
	}
	digest := ""
	if at := strings.Index(raw, "@"); at > 0 {
		digest = strings.ToLower(strings.TrimSpace(raw[at+1:]))
		raw = strings.TrimSpace(raw[:at])
	}
	lastSlash := strings.LastIndex(raw, "/")
	lastColon := strings.LastIndex(raw, ":")
	repo := raw
	tag := ""
	if lastColon > lastSlash {
		repo = strings.TrimSpace(raw[:lastColon])
		tag = strings.TrimSpace(raw[lastColon+1:])
	}
	repo = normalizedImageRefKey(repo)
	if repo == "" {
		return normalizedSourceRef{}, false
	}
	if tag == "" && digest == "" {
		tag = "latest"
	}
	return normalizedSourceRef{
		repo:   repo,
		tag:    strings.ToLower(tag),
		digest: digest,
	}, true
}
