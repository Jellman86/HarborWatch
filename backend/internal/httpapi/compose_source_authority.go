package httpapi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/composecli"
	"github.com/Jellman86/HarborWatch/backend/internal/composeexec"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"gopkg.in/yaml.v3"
)

var (
	ErrComposeSourceVerificationUnavailable = errors.New("compose source verification unavailable")
	ErrComposeSourceDriftDetected           = errors.New("runtime image diverges from compose source")
	ErrComposeTargetDivergesFromSource      = errors.New("target image diverges from compose source")

	dockerComposeConfigRunner = func(ctx context.Context, workingDir string, configFiles, envFiles []string) ([]byte, error) {
		composeCtx := composeexec.New(workingDir, configFiles, envFiles)
		runner, err := composecli.Resolve(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve compose runtime: %w", err)
		}
		cmd := runner.CommandContext(ctx, composeCtx.RunnerArgs("config")...)
		if wd := strings.TrimSpace(workingDir); wd != "" {
			cmd.Dir = wd
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("compose config failed: %w (%s)", err, truncateAuthorityOutput(string(out), 400))
		}
		return out, nil
	}
)

func enforceComposeSourceAuthorityForAuto(
	ctx context.Context,
	summary gen.ContainerSummary,
	targetImage string,
	portainerService PortainerClient,
	composeEnvFiles []string,
) error {
	if !isComposeManagedContainer(summary) {
		return nil
	}

	declared, err := resolveDeclaredComposeImageRef(ctx, summary, portainerService, composeEnvFiles)
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

func resolveDeclaredComposeImageRef(ctx context.Context, summary gen.ContainerSummary, portainerService PortainerClient, composeEnvFiles []string) (string, error) {
	if !isComposeManagedContainer(summary) {
		return "", nil
	}
	if detectContainerOrchestrationMode(summary) == orchestrationModeDockerCompose {
		return resolveDeclaredLocalComposeImageRef(ctx, summary, composeEnvFiles)
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

func resolveDeclaredLocalComposeImageRef(ctx context.Context, summary gen.ContainerSummary, composeEnvFiles []string) (string, error) {
	project, service, workingDir, configFiles, ok := localComposeProjectMetadata(summary)
	if !ok {
		return "", fmt.Errorf("%w: compose labels missing", ErrComposeSourceVerificationUnavailable)
	}
	if len(configFiles) == 0 {
		return "", fmt.Errorf("%w: local compose project %q has no config files", ErrComposeSourceVerificationUnavailable, project)
	}
	status := detectComposeSourceStatus(configFiles)
	if status == composeSourceStatusUnverified {
		return "", fmt.Errorf("%w: local compose source is %s", ErrComposeSourceVerificationUnavailable, status)
	}
	imageRef, err := extractComposeServiceImageRefFromFiles(configFiles, service)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrComposeSourceVerificationUnavailable, err)
	}
	if composeImageRefNeedsResolution(imageRef) {
		resolved, err := resolveComposeServiceImageRefViaDockerConfig(ctx, workingDir, configFiles, composeEnvFiles, service)
		if err != nil {
			return "", fmt.Errorf("%w: resolve interpolated compose image for %q: %v", ErrComposeSourceVerificationUnavailable, service, err)
		}
		if strings.TrimSpace(resolved) != "" {
			return resolved, nil
		}
	}
	return imageRef, nil
}

func extractComposeServiceImageRefFromFiles(configFiles []string, service string) (string, error) {
	lastImage := ""
	foundService := false
	for _, path := range configFiles {
		raw, err := os.ReadFile(strings.TrimSpace(path))
		if err != nil {
			return "", fmt.Errorf("read compose file %s: %w", path, err)
		}
		imageRef, found, err := extractComposeServiceImageRefOptional(string(raw), service)
		if err != nil {
			return "", err
		}
		if found {
			foundService = true
		}
		if strings.TrimSpace(imageRef) != "" {
			lastImage = strings.TrimSpace(imageRef)
		}
	}
	if !foundService {
		return "", fmt.Errorf("service %q not found in compose source", service)
	}
	if lastImage == "" {
		return "", fmt.Errorf("service %q has no image field", service)
	}
	return lastImage, nil
}

func resolveComposeServiceImageRefViaDockerConfig(ctx context.Context, workingDir string, configFiles, envFiles []string, service string) (string, error) {
	if strings.TrimSpace(service) == "" {
		return "", errors.New("missing compose service name")
	}
	if len(configFiles) == 0 {
		return "", errors.New("compose config files required")
	}
	if strings.TrimSpace(workingDir) == "" {
		workingDir = filepath.Dir(strings.TrimSpace(configFiles[0]))
	}
	out, err := dockerComposeConfigRunner(ctx, workingDir, configFiles, envFiles)
	if err != nil {
		return "", err
	}
	imageRef, err := extractComposeServiceImageRef(string(out), service)
	if err != nil {
		return "", err
	}
	return imageRef, nil
}

func composeImageRefNeedsResolution(imageRef string) bool {
	ref := strings.TrimSpace(imageRef)
	return strings.Contains(ref, "${") || strings.Contains(ref, "$")
}

func truncateAuthorityOutput(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 4 {
		return s[:n]
	}
	return s[:n-3] + "..."
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
	imageRef, found, err := extractComposeServiceImageRefOptional(stackYAML, serviceName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("service %q not found in compose source", strings.TrimSpace(serviceName))
	}
	if strings.TrimSpace(imageRef) == "" {
		return "", fmt.Errorf("service %q has no image field", strings.TrimSpace(serviceName))
	}
	return imageRef, nil
}

func extractComposeServiceImageRefOptional(stackYAML, serviceName string) (string, bool, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "", false, errors.New("missing compose service name")
	}
	var root map[string]any
	if err := yaml.Unmarshal([]byte(stackYAML), &root); err != nil {
		return "", false, fmt.Errorf("parse stack yaml: %w", err)
	}
	services, ok := root["services"].(map[string]any)
	if !ok || len(services) == 0 {
		return "", false, nil
	}
	serviceDef, ok := services[serviceName]
	if !ok {
		return "", false, nil
	}
	serviceMap, ok := serviceDef.(map[string]any)
	if !ok {
		return "", false, fmt.Errorf("service %q definition is invalid", serviceName)
	}
	imageRef := strings.TrimSpace(fmt.Sprint(serviceMap["image"]))
	if imageRef == "<nil>" {
		imageRef = ""
	}
	return imageRef, true, nil
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
