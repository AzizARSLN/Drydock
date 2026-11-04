# syntax=docker/dockerfile:1

# ── Stage 1: build the web UI ───────────────────────────────────────────────
FROM node:20-alpine AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# vite.config.ts emits the bundle into ../backend/web (i.e. /src/backend/web).
RUN npm run build

# ── Stage 2: build the Go control plane ─────────────────────────────────────
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /src/backend/web ./web
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/drydock .

# ── Stage 3: minimal runtime ────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 10001 -h /app drydock
WORKDIR /app
COPY --from=api /out/drydock /app/drydock
COPY --from=api /src/web /app/web

ENV DRYDOCK_ADDR=0.0.0.0:8788 \
    DRYDOCK_DATA_DIR=/data \
    DRYDOCK_DB=/data/drydock.db \
    DRYDOCK_WEB_DIR=/app/web

RUN mkdir -p /data && chown -R drydock:drydock /data
VOLUME ["/data"]
USER drydock
EXPOSE 8788

ENTRYPOINT ["/app/drydock"]
