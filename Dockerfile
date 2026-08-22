2# syntax=docker/dockerfile:1

########################################
# Build stage
########################################
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install git for module fetching over https (and ca-certs for TLS)
RUN apk add --no-cache git ca-certificates

# Dependency caching: copy manifests first
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source
COPY . .

# Static build, no cgo, stripped binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/app .

########################################
# Runtime stage
########################################
FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 -S app \
    && adduser -u 10001 -S -G app app

WORKDIR /app

COPY --from=builder /out/app /app/app
# Runtime assets (HTML templates rendered by main.go)
COPY --from=builder /src/templates /app/templates

ENV PORT=8080

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/app/app"]
