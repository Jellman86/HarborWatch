# Documentation & Roadmap Update (v0.8.0 Refresh) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Systematically update all documentation in the `agents/` folder and the root `CHANGELOG.md` to reflect the current state of HarborWatch v0.8.0, focusing on Portainer integration, safety gates, and performance optimizations.

**Architecture:** Update Markdown documents to align with the "Autonomous Container Security Appliance" vision.

**Tech Stack:** Markdown

---

### Task 1: Update Root CHANGELOG.md

**Files:**
- Modify: `CHANGELOG.md`

**Step 1: Append detailed notes for v0.8.0**
Reflect the very latest changes:
- Portainer robust path detection (falling back to `/data/compose/` patterns).
- Portainer environment variable preservation during stack redeploy.
- Configurable CPU normalization toggle (System Total vs. Per-Core).
- Smart Polling for active jobs (5s -> 30s backoff).
- Safety gates blocking local updates on Portainer-managed containers.
- Enhanced UI indicators (Portainer logo badges).

---

### Task 2: Update agents/AUDIT_IMPROVEMENT_PLAN.md

**Files:**
- Modify: `agents/AUDIT_IMPROVEMENT_PLAN.md`

**Step 1: Update Audit Summary and Next Steps**
- Move "Toast Notifications" to completed (already done).
- Add Smart Polling and CPU normalization to performance goals.
- Mark UI Overhauls (Images, Global Progress) as completed.

---

### Task 3: Update agents/MASTER_PLAN_V2.md

**Files:**
- Modify: `agents/MASTER_PLAN_V2.md`

**Step 1: Move completed items to Phases 1-4 and add Phase 5 details**
- Formally document the Portainer API integration as part of Phase 5.
- Update "Next Steps" with the remaining items (Global Rules, Maintenance Windows).

---

### Task 4: Update agents/README.md

**Files:**
- Modify: `agents/README.md`

**Step 1: Refresh Project Overview**
- Ensure the description reflects the shift toward a full security appliance.
- Highlight the Portainer-native update flow as a key differentiator.

---

### Task 5: Update agents/AGENTS_OVERVIEW.md

**Files:**
- Modify: `agents/AGENTS_OVERVIEW.md`

**Step 1: Update Agent Grounding**
- Update technical context for future agents regarding Portainer detection logic.

---

### Task 6: Update agents/improvement_requests.md

**Files:**
- Modify: `agents/improvement_requests.md`

**Step 1: Synchronize feature requests**
- Mark today's requests (Portainer logo, user guidance) as implemented.
