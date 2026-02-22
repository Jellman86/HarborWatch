# ⚙️ Automation & Self-Healing

The goal of HarborWatch isn't just to *show* you what's happening, but to take action when you're not looking. I've designed several layers of automation that work together to keep your apps running and up-to-date.

---

## 🚀 Lifecycle Manager
Updating containers is usually a manual chore. HarborWatch turns this into a reliable pipeline.

### How the Upgrade Flow Works:
1.  **Discovery**: HarborWatch checks registries (Docker Hub, GHCR, etc.) for newer image tags.
2.  **AI Risk Review**: If configured, the AI reads the release notes and changelogs to look for breaking changes.
3.  **Policy Gate**: Based on your per-container rules (Auto, Manual, or Locked), it decides whether to proceed.
4.  **Application**: It pulls the new image and redeploys the container (with Portainer support if available!).
5.  **Health Verification**: After updating, it waits to see if the container becomes `healthy`. If it stays unhealthy, it can **automatically roll back** to the previous version.

### Per-Container Rules
You can be as hands-on or hands-off as you like:
*   **Automatic**: "I trust this app, keep it updated for me."
*   **Manual**: "Tell me there's an update, but wait for me to click the button."
*   **Locked**: "Never touch this container."

---

## ❤️ Unhealthy Auto-Remediation
Sometimes containers just "get stuck." HarborWatch includes an event-driven daemon that listens to the Docker socket.

*   **Trigger**: If a container with a health check emits an `unhealthy` event, HarborWatch catches it immediately.
*   **Remediation**: It will attempt to restart the container to clear the failure.
*   **Circuit Breakers**: To prevent infinite restart loops, I've built in:
    *   **Cooldowns**: A wait time (e.g., 5 minutes) before it tries to restart the same container again.
    *   **Attempt Limits**: A hard cap (e.g., 3 restarts per hour). If it still fails, it stops and lets you know in the logs.
*   **Safety**: It will **never** attempt a remediation restart if an update or scan is already running for that container.

---

## 📅 The Scheduler
Almost every background task in HarborWatch is controlled by a flexible scheduler. You can set the time and cadence (Daily, Weekly, Monthly) for:
*   Update checks.
*   Security sweeps (Trivy & ClamAV).
*   Data retention cleanup.
*   Docker system pruning.

---

[⬅️ Back to Home](../README.md) | [🛡️ Security Features](FEATURES.md) | [🤖 AI Guide](AI_GUIDE.md)
