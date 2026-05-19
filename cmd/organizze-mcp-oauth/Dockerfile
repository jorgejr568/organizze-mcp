# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETARCH
ARG VERSION=dev
ARG INGEST_URL=
ARG INGEST_TOKEN=
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath \
      -ldflags="-s -w \
        -X 'github.com/jorgejr568/organizze-mcp/internal/adapter/mcp.Version=${VERSION}' \
        -X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestURL=${INGEST_URL}' \
        -X 'github.com/jorgejr568/organizze-mcp/internal/stats.DefaultIngestToken=${INGEST_TOKEN}'" \
      -o /out/organizze-mcp-oauth ./cmd/organizze-mcp-oauth

FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.source="https://github.com/jorgejr568/organizze-mcp"
LABEL org.opencontainers.image.description="Multi-tenant MCP server for the Organizze API with embedded OAuth 2.1 Authorization Server"

COPY --from=build /out/organizze-mcp-oauth /usr/local/bin/organizze-mcp-oauth

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/organizze-mcp-oauth"]
