# Stellar Compass Theme Refactor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the "Signpost" theme with the high-detail "Stellar Compass" aesthetic and integrate the new logo.

**Architecture:** Systematic replacement of CSS variables, background effects, and component styles. Removal of legacy attributions and roadmap updates.

**Tech Stack:** Svelte 5, Tailwind CSS, Vanilla CSS.

---

### Task 1: Logo Asset Integration
Integrate the high-detail compass logo into the application.

**Files:**
- Modify: `web/public/logo-64.png` (Update placeholder with new logo concept if possible, or use CSS to represent the logo's energy)
- Modify: `web/public/logo-128.png`
- Modify: `web/public/favicon.ico`

**Step 1: Replace logo assets**
(Note: As an AI, I will describe the placement and scaling of the logo in the UI since I cannot literally "create" a PNG, but I will ensure the `<img>` tags point to the correct high-res versions).

---

### Task 2: Stellar Compass CSS Refactor
Update the core styling to reflect the new technical nautical theme.

**Files:**
- Modify: `web/src/app.css`

**Step 1: Update Color Variables**
Replace the signpost palette with the "Void Navy", "Cyber Cyan", and "Data Emerald" palette.

**Step 2: Redesign Backgrounds**
Implement the "Tactical Grid" and "Deep Radial Glow" effects in both Light and Dark modes.

**Step 3: Sharpen Geometry**
Reduce global border radii to match the logo's sharp points.

---

### Task 3: Component Aesthetic Alignment
Align individual UI components with the new theme.

**Files:**
- Modify: `web/src/App.svelte`
- Modify: `web/src/lib/components/GlobalProgress.svelte`
- Modify: `web/src/lib/pages/Containers.svelte`

**Step 1: Update Header and Sidebar**
Scale the logo correctly and ensure text contrast against the new backgrounds.

**Step 2: Refactor Badges**
Switch to the "Outline + Glow" style for Portainer and Intel indicators.

---

### Task 4: About Page & Roadmap Cleanup
Remove legacy assets and update documentation.

**Files:**
- Modify: `web/src/lib/pages/About.svelte`
- Modify: `agents/MASTER_PLAN_V2.md`
- Modify: `CHANGELOG.md`

**Step 1: Remove Flaticon attribution**
Delete the "Icon Attribution" section from `About.svelte`.

**Step 2: Update Roadmap**
Mark Phase 6 (Aesthetic Refactor) as complete.
