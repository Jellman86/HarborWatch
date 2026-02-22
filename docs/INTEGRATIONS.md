# 🔗 Integrations

HarborWatch doesn't live in a bubble. I've designed it to play nice with the tools you're likely already using.

---

## 🏗️ Portainer
If you manage your containers using Portainer "Stacks" (docker-compose), you probably know that updating an image manually can sometimes lose your environment variables or secret configurations.

HarborWatch solves this with a **deep Portainer integration**.

*   **How it works**: You provide your Portainer URL and an API Key.
*   **Stack-Aware Updates**: When HarborWatch applies an update, it doesn't just run `docker pull`. It tells Portainer to "re-deploy" the stack. 
*   **Safety**: This ensures that Portainer remains the "source of truth" for your configuration, and all your secrets/labels are perfectly preserved.

## 🔔 Discord
Sometimes you just want to know when things happen without checking a dashboard.

*   **Notifications**: Get real-time alerts for:
    *   New updates found.
    *   Successful (or failed) upgrades.
    *   Critical security vulnerabilities found in a scan.
    *   Remediation restarts (self-healing events).
*   **Easy Setup**: Just paste in a Discord Webhook URL, and you're good to go.

---

### How to get a Discord Webhook:
1.  Go to your Discord Server settings.
2.  Select **Integrations** > **Webhooks**.
3.  Create a "New Webhook" and copy the URL.
4.  Paste it into the **Settings > Integrations** tab in HarborWatch.

---

[⬅️ Back to Home](../README.md) | [🤖 AI Features](AI_GUIDE.md) | [⚙️ Configuration](CONFIGURATION.md)
