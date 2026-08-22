# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install git for module fetching (some deps resolve via VCS)
RUN apk add --no-cache git ca-certificates

# Dependency caching: copy manifests first so this layer is reused
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source
COPY . .

# Build a static binary (CGO disabled; lib/pq is pure Go)
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/app .

# ---- Runtime stage ----
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -u 10001 -G app app

WORKDIR /app

# Application binary and runtime assets (HTML templates are read at runtime)
COPY --from=builder /out/app /app/app
COPY --from=builder /src/templates /app/templates

USER 10001:10001

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/app"]
