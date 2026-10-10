# syntax=docker/dockerfile:1

# ── Stage 1: Build UI ─────────────────────────────────────────────────────────
FROM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci
COPY ui/ .
RUN npm run build

# ── Stage 2: Build Go binary ──────────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Embed the pre-built UI
COPY --from=ui-builder /app/assets/dist ./assets/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /mockly ./cmd/mockly

# ── Stage 3: Final image ──────────────────────────────────────────────────────
FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
    && addgroup -S mockly && adduser -S -G mockly -h /config mockly

COPY --from=go-builder --chown=mockly:mockly /mockly /usr/local/bin/mockly

# Config is mounted at runtime; /config is the default working directory
WORKDIR /config
RUN chown mockly:mockly /config
VOLUME ["/config"]

EXPOSE 8080 9091

# Run as a dedicated non-root user — mockly only ever listens on ports
# >1024 (see EXPOSE above), so no root privileges are needed at runtime.
USER mockly

ENTRYPOINT ["mockly"]
CMD ["start", "-c", "/config/mockly.yaml"]
