# 🐳 Project Features

HarborWatch was built to be a friendly, all-in-one companion for your Docker fleet. I wanted to create something that handles the boring stuff (like checking for updates and cleaning up logs) so you can focus on actually using your self-hosted services.

Here is a breakdown of what HarborWatch can do for you.

---

## 📊 Fleet Dashboard
The dashboard is the "command center" for your host. It gives you a high-level view of everything running without being overwhelming.

*   **Real-time Metrics**: See CPU and Memory usage for every container with pretty sparkline charts. It helps you spot a "runaway" container at a glance.
*   **Disk Usage Analysis**: HarborWatch calculates the writable layer and mount sizes, so you know exactly which container is eating up your SSD.
*   **Quick Actions**: Need to restart a container or check its logs? You can do it right from the dashboard or the detail page.
*   **Health Indicators**: It pulls in Docker's native health checks, so you can see if a container is `healthy`, `starting`, or `unhealthy` immediately.

## 🛡️ Security Hub
Keeping your home server safe shouldn't require a degree in cybersecurity. HarborWatch integrates industry-standard tools directly into the UI.

*   **Vulnerability Scanning (Trivy)**: Scans your container images for known security holes (CVEs). It categorizes them by severity so you know what needs an urgent update.
*   **Malware Detection (ClamAV)**: This is one of my favorite features. It can scan not just the container's internal files, but also your **host mounts**. If you have a media folder or a download directory, HarborWatch can keep an eye on it.
*   **Automatic Signature Updates**: The malware database stays fresh automatically, so you're always protected against the latest threats.

## 🧹 Maintenance & Housekeeping
A clean server is a happy server. HarborWatch handles the digital dust for you.

*   **Docker System Prune**: Automatically removes dangling images, unused networks, and stopped containers to reclaim disk space.
*   **Historical Data Retention**: Instead of letting the database grow forever, you can set a "Retention Window" (like 1 month). HarborWatch will then neatly prune old metrics and logs to keep things snappy.
*   **Manual Wipe**: If you ever want a fresh start with your stats, there's a "Clear All History" button that resets everything while keeping your settings safe.

---

[⬅️ Back to Home](../README.md) | [🤖 Read about AI Automation](AI_GUIDE.md) | [⚙️ Configuration Guide](CONFIGURATION.md)
