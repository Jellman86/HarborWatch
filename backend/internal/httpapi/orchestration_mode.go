package httpapi

import (
	"os"
	"sort"
	"strings"

	"github.com/Jellman86/HarborWatch/backend/internal/composesnapshots"
	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

type orchestrationMode string

const (
	orchestrationModePlainDocker    orchestrationMode = "plain_docker"
	orchestrationModeDockerCompose  orchestrationMode = "docker_compose"
	orchestrationModePortainerStack orchestrationMode = "portainer_stack"
)

type composeSourceStatus string

const (
	composeSourceStatusUnverified       composeSourceStatus = "unverified"
	composeSourceStatusVerifiedReadonly composeSourceStatus = "verified_readonly"
	composeSourceStatusVerifiedWritable composeSourceStatus = "verified_writable"
)

type localComposeProjectMember struct {
	ContainerID     string `json:"containerId"`
	ContainerName   string `json:"containerName,omitempty"`
	ServiceName     string `json:"serviceName,omitempty"`
	Image           string `json:"image,omitempty"`
	State           string `json:"state,omitempty"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

type localComposeProject struct {
	ProjectName      string                      `json:"projectName"`
	WorkingDir       string                      `json:"workingDir,omitempty"`
	ConfigFiles      []string                    `json:"configFiles,omitempty"`
	SourceStatus     composeSourceStatus         `json:"sourceStatus"`
	SourceVerified   bool                        `json:"sourceVerified"`
	SourceWritable   bool                        `json:"sourceWritable"`
	ContainerCount   int                         `json:"containerCount"`
	UpdateCandidates int                         `json:"updateCandidates"`
	SnapshotRootPath string                      `json:"snapshotRootPath,omitempty"`
	SnapshotStatus   string                      `json:"snapshotStatus,omitempty"`
	SnapshotCount    int                         `json:"snapshotCount,omitempty"`
	ChangedFiles     int                         `json:"changedFiles,omitempty"`
	LastSnapshotAt   int64                       `json:"lastSnapshotAt,omitempty"`
	LastSnapshotPath string                      `json:"lastSnapshotPath,omitempty"`
	SnapshotError    string                      `json:"snapshotError,omitempty"`
	Members          []localComposeProjectMember `json:"members"`
}

func detectContainerOrchestrationMode(summary gen.ContainerSummary) orchestrationMode {
	if isPortainerManagedLabels(summary.Labels) {
		return orchestrationModePortainerStack
	}
	if hasComposeLabels(summary.Labels) {
		return orchestrationModeDockerCompose
	}
	return orchestrationModePlainDocker
}

func hasComposeLabels(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	return strings.TrimSpace(labels["com.docker.compose.project"]) != "" &&
		strings.TrimSpace(labels["com.docker.compose.service"]) != ""
}

func isPortainerManagedLabels(labels map[string]string) bool {
	if labels == nil {
		return false
	}
	if strings.TrimSpace(labels["io.portainer.stack_id"]) != "" {
		return true
	}
	for _, path := range composeLabelPaths(labels) {
		if strings.HasPrefix(path, "/data/compose/") {
			return true
		}
	}
	return false
}

func composeLabelPaths(labels map[string]string) []string {
	if labels == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 2)
	for _, raw := range []string{
		labels["com.docker.compose.project.config_files"],
		labels["com.docker.compose.project.working_dir"],
	} {
		for _, p := range splitComposePathList(raw) {
			if p == "" {
				continue
			}
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out
}

func splitComposePathList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func localComposeProjectMetadata(summary gen.ContainerSummary) (projectName, serviceName, workingDir string, configFiles []string, ok bool) {
	if detectContainerOrchestrationMode(summary) != orchestrationModeDockerCompose {
		return "", "", "", nil, false
	}
	projectName = strings.TrimSpace(summary.Labels["com.docker.compose.project"])
	serviceName = strings.TrimSpace(summary.Labels["com.docker.compose.service"])
	workingDir = strings.TrimSpace(summary.Labels["com.docker.compose.project.working_dir"])
	configFiles = splitComposePathList(summary.Labels["com.docker.compose.project.config_files"])
	if projectName == "" || serviceName == "" {
		return "", "", "", nil, false
	}
	return projectName, serviceName, workingDir, configFiles, true
}

func detectComposeSourceStatus(configFiles []string) composeSourceStatus {
	if len(configFiles) == 0 {
		return composeSourceStatusUnverified
	}
	verified := 0
	writable := 0
	for _, p := range configFiles {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		verified++
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err == nil {
			writable++
			_ = f.Close()
		}
	}
	if verified == 0 {
		return composeSourceStatusUnverified
	}
	if writable == verified {
		return composeSourceStatusVerifiedWritable
	}
	return composeSourceStatusVerifiedReadonly
}

func discoverLocalComposeProjects(containers []gen.ContainerSummary) []localComposeProject {
	return discoverLocalComposeProjectsWithSnapshotRoot(containers, "")
}

func discoverLocalComposeProjectsWithSnapshotRoot(containers []gen.ContainerSummary, snapshotRoot string) []localComposeProject {
	type key struct {
		project    string
		workingDir string
		configs    string
	}
	grouped := map[key]*localComposeProject{}

	for _, c := range containers {
		projectName, serviceName, workingDir, configFiles, ok := localComposeProjectMetadata(c)
		if !ok {
			continue
		}
		cfgKey := strings.Join(configFiles, ",")
		k := key{project: projectName, workingDir: workingDir, configs: cfgKey}
		p := grouped[k]
		if p == nil {
			status := detectComposeSourceStatus(configFiles)
			snapshotStatus, snapshotErr := composesnapshots.DetectProjectChangeStatus(snapshotRoot, projectName, workingDir, configFiles)
			p = &localComposeProject{
				ProjectName:      projectName,
				WorkingDir:       workingDir,
				ConfigFiles:      append([]string(nil), configFiles...),
				SourceStatus:     status,
				SourceVerified:   status != composeSourceStatusUnverified,
				SourceWritable:   status == composeSourceStatusVerifiedWritable,
				SnapshotRootPath: strings.TrimSpace(composesnapshots.ResolveRoot(snapshotRoot)),
				Members:          []localComposeProjectMember{},
			}
			if snapshotErr != nil {
				p.SnapshotError = snapshotErr.Error()
				p.SnapshotStatus = string(composesnapshots.SnapshotChangeStatusNone)
			} else {
				p.SnapshotStatus = string(snapshotStatus.Status)
				p.SnapshotCount = snapshotStatus.SnapshotCount
				p.ChangedFiles = snapshotStatus.ChangedFileCount
				if snapshotStatus.Latest != nil {
					p.LastSnapshotAt = snapshotStatus.Latest.CreatedAt
					p.LastSnapshotPath = snapshotStatus.Latest.ArchivePath
				}
			}
			grouped[k] = p
		}
		member := localComposeProjectMember{
			ContainerID:     strings.TrimSpace(c.ID),
			ContainerName:   trimContainerName(c.Names),
			ServiceName:     serviceName,
			Image:           strings.TrimSpace(c.Image),
			State:           strings.TrimSpace(c.State),
			UpdateAvailable: c.UpdateAvailable,
		}
		p.Members = append(p.Members, member)
		p.ContainerCount++
		if c.UpdateAvailable {
			p.UpdateCandidates++
		}
	}

	out := make([]localComposeProject, 0, len(grouped))
	for _, p := range grouped {
		sort.Slice(p.Members, func(i, j int) bool {
			left := strings.ToLower(strings.TrimSpace(p.Members[i].ServiceName))
			right := strings.ToLower(strings.TrimSpace(p.Members[j].ServiceName))
			if left == right {
				return p.Members[i].ContainerID < p.Members[j].ContainerID
			}
			return left < right
		})
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(out[i].ProjectName))
		right := strings.ToLower(strings.TrimSpace(out[j].ProjectName))
		if left == right {
			return out[i].WorkingDir < out[j].WorkingDir
		}
		return left < right
	})
	return out
}
