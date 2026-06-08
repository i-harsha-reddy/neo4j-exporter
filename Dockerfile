# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder
WORKDIR /src

ARG VERSION=dev
ARG REVISION=unknown
ARG BRANCH=unknown
ARG BUILD_USER=docker
ARG BUILD_DATE=unknown

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
    -ldflags="-s -w \
      -X github.com/i-harsha-reddy/neo4j-exporter/internal/version.Version=${VERSION} \
      -X github.com/i-harsha-reddy/neo4j-exporter/internal/version.Revision=${REVISION} \
      -X github.com/i-harsha-reddy/neo4j-exporter/internal/version.Branch=${BRANCH} \
      -X github.com/i-harsha-reddy/neo4j-exporter/internal/version.BuildUser=${BUILD_USER} \
      -X github.com/i-harsha-reddy/neo4j-exporter/internal/version.BuildDate=${BUILD_DATE}" \
    -o /out/neo4j-exporter ./cmd/neo4j-exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/neo4j-exporter /usr/local/bin/neo4j-exporter
USER nonroot:nonroot
EXPOSE 9412
ENTRYPOINT ["/usr/local/bin/neo4j-exporter"]
CMD ["--config.file=/etc/neo4j-exporter/config.yml"]
