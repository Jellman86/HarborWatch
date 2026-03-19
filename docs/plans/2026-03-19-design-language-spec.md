# HarborWatch Design Language Spec

**Date:** 2026-03-19  
**Status:** Draft / Living Document  
**Applies To:** All new HarborWatch UI work unless a page explicitly documents an exception

## 1. Purpose
This document defines the visual and interaction language for HarborWatch so new UI elements can be added consistently even when implementation work is spread across time or different sessions. It is not a brand essay. It is an operating spec for building pages, cards, controls, and states that feel like the same application.

## 2. Core Principles
- HarborWatch is an operator console, not a marketing site.
- Every surface should answer three questions quickly: what is this, what state is it in, what can I do next.
- Layout density is acceptable when information is structured clearly.
- Visual depth should come from layered cards, state treatments, and section shells, not from random decoration.
- New UI should prefer stronger hierarchy over more text.

## 3. Page Shell Rules
- Standard management pages should use the same content width and outer spacing rhythm as the strongest HarborWatch views.
- Page headers should include a title, a one-line operational subtitle, and a right-aligned action cluster when relevant.
- Section transitions should be obvious through spacing and shell changes, not by stacking unrelated panels with identical treatments.
- Avoid narrow layouts on data-heavy management pages unless there is a specific reading-mode need.

## 4. Card System
### 4.1 Card Anatomy
Every operational card should have three predictable zones:
- Identity: title, source, path, image, or object name
- State: status chips, alerts, activity, health, sync/redeploy/deploy state
- Controls: the actions an operator can take now

### 4.2 Card Hierarchy
- Primary cards represent managed objects: repository, stack, compose project, container, deployment rule.
- Secondary cards represent subordinate managed objects inside a primary card.
- Metadata chips should summarize state; they should not replace explanatory text when context is needed.

### 4.3 Visual Treatment
- Use rounded cards with visible border definition and subtle depth.
- Flat white panels without sectional hierarchy should be avoided on management pages.
- Card hover states may sharpen borders and deepen shadow, but should not cause layout movement.

## 5. Status and State Rules
- Status must be visible on the object itself, not only in a global header.
- Running work uses animated but restrained treatment: pulse, shimmer, or static-like texture only where it communicates active processing.
- Success, warning, danger, inactive, queued, and running should have stable semantic treatments reused across pages.
- Read-only, discovered, auto-created, and operator-managed states should have distinct labels and descriptions.

## 6. Action Hierarchy
- Each card should have one visually dominant primary action.
- Secondary actions should be grouped and clearly subordinate.
- Destructive actions should never compete visually with the primary action.
- Ambiguous labels such as `Manage` should be avoided when a more specific verb exists.

## 7. Iconography Rules
- Icons must reinforce the action or object type, not decorate it.
- Repository, stack, compose, sync, redeploy, deploy, edit, and delete should each have a clear and stable icon mapping.
- Avoid icons that suggest a different object class than the one being represented.

## 8. Copy Rules
- Prefer short operator-facing labels.
- Subtext should explain implications, not repeat the title.
- Status summaries should be direct and specific.
- Empty states should tell the operator what to do next.

## 9. Motion Rules
- Motion is for state communication, not ornament.
- Use pulse/static/shimmer only for active jobs, discovery, sync, or redeploy/deploy states.
- Avoid multiple competing animated surfaces in the same viewport.

## 10. Mobile Rules
- Card structure must survive stacking on narrow widths.
- Action groups should collapse cleanly without overlapping badges or sticky chrome.
- Metadata should wrap without producing unreadable walls of chips.

## 11. Empty, Loading, and Error States
- Loading states should mirror the final structure when possible.
- Empty states should use a dedicated shell, not just a blank area with one line of text.
- Error states should explain what failed and what the operator can do next.

## 12. Practical Do / Don't Rules
### Do
- Turn important objects into cards with identity, state, and actions.
- Use badges and metadata rows to improve scanability.
- Show active operations directly on affected cards.
- Normalize width and spacing across related pages.

### Don't
- Hide important state in hover-only affordances.
- Use flat panels for complex operational surfaces.
- Mix unrelated control layouts within the same page family.
- Add new components that ignore the established card and status grammar.

## 13. Evolution Rule
This document is expected to grow. When a new page introduces a valid pattern not yet covered here, update this spec so future work can reuse it deliberately.
