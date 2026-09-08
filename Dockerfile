# --- Stage 1: Build the Frontend ---
FROM node:20-alpine AS frontend-builder
WORKDIR /build

# Copy frontend specifications and sources
COPY frontend/package.json frontend/package-lock.json frontend/tsconfig.json frontend/vite.config.ts frontend/index.html ./
COPY frontend/src ./src
COPY frontend/public ./public

# Install dependencies and compile SPA distribution
RUN --mount=type=cache,target=/root/.npm \
    npm install && \
    npm run build

# --- Stage 2: Build the Go Binary ---
FROM golang:1.26-bookworm AS backend-builder
WORKDIR /build

ENV GOTOOLCHAIN=auto

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy Go backend sources
COPY cmd ./cmd
COPY internal ./internal

# Compile statically linked production binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o node_monitor ./cmd/node_monitor

# --- Stage 3: Runtime ---
FROM debian:bookworm-slim
WORKDIR /app

# Install CA certificates for external DNS API HTTPS requests
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy compiled Go executable
COPY --from=backend-builder /build/node_monitor /app/node_monitor

# Copy compiled SPA distribution
COPY --from=frontend-builder /build/dist /app/frontend/dist

# Expose default daemon port
EXPOSE 8080

# Create data persistence directory
RUN mkdir -p /app/data

# Run daemon pointing to persistent SQLite DB
CMD ["/app/node_monitor", "--db", "/app/data/node_monitor.db", "daemon", "--port", "8080"]
