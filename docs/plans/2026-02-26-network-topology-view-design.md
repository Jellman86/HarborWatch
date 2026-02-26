# Network Topology View Design

## Goal

Add a read-only Docker Network Topology view that visually maps user-defined Docker networks and the containers connected to them, using live runtime network data from Docker (not inferred labels).

## Scope (v1)

- User-defined Docker networks only (exclude default `bridge`, `host`, `none`)
- Snapshot view with manual refresh (no live updates)
- Read-only UI (no network create/delete/connect/disconnect actions)
- Interactive graph + details side panel

Out of scope (v1):

- Mutating network operations
- Force-directed layout
- Table/list fallback (can be added later)
- Live Docker event updates

## Source of Truth

Use Docker network inspect/runtime data as the canonical source:

- network metadata (driver, scope, internal, attachable)
- IPAM subnets/gateways
- connected container endpoints (IPv4, IPv6, MAC, aliases)

This ensures the view reflects the current configuration as it actually exists.

## Backend Design

### Endpoint

`GET /api/networks/topology`

Returns a normalized payload:

- `networks[]` — deduped network nodes
- `containers[]` — deduped container nodes connected to returned networks
- `edges[]` — network attachment edges (`container <-> network`)
- `generatedAt` — snapshot timestamp

### Docker integration

Add Docker client support for listing network topology from Docker Engine:

- list networks
- filter user-defined
- inspect network attachments/endpoints
- normalize into HarborWatch topology structs

### Filtering

v1 default filter: user-defined only

Exclude:

- `bridge`
- `host`
- `none`

Prefer runtime classification via driver/name/scope where available, with a conservative name-based fallback.

## Frontend Design

### Page

New `Networks` page:

- Header + manual refresh button
- Read-only safety banner
- Graph canvas (primary)
- Selection side panel (secondary)

### Graph model

Deterministic layered SVG layout:

- left column: network nodes
- right column: container nodes
- curved edges between them

Edge labels (IPs/aliases) show only on hover/selection.

### Selection side panel

If `network` selected:

- name/id
- driver/scope
- internal/attachable flags
- IPAM subnet/gateway
- attached containers

If `container` selected:

- name/id/image/state
- orchestration mode badge if available (future enrichment optional)
- connected networks with endpoint details

### Visual direction (v1)

- Networks: bold colored cards (driver-aware accents)
- Containers: compact neutral cards
- Subtle topology-grid background
- Hover/selection edge highlight
- Dim unrelated nodes/edges on selection

## Correctness / Robustness

- Backend tests for:
  - user-defined filtering
  - edge extraction and dedupe
  - empty topology behavior
- UI build verification and empty-state rendering

## Risks / Tradeoffs

- Large topologies may need scrolling and later clustering/table support
- Snapshot (manual refresh) can go stale between refreshes (acceptable in v1)
- Docker network inspect payload shape can vary; parser must be defensive
