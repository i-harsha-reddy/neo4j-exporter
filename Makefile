SHELL := /bin/bash

BIN          := neo4j-exporter
PKG          := github.com/i-harsha-reddy/neo4j-exporter
VERSION      ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
REVISION     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH       ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
BUILD_USER   ?= $(USER)
BUILD_DATE   ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Revision=$(REVISION) \
  -X $(PKG)/internal/version.Branch=$(BRANCH) \
  -X $(PKG)/internal/version.BuildUser=$(BUILD_USER) \
  -X $(PKG)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: all build test lint vet tidy fmt clean docker run e2e chart-lint chart-template chart-test chart-package

all: lint test build

build:
	go build -trimpath -ldflags='$(LDFLAGS)' -o $(BIN) ./cmd/neo4j-exporter

test:
	go test -race -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -s -w .

tidy:
	go mod tidy

lint:
	@which golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed; skipping"; exit 0; }
	golangci-lint run ./...

clean:
	rm -f $(BIN) coverage.out coverage.html
	rm -rf dist/

docker:
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  --build-arg REVISION=$(REVISION) \
	  --build-arg BRANCH=$(BRANCH) \
	  --build-arg BUILD_USER=$(BUILD_USER) \
	  --build-arg BUILD_DATE=$(BUILD_DATE) \
	  -t neo4j-exporter:$(VERSION) -t neo4j-exporter:latest .

run: build
	./$(BIN) --config.file=config/neo4j-exporter.minimal.yml --log.level=debug

e2e:
	bash scripts/e2e.sh

# --- Helm chart -------------------------------------------------------------

CHART_DIR := charts/neo4j-exporter

chart-lint:
	helm lint $(CHART_DIR) -f $(CHART_DIR)/ci/minimal-values.yaml
	helm lint $(CHART_DIR) -f $(CHART_DIR)/ci/full-values.yaml

chart-template:
	@mkdir -p dist/chart-render
	helm template foo $(CHART_DIR) -f $(CHART_DIR)/ci/minimal-values.yaml > dist/chart-render/minimal.yaml
	helm template foo $(CHART_DIR) -f $(CHART_DIR)/ci/full-values.yaml    > dist/chart-render/full.yaml
	@echo "Rendered: dist/chart-render/{minimal,full}.yaml"

chart-test: chart-lint chart-template
	@command -v kubeconform >/dev/null 2>&1 || { echo "kubeconform not installed; skipping schema validation"; exit 0; }
	@for f in dist/chart-render/*.yaml; do \
	  echo "==> $$f"; \
	  kubeconform -strict -summary -kubernetes-version 1.29.0 -schema-location default $$f; \
	done

chart-package:
	@mkdir -p dist
	helm package $(CHART_DIR) --destination dist
	@ls -1 dist/neo4j-exporter-*.tgz | tail -1
