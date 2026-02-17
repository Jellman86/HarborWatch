package httpapi

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/dockerengine"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
	"github.com/Jellman86/HarborWatch/backend/internal/rules"
	"github.com/Jellman86/HarborWatch/backend/internal/settings"
)

var healthURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func effectiveContainerRules(ctx context.Context, summary gen.ContainerSummary, existing rules.ContainerRules, settingsService SettingsService, diagService DiagService) rules.ContainerRules {
	out := existing
	out.UpdatePolicy = normalizeUpdatePolicy(out.UpdatePolicy)
	if out.UpdatePolicy == "" {
		out.UpdatePolicy = "manual"
	}
	out.ValidateMode = normalizeValidateMode(out.ValidateMode)
	if out.ValidateMode == "" {
		out.ValidateMode = "both"
	}
	if out.ValidateTimeoutSec <= 0 {
		out.ValidateTimeoutSec = 45
	}
	if out.ValidateIntervalSec <= 0 {
		out.ValidateIntervalSec = 2
	}
	if strings.TrimSpace(out.ValidateURL) != "" {
		return out
	}
	out.ValidateURL = deriveValidationURL(ctx, summary, settingsService, diagService)
	return out
}

func normalizeUpdatePolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "auto":
		return "auto"
	case "manual":
		return "manual"
	case "locked":
		return "locked"
	default:
		return ""
	}
}

func normalizeValidateMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "http":
		return "http"
	case "docker":
		return "docker"
	case "both":
		return "both"
	default:
		return ""
	}
}

func deriveValidationURL(ctx context.Context, summary gen.ContainerSummary, settingsService SettingsService, diagService DiagService) string {
	if v := firstNonEmpty(
		summary.Labels["harborwatch.validate.url"],
		summary.Labels["io.harborwatch.validate_url"],
	); v != "" {
		return v
	}

	name := firstNonEmpty(trimContainerName(summary.Names), summary.ID)
	image := strings.TrimSpace(summary.Image)
	port := firstNonEmpty(
		summary.Labels["harborwatch.validate.port"],
		summary.Labels["io.harborwatch.validate_port"],
	)
	path := normalizeHealthPath(firstNonEmpty(
		summary.Labels["harborwatch.validate.path"],
		summary.Labels["io.harborwatch.validate_path"],
		"/health",
	))
	scheme := firstNonEmpty(
		strings.ToLower(strings.TrimSpace(summary.Labels["harborwatch.validate.scheme"])),
		"http",
	)
	host := firstNonEmpty(
		summary.Labels["harborwatch.validate.host"],
		"localhost",
	)

	if info, ok := inspectValidationHints(ctx, summary.ID, diagService); ok {
		if port == "" {
			port = info.port
		}
		if path == "/health" && info.path != "" {
			path = normalizeHealthPath(info.path)
		}
		if scheme == "http" && info.scheme != "" {
			scheme = info.scheme
		}
		if host == "localhost" && info.host != "" {
			host = info.host
		}
	}

	st := settings.Settings{}
	if settingsService != nil {
		ctxSettings, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if loaded, err := settingsService.Get(ctxSettings); err == nil {
			st = loaded
		}
	}

	instance := strings.TrimSpace(st.InstanceURL)
	pattern := strings.TrimSpace(st.ValidateURLPattern)
	replacements := map[string]string{
		"INSTANCE_URL":   instance,
		"HOST":           host,
		"PORT":           port,
		"SCHEME":         scheme,
		"PATH":           path,
		"CONTAINER_ID":   summary.ID,
		"CONTAINER_NAME": name,
		"IMAGE":          image,
	}

	if pattern != "" {
		if rendered := renderValidationPattern(pattern, replacements); rendered != "" {
			return rendered
		}
	}

	if instance != "" {
		if derived := joinInstanceAndPath(instance, path); derived != "" {
			return derived
		}
	}

	if port != "" {
		return fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

func joinInstanceAndPath(instanceURL, path string) string {
	u, err := url.Parse(strings.TrimSpace(instanceURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + normalizeHealthPath(path)
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func renderValidationPattern(pattern string, values map[string]string) string {
	out := strings.TrimSpace(pattern)
	if out == "" {
		return ""
	}
	for key, value := range values {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
		out = strings.ReplaceAll(out, "{{"+strings.ToLower(key)+"}}", value)
	}
	out = strings.TrimSpace(out)
	if strings.Contains(out, "{{") {
		return ""
	}
	return out
}

func normalizeHealthPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/health"
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func trimContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(names[0]), "/")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

type validationHints struct {
	host   string
	port   string
	path   string
	scheme string
}

func inspectValidationHints(ctx context.Context, containerID string, diagService DiagService) (validationHints, bool) {
	raw, err := dockerengine.NewRawClient()
	if err != nil || raw == nil {
		return validationHints{}, false
	}
	defer raw.Close()

	inspectCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	inspect, err := raw.ContainerInspect(inspectCtx, containerID)
	if err != nil {
		return validationHints{}, false
	}

	hints := validationHints{
		host:   "localhost",
		scheme: "http",
	}

	if inspect.NetworkSettings != nil {
		for _, bindings := range inspect.NetworkSettings.Ports {
			if len(bindings) == 0 {
				continue
			}
			b := bindings[0]
			if strings.TrimSpace(b.HostPort) == "" {
				continue
			}
			hints.port = strings.TrimSpace(b.HostPort)
			if hostIP := strings.TrimSpace(b.HostIP); hostIP != "" && hostIP != "0.0.0.0" && hostIP != "::" {
				hints.host = hostIP
			}
			break
		}
	}

	if hints.port == "" && inspect.Config != nil {
		for p := range inspect.Config.ExposedPorts {
			portProto := strings.TrimSpace(string(p))
			parts := strings.SplitN(portProto, "/", 2)
			if len(parts) > 0 && parts[0] != "" {
				hints.port = parts[0]
				break
			}
		}
	}

	if inspect.Config != nil && inspect.Config.Healthcheck != nil {
		for _, test := range inspect.Config.Healthcheck.Test {
			match := healthURLPattern.FindString(test)
			if match == "" {
				continue
			}
			u, err := url.Parse(match)
			if err != nil {
				continue
			}
			if p := strings.TrimSpace(u.Port()); p != "" {
				hints.port = p
			}
			if h := strings.TrimSpace(u.Hostname()); h != "" {
				hints.host = h
			}
			if scheme := strings.ToLower(strings.TrimSpace(u.Scheme)); scheme != "" {
				hints.scheme = scheme
			}
			if path := strings.TrimSpace(u.Path); path != "" {
				hints.path = path
			}
			break
		}
	}

	if hints.port == "" && diagService != nil {
		diagService.Log("WARN", "Lifecycle", fmt.Sprintf("Validation URL for %s derived without port; using fallback path only", containerID))
	}
	return hints, true
}
