# HarborWatch Self-Upgrade Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable HarborWatch to upgrade itself automatically or on-demand, supporting both Standalone Docker and Portainer-provisioned environments.

**Architecture:** A unified upgrade service that identifies the runtime environment and executes the appropriate upgrade path. For Portainer, it uses the Stack Update API. For Standalone Docker, it uses a temporary relay container ('harborwatch-upgrade-relay') to pull the new image, stop the old container, and start the new one with identical configuration.

**Tech Stack:** Go (Backend), Svelte 5 (Frontend), Docker SDK (Moby), Portainer API.

---

### Task 1: Create Upgrade Service Interface and Models

**Files:**
- Create: `backend/internal/updates/self_upgrade.go`
- Modify: `backend/internal/gen/api.go` (Add SelfUpgradeStatus and SelfUpgradeRequest)

**Step 1: Define the core types**

```go
package updates

import (
	"context"
	"time"
)

type SelfUpgradeStatus struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	LastCheckAt     int64  `json:"lastCheckAt"`
	Status          string `json:"status"` // idle, checking, pulling, upgrading, failed
	Error           string `json:"error,omitempty"`
}

type SelfUpgradeService interface {
	GetStatus(ctx context.Context) (SelfUpgradeStatus, error)
	CheckForUpdates(ctx context.Context) (SelfUpgradeStatus, error)
	StartUpgrade(ctx context.Context) error
}
```

**Step 2: Commit**

```bash
git add backend/internal/updates/self_upgrade.go
git commit -m "feat: add self-upgrade service interfaces"
```

### Task 2: Implement Version Checker

**Files:**
- Modify: `backend/internal/updates/self_upgrade.go`

**Step 1: Add version checking logic**

```go
func (s *SelfUpgradeEngine) CheckForUpdates(ctx context.Context) (SelfUpgradeStatus, error) {
    // 1. Fetch current version from environment or file
    // 2. Fetch latest version from GHCR or GitHub API
    // 3. Compare and update internal state
}
```

**Step 2: Commit**

```bash
git commit -m "feat: implement version checking logic"
```

### Task 3: Implement Portainer Upgrade Path

**Files:**
- Modify: `backend/internal/updates/self_upgrade.go`

**Step 1: Use Portainer API to redeploy the stack**

```go
func (s *SelfUpgradeEngine) upgradePortainer(ctx context.Context) error {
    // 1. Find the stack containing HarborWatch
    // 2. Call s.portainer.UpdateStack(stackID, endpointID, yaml, env, true, true)
}
```

**Step 2: Commit**

```bash
git commit -m "feat: implement portainer self-upgrade path"
```

### Task 4: Implement Standalone Docker Upgrade Path (Relay Container)

**Files:**
- Modify: `backend/internal/updates/self_upgrade.go`

**Step 1: Implement the Relay Container logic**

```go
func (s *SelfUpgradeEngine) upgradeDocker(ctx context.Context) error {
    // 1. Pull latest image
    // 2. Create 'harborwatch-upgrade-relay' container
    // 3. Command: "docker stop harborwatch && docker rm harborwatch && docker run --name harborwatch ..."
    // 4. Start relay
}
```

**Step 2: Commit**

```bash
git commit -m "feat: implement docker relay upgrade path"
```

### Task 5: Add API Endpoints

**Files:**
- Modify: `backend/internal/httpapi/server.go`

**Step 1: Register routes**

```go
r.Route("/system/upgrade", func(r chi.Router) {
    r.Get("/status", getUpgradeStatus)
    r.Post("/check", checkUpdates)
    r.Post("/start", startUpgrade)
})
```

**Step 2: Commit**

```bash
git commit -m "feat: add self-upgrade api endpoints"
```

### Task 6: Implement Frontend UI

**Files:**
- Modify: `web/src/lib/pages/Settings.svelte`

**Step 1: Add "System Updates" section**

```svelte
<div class="px-2 py-1">
    <p class="text-xs font-black uppercase tracking-wider text-brand-500">System Updates</p>
</div>
<div class="rounded-2xl border border-slate-200 p-4">
    <!-- Status and Upgrade Button -->
</div>
```

**Step 2: Commit**

```bash
git commit -m "feat: add system updates ui in settings"
```
