# Reverse Proxy Configuration Guide

HarborWatch utilizes Server-Sent Events (SSE) for real-time Docker events and update progress tracking. For a stable experience behind a reverse proxy, specific configurations are required to prevent buffering and connection timeouts.

## Nginx Proxy Manager (NPM)

Paste the following into the **Advanced** tab of your Proxy Host configuration. Replace `<harborwatch-ip>` with your internal server IP.

```nginx
# Main Application Handling
location / {
    proxy_pass http://<harborwatch-ip>:18080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # Support for potential future WebSockets
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}

# Real-time Event Streams (SSE)
# Required for Live Docker Events and Update Progress
location ~ ^/api/(docker|updates)/events {
    proxy_pass http://<harborwatch-ip>:18080;
    proxy_set_header Host $host;
    
    # CRITICAL: Disable buffering for real-time streams
    proxy_buffering off;
    proxy_cache off;
    
    # Prevent connection drop for long-running streams (24h)
    proxy_read_timeout 86400s;
    proxy_send_timeout 86400s;
    
    # Ensure chunked transfer encoding works
    chunked_transfer_encoding on;
    
    # Reset connection headers for SSE
    proxy_http_version 1.1;
    proxy_set_header Connection "";
}

# API Timeout Hardening
# Prevents timeouts during long-running scans or update pulls
location /api/ {
    proxy_pass http://<harborwatch-ip>:18080;
    proxy_read_timeout 600s;
    proxy_send_timeout 600s;
    
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

## Traefik (Labels)

If using Traefik, add these labels to your `docker-compose.yml`:

```yaml
labels:
  - "traefik.http.routers.harborwatch.rule=Host(`harborwatch.yourdomain.com`)"
  - "traefik.http.services.harborwatch.loadbalancer.server.port=8000"
  # SSE and long-running job support
  - "traefik.http.middlewares.hw-buffering.buffering.retryExpression=IsNetworkError() && ResponseCode() == 502"
```

## Caddy

```caddy
harborwatch.yourdomain.com {
    reverse_proxy <harborwatch-ip>:18080 {
        header_up X-Real-IP {remote_host}
    }
}
```
