# HarborWatch Settings UI Polish Plan

## Objective
Transform the Settings view from a "card-heavy" nested structure to a modern, clean, and beautiful interface. It must be highly responsive, easily readable, and adhere to contemporary design practices (e.g., flat design, thoughtful whitespace, clear visual hierarchy).

## Current Issues Identified from Playwright Audit
1. **Nested Cards:** The UI still relies on "boxes within boxes." The entire settings area is wrapped in a large bordered card, and individual settings/sections are wrapped in further bordered cards. This creates visual clutter.
2. **Heavy Tab Wrappers:** The main tab and sub-tab navigations use thick background wrappers that enclose pill buttons. This adds unnecessary visual weight.
3. **Input Styling:** Inputs and selects are full-width bordered boxes that feel disconnected from their labels.
4. **Mobile Constraints:** Horizontal tab bars and complex flex layouts may not scale down perfectly to mobile without squishing or awkward wrapping.

## Fundamental Improvements Plan

### 1. Flatten the Container Hierarchy
- **Remove the Master Card Wrapper:** The `settings-shell` should not look like a floating white box inside the page. It should integrate seamlessly with the main content area, or at least lose its heavy outer border and drop shadow.
- **Section Dividers over Boxes:** Replace the `border border-slate-200 rounded-2xl` section wrappers with clean, full-width subtle bottom borders (`border-b border-slate-200/50`) and generous vertical padding (`py-8`).

### 2. Modernize Navigation (Tabs)
- **Scrollable Tab Bars:** Ensure both main tabs and sub-tabs use `overflow-x-auto` and `whitespace-nowrap` on mobile to prevent ugly wrapping. Hide the scrollbar for a cleaner look.
- **Underline Tabs or Sleek Pills:** Replace the heavy `bg-slate-100` wrappers with either a clean transparent container using underline indicators for the active state, or minimalist transparent pills that turn colored when active.

### 3. Elevate the Typography & Layout
- **Two-Column Layout for Desktop:** On larger screens, use a CSS grid layout where the setting label and description are on the left, and the input/toggle/action is on the right. This is a common pattern in macOS, iOS, and modern web apps (like Vercel).
  - *Example:* `<div class="grid grid-cols-1 md:grid-cols-3 gap-6">` -> Left col spans 1 or 2 (Title/Desc), Right col contains the control.
- **Responsive Stack:** On mobile, this layout naturally stacks vertically, providing an excellent mobile experience.

### 4. Refine Controls (Toggles, Selects, Inputs)
- **Subtle Inputs:** Use inputs with a subtle background (`bg-slate-50 dark:bg-slate-900`), removing heavy borders until focused (`focus:ring-2 focus:ring-brand-500`).
- **Right-Aligned Controls:** For toggle switches or small select dropdowns, right-align them against the setting text on desktop to create a neat "ragged right" edge of controls.

### 5. Beautiful Section Headers (Hero Banners)
- Keep the gradient hero banners (`settings-pane-hero`) but remove their borders. Let them act as vibrant, flat, full-width headers for each major settings category.
- Reduce their bottom margin slightly so they connect better with the subsequent list of settings.

## Implementation Steps (Actionable)
1. **Refactor `Settings.svelte` Layout:** Strip out the outer `border` and `rounded-3xl` from the main `settings-shell`.
2. **Update Tabs:** Modify `.settings-tab-strip` and `.settings-subtab-strip` to use `flex-nowrap overflow-x-auto hide-scrollbar` and remove their heavy background wrappers.
3. **Apply Two-Column Design:** Iterate through each setting (e.g., Global Task Concurrency, Automation Safety Exclusions) and convert the markup to the `grid grid-cols-1 md:grid-cols-[1fr_auto]` or `md:grid-cols-3` layout.
4. **Remove Remaining Inner Borders:** Target any remaining `border-slate-200` classes on inner elements (like metric blocks or lists) and replace them with very soft background tints (`bg-slate-50/50`).
5. **Mobile Testing:** Run playwright audits specific to mobile viewports to guarantee horizontal scroll works and layout stacks cleanly.
