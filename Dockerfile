# syntax=docker/dockerfile:1

############################
# Stage 1: build
############################
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Build tooling needed for module fetching
RUN apk add --no-cache ca-certificates git

# Dependency layer first for better caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Application sources
COPY . .

# Static, stripped binary (multi-arch aware via buildx TARGET* args)
ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/app ./...

############################
# Stage 2: runtime
############################
FROM alpine:3.22 AS runtime

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -g 10001 -S app \
    && adduser -u 10001 -S app -G app

WORKDIR /app

# Binary and runtime assets (HTML templates)
COPY --from=builder /out/app /app/app
COPY --from=builder /src/templates /app/templates

ENV PORT=8080

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/app/app"]
