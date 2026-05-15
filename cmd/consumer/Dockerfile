# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build
WORKDIR /src

# The consumer is its own Go module; copy only what's needed.
COPY cmd/consumer/go.mod cmd/consumer/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/consumer/ ./

ARG TARGETARCH
ARG VERSION=dev
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath \
      -ldflags="-s -w" \
      -o /out/consumer ./

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source="https://github.com/jorgejr568/organizze-mcp"
LABEL org.opencontainers.image.description="Stats consumer for organizze-mcp — drains the SQS stats queue into Postgres"

COPY --from=build /out/consumer /usr/local/bin/consumer

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/consumer"]
