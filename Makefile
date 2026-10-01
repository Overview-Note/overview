# Overview - developer tasks
# Requires: Go 1.26+, Node 22+, Docker (optional)

BINARY := bin/overview
VERSION ?= 0.2.0-dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: help dev build build-web build-go test test-go test-web lint fmt vet clean docker

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

dev: ## Run backend (embedded frontend) on :5230
	go run ./cmd/overview

build: build-web build-go ## Build frontend then single binary

build-web: ## Build the Vue frontend into internal/webui/dist
	cd web && npm ci && npm run build

build-go: ## Compile the Go binary (expects frontend built)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/overview

test: test-go ## Run all tests

test-go: ## Run Go tests
	go test ./...

test-web: ## Type-check the frontend
	cd web && npm run lint

lint: ## Run linters (golangci-lint + frontend typecheck)
	golangci-lint run ./...
	cd web && npm run lint

fmt: ## Format Go and frontend sources
	gofmt -w .
	cd web && npm run format

vet: ## Run go vet
	go vet ./...

clean: ## Remove build artifacts
	rm -rf bin internal/webui/dist/assets

docker: ## Build the Docker image
	docker build -t overview:$(VERSION) .
