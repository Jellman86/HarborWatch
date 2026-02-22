# ⚙️ Configuration & Setup

HarborWatch is designed to be a "single container" appliance. Everything—the backend, the UI, and the database—lives inside one Docker image. This makes it very easy to deploy and move between hosts.

---

## 🛠️ Environment Variables
While you can change most settings in the UI, you can also pre-configure HarborWatch using environment variables in your `docker-compose.yml`.

| Variable | Description | Default |
| :--- | :--- | :--- |
| `HARBORWATCH_DB_PATH` | Path to the SQLite database file. | `/data/harborwatch.db` |
| `PORT` | The internal port the server listens on. | `8080` |
| `PUID` / `PGID` | User/Group ID for file permissions. | `1000`/`1000` |
| `AI_PROVIDER` | `openai`, `anthropic`, or `gemini`. | `""` |
| `OPENAI_API_KEY` | Your OpenAI secret key. | `""` |
| `ANTHROPIC_API_KEY` | Your Anthropic secret key. | `""` |
| `GEMINI_API_KEY` | Your Google Gemini secret key. | `""` |
| `PORTAINER_URL` | Base URL for your Portainer instance. | `""` |
| `PORTAINER_API_KEY` | Portainer API access token. | `""` |
| `DISCORD_WEBHOOK_URL` | For system notifications. | `""` |
| `HW_DATA_RETENTION_DAYS`| How many days of history to keep. | `30` |

---

## 💾 Persisting Data
I strongly recommend mounting a volume for the database. If you don't, you'll lose your history and rules every time you update HarborWatch!

**Example Compose Snippet:**
```yaml
services:
  harborwatch:
    image: jellman86/harborwatch:latest
    container_name: harborwatch
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock # Required to manage containers
      - ./data:/data # Keep your settings and history safe
    ports:
      - 8080:8080
    restart: unless-stopped
```

## 🐳 Docker Socket
HarborWatch needs access to `/var/run/docker.sock` to see your containers, read their logs, and perform updates. I've built the backend to be lightweight and read-only whenever possible, only taking "write" actions (like restarting or pulling images) when you or your automation rules specifically request it.

---

[⬅️ Back to Home](../README.md) | [📊 Features](FEATURES.md) | [🔗 Integrations](INTEGRATIONS.md)
