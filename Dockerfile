# syntax=docker/dockerfile:1

# ---------- build stage ----------
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Dependency layer first for effective caching
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Application source
COPY . .

# Static, stripped binary (no CGO so it runs on a minimal base image)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /out/app .

# ---------- runtime stage ----------
FROM alpine:3.20 AS runtime

# TLS roots for outbound connections (e.g. Postgres over SSL)
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 10001 -S app \
    && adduser -u 10001 -S app -G app

WORKDIR /app

COPY --from=builder /out/app /app/app
# Templates are rendered at runtime and must ship with the binary
COPY --from=builder /src/templates /app/templates

USER 10001:10001

ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["/app/app"]
