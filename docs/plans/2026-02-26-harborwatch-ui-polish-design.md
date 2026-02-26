# HarborWatch UI Polish Design (Automations, Lifecycle, Stacks, Settings)

**Date:** 2026-02-26

## Goal
Implement the UI improvements requested in `agents/improvement_requests.md` across Automations, Fleet lifecycle sections, Stacks, and Settings with minimal backend impact.

## Scope
- Automations:
  - Move Upgrade "Default Policy" segment below other upgrade controls
  - Remove "Task Status Overview" explainer
  - Increase flow diagram spacing
  - Improve Security automation grouping (Trivy / ClamAV)
- Fleet lifecycle (`ContainerPage`):
  - Replace dirty-state dot with icon states (tick / ! / x)
  - Move Lifecycle Log below Breaking Change Signals in a 2-column layout
- Stacks:
  - Add view tabs for local compose projects vs Portainer stacks
- Settings:
  - Dirty-state status badge uses icon states (tick / ! / x)
  - AI page split into `Settings` and `Costs` sub-tabs
  - Add top-level `Backups` tab and move compose snapshot config there
  - Improve icon consistency in settings navigation / touched sections

## Architecture / Approach
- Frontend-only changes in Svelte pages/components:
  - `web/src/lib/pages/Settings.svelte`
  - `web/src/lib/pages/ContainerPage.svelte`
  - `web/src/lib/pages/Stacks.svelte`
  - `web/src/lib/components/AutomationFlowChart.svelte`
- Reuse existing settings fields and schedule APIs. No backend schema or API changes planned.
- Track save error state per signature in Settings/Lifecycle to support red error status without losing pending-state behavior after further edits.

## Non-Goals
- Full settings-wide icon redesign
- Backend validation changes for settings/rules
- New test harness setup (repo currently lacks frontend test framework)

## Verification Plan
- `npm --prefix web run build` (primary compile gate)
- Spot-check changed Svelte files for syntax and type errors via build output
- Optional manual UI smoke if app is running

