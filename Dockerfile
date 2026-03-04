# syntax=docker/dockerfile:1

FROM node:22 AS web-builder
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26 AS go-builder
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/harborwatch ./cmd/server

FROM debian:bookworm-slim
ARG APP_VERSION=dev
ARG GIT_HASH=unknown
ARG TRIVY_VERSION=0.69.3

ENV HARBORWATCH_VERSION=${APP_VERSION}
ENV HARBORWATCH_DB_PATH=/data/harborwatch.db
ENV PORT=8000

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    docker.io \
    gnupg \
    lsb-release \
    clamav \
    clamav-daemon \
    && TRIVY_TARBALL="trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" \
    && curl -sfL "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/${TRIVY_TARBALL}" -o "/tmp/${TRIVY_TARBALL}" \
    && curl -sfL "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_checksums.txt" -o /tmp/trivy_checksums.txt \
    && (cd /tmp && grep " ${TRIVY_TARBALL}$" trivy_checksums.txt | sha256sum -c -) \
    && tar -xzf "/tmp/${TRIVY_TARBALL}" -C /tmp trivy \
    && install -m 0755 /tmp/trivy /usr/local/bin/trivy \
    && rm -f "/tmp/${TRIVY_TARBALL}" /tmp/trivy_checksums.txt /tmp/trivy \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -u 1000 appuser && \
    mkdir -p /app/backend /app/web/dist /data && \
    chown -R appuser:appuser /app /data

WORKDIR /app/backend
COPY --from=go-builder /out/harborwatch /app/backend/harborwatch
COPY --from=web-builder /src/web/dist /app/web/dist

USER appuser

EXPOSE 8000

HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8000/health || exit 1

CMD ["./harborwatch"]
