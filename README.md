# HarborWatch

HarborWatch is a professional, local-first container maintenance and security appliance for self-hosted Docker environments. It transforms passive monitoring into an active, asset-centric security strategy.

## Key Features

- **Container Command Center:** Dedicated full-page views for every container with deep insights into metrics, security history, and lifecycle status.
- **Fleet Management:** modern Card and List views for your entire container inventory with real-time "Update Available" detection.
- **Automated Compose Doctor:** Zero-config security auditing. HarborWatch automatically retrieves or reconstructs your `docker-compose.yml` for AI-powered security analysis.
- **Per-Container Lifecycle Policies:** Fine-grained control over updates. Set "Auto", "Manual", or "Locked" policies per asset with custom health check validation.
- **Dual-Engine Security Scanning:** Integrated vulnerability (Trivy) and malware (ClamAV) scanning with shared database persistence.
- **High-Resolution Performance Profiling:** Real-time sparklines and detailed historical charts for CPU and Memory utilization.
- **Unified Configuration:** Seamlessly merge `docker-compose` environment variables with persistent database settings.

## Technology Stack

- **Backend:** Go 1.26 with `go-chi` router and official Moby Docker SDK.
- **Frontend:** Svelte 5 (Runes) with Tailwind CSS and ApexCharts.
- **Database:** SQLite (Embedded) for persistence of scans, rules, and metrics.
- **AI Core:** OpenAI integration for release note analysis and security auditing.

## Quick Start

1. Generate shared API types:
```bash
scripts/generate-types.sh
```

2. Start the appliance (Development Mode):
```bash
scripts/dev.sh
```

3. Access the UI at `http://localhost:18080`.

## Architecture Note

HarborWatch is designed as a **single monolithic container**. It serves the REST API, background jobs, and the built Svelte static assets from a single Go binary. No Node.js or complex sidecars are required in production.

## Environment Overrides

HarborWatch prioritizes standard Docker environment variables for configuration. If defined in your `docker-compose.yml`, these values will be locked in the UI:
- `DISCORD_WEBHOOK_URL`
- `OPENAI_API_KEY`
- `PORTAINER_URL` / `PORTAINER_API_KEY`
- `HW_INSTANCE_URL`

## CI/CD

- **PR Validation:** Automatic linting and backend tests.
- **Build & Push:** Automatic image generation to `ghcr.io/jellman86/harborwatch:dev` on every push to the `dev` branch.
- **Releases:** Versioned tags trigger production builds.
