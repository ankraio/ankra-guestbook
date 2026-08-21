# Build a static binary so the runtime image can be distroless.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/guestbook .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/guestbook /guestbook
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/guestbook"]
