# Compose Source-of-Truth Update Automation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent HarborWatch automated upgrades from diverging from compose/Portainer-declared image references while keeping manual updates unchanged.

**Architecture:** Add compose source-resolution and image-ref equivalence helpers in `httpapi`, then enforce strict compose-source checks in `buildUpdateRequestForContainer` only when `RequireAutoPolicy` is enabled. This ensures all auto-apply paths inherit the protection without changing manual `/updates/run` behavior.

**Tech Stack:** Go, chi/httpapi, yaml.v3, existing Portainer client + update automation pipeline

---

### Task 1: Add failing tests for strict compose-source auto enforcement

**Files:**
- Modify: `backend/internal/httpapi/update_apply_task_test.go`

**Step 1: Write the failing tests**
- Add test: Portainer-managed compose container with stack YAML service image different from runtime image is skipped by auto-apply.
- Add test: Compose-managed container without source verification path is skipped by auto-apply.

**Step 2: Run tests to verify they fail**
Run: `go test ./internal/httpapi -run 'TestAutomatedUpdateApplyTask_(SkipsWhenPortainerComposeSourceDriftDetected|SkipsComposeManagedWhenSourceCannotBeVerified)'`
Expected: FAIL because auto-apply currently starts updates.

**Step 3: Commit (after implementation in Task 3)**
- Commit with implementation slice.

### Task 2: Implement compose source resolution + image-ref equivalence helpers

**Files:**
- Create: `backend/internal/httpapi/compose_source_authority.go`
- Test (covered indirectly first): `backend/internal/httpapi/update_apply_task_test.go`

**Step 1: Add helper(s)**
- Detect compose-managed containers via compose labels.
- Resolve Portainer stack id + service name from labels.
- Fetch stack YAML via Portainer client and extract service `image`.
- Add normalized image-ref equivalence helper using existing normalization/variant helpers.

**Step 2: Keep helpers internal and conservative**
- Return explicit reasons for source unavailable / parse failure.
- Do not mutate behavior outside auto mode yet.

### Task 3: Enforce strict compose-source checks in auto request builder path

**Files:**
- Modify: `backend/internal/httpapi/update_request_builder.go`
- Modify: `backend/internal/httpapi/update_apply_task.go` (skip reason classification only, if needed)

**Step 1: Wire enforcement behind `RequireAutoPolicy`**
- If compose-managed:
  - require source verification
  - block on runtime/source drift
  - block on target/source divergence
- Manual path (`RequireAutoPolicy=false`) unchanged.

**Step 2: Run tests**
Run: `go test ./internal/httpapi -run 'TestAutomatedUpdateApplyTask_(StartsAutoContainersWithUpdates|SkipsWhenPortainerComposeSourceDriftDetected|SkipsComposeManagedWhenSourceCannotBeVerified)'`
Expected: PASS

**Step 3: Commit**
```bash
git add backend/internal/httpapi/update_apply_task_test.go backend/internal/httpapi/compose_source_authority.go backend/internal/httpapi/update_request_builder.go backend/internal/httpapi/update_apply_task.go docs/plans/2026-02-24-compose-source-of-truth-updates-*.md
git commit -m "feat: enforce compose source of truth for auto updates"
```

### Task 4: Broader verification and changelog note

**Files:**
- Modify: `CHANGELOG.md`

**Step 1: Run broader verification**
Run: `go test ./internal/httpapi ./internal/updates ./internal/rules ./internal/settings`
Expected: PASS

**Step 2: Update changelog**
- Add `Unreleased` note for strict compose-source auto-update enforcement.

**Step 3: Commit**
```bash
git add CHANGELOG.md
git commit -m "docs: note compose source auto-update enforcement"
```
