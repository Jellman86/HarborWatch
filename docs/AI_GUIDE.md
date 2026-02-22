# 🤖 AI Copilot Guide

HarborWatch doesn't *need* AI to work, but it certainly makes life easier. I wanted to see if I could use LLMs to help with the "human reasoning" part of server management—like reading long changelogs or finding security smells in a config file.

HarborWatch supports **OpenAI**, **Anthropic (Claude)**, and **Google Gemini**.

---

## 🧐 Release Analysis
When an update is found, HarborWatch can ask the AI to summarize what's new. It specifically looks for "Breaking Change Signals."

**Example Assessment:**
> ⚠️ **Medium Risk**: Version 2.0 of this database image removes the deprecated `LEGACY_AUTH` environment variable. If you haven't migrated to the new auth system, your container will fail to start.

This allows the "Automatic" update policy to be even smarter—it can block an update if the AI detects a high risk, even if you told it to be automatic.

## 🩺 Compose Doctor
You can point the AI at your `docker-compose.yml` files (or HarborWatch can discover them automatically). It will perform a security and best-practice audit.

**Example Recommendation:**
> 🔒 **Security Tip**: Your `app` service is running as `root`. Consider adding `user: 1000:1000` to your compose file to follow the principle of least privilege.

## 📈 Performance Insights
If you see a spike in a container's CPU chart, you can ask the AI to analyze the metrics history. It looks for patterns like memory leaks or CPU cycles that coincide with specific times of day.

---

## 🛠️ Configuration & History
*   **Provider Choice**: You can swap between providers in the settings. I've found that GPT-4o or Claude 3.5 Sonnet work best for these technical tasks.
*   **AI Usage**: Because API calls cost money, HarborWatch includes a token tracker and cost estimator so you can keep an eye on your spend.
*   **Conversation Log**: Want to see exactly what was asked and what was answered? There's a full history log available in the settings tab.

---

[⬅️ Back to Home](../README.md) | [⚙️ Automation Logic](AUTOMATION.md) | [🔗 Integrations](INTEGRATIONS.md)
