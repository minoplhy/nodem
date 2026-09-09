# ==============================================================================
# Multi-stage Dockerfile: Node Monitor + ECH Manager
# Bundles OpenSSL with native ECH support, compiled frontend, and Go binaries
# ==============================================================================

# --- Stage 1: Build OpenSSL 4.0.2 with Native ECH Support ---
FROM alpine:latest AS openssl-builder

RUN apk add --no-cache \
    git \
    build-base \
    perl \
    linux-headers

WORKDIR /src

ARG OPENSSL_TAG=openssl-4.0.2
RUN git clone --depth 1 --branch "${OPENSSL_TAG}" https://github.com/openssl/openssl.git

WORKDIR /src/openssl

RUN ./Configure \
    --prefix=/opt/openssl \
    --openssldir=/opt/openssl/ssl \
    no-shared \
    no-tests \
    no-docs \
    no-unit-test

RUN make -j"$(nproc)"
RUN make install_sw

# --- Stage 2: Build the Frontend ---
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

# --- Stage 3: Build Go Binaries ---
FROM golang:1.26-bookworm AS backend-builder
WORKDIR /build

ENV GOTOOLCHAIN=auto

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy Go backend sources and frontend package for embedding
COPY cmd ./cmd
COPY internal ./internal
COPY frontend ./frontend

# Copy compiled SPA distribution from frontend-builder into frontend/dist for Go binary embedding
COPY --from=frontend-builder /build/dist ./frontend/dist

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=""

# Compile statically linked production binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/minoplhy/nodem/internal/version.Version=${VERSION} -X github.com/minoplhy/nodem/internal/version.Commit=${COMMIT} -X github.com/minoplhy/nodem/internal/version.BuildDate=${BUILD_DATE}" \
    -o /build/nodem ./cmd/nodem

# --- Stage 4: Runtime ---
FROM alpine:latest
WORKDIR /app

# Install CA certificates for external DNS API HTTPS requests, tzdata, and basic tools
RUN apk add --no-cache ca-certificates tzdata curl bash

# Copy compiled OpenSSL binaries and libraries with native ECH support
COPY --from=openssl-builder /opt/openssl /opt/openssl
ENV PATH="/opt/openssl/bin:${PATH}"

# Copy compiled Go binary and create compatibility symlink
COPY --from=backend-builder /build/nodem /app/nodem
RUN ln -sf /app/nodem /app/node_monitor

# Copy compiled SPA distribution
COPY --from=frontend-builder /build/dist /app/frontend/dist

# Expose HTTP web/api daemon port (8080) and ECH SSH pull server port (34234)
EXPOSE 8080 34234

# Create data persistence directory
RUN mkdir -p /app/data

# Run daemon pointing to persistent SQLite DB
CMD ["/app/nodem", "--db", "/app/data/nodem.db", "daemon"]
