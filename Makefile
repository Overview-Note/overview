# Overview - developer tasks
# Requires: Go 1.26+, Node 22+, Docker (optional)

BINARY := bin/overview
DESKTOP_BINARY := bin/overview-desktop
EXE := $(shell go env GOEXE)
VERSION ?= 0.14.0
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: help dev demo build build-web build-go build-desktop package-desktop test test-go test-web lint fmt vet clean docker site site-export

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

dev: ## Run backend (embedded frontend) on :5230
	go run ./cmd/overview

demo: ## Run the sample vault in demo/ on :5230 (first visit creates an admin)
	OVERVIEW_DATA_DIR=./demo go run ./cmd/overview

build: build-web build-go ## Build frontend then single binary

build-web: ## Build the Vue frontend into internal/webui/dist
	cd web && npm ci && npm run build

build-go: ## Compile the Go binary (expects frontend built)
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/overview

# The desktop shell is built with a plain `go build` (Windows/macOS build from
# the default tags; Linux also needs -tags desktop plus GTK/WebKitGTK headers).
# This is the supported fallback for the Wails v3 packager (`wails3 package`),
# which expects the generated Taskfile/build-assets tree; see docs/DESIGN.md
# ADR-059. The installers (NSIS/DMG/AppImage) are produced by wails3 from the
# same build/config.yml once those assets exist.
build-desktop: build-web ## Build the desktop shell for the host platform (frontend + shell)
	go build -trimpath -tags desktop -ldflags="$(LDFLAGS)" -o $(DESKTOP_BINARY)$(EXE) ./cmd/overview-desktop

package-desktop: build-desktop ## Archive the desktop binary for the host platform
	cd bin && tar -czf overview-desktop-$(VERSION)-$(shell go env GOOS)-$(shell go env GOARCH).tar.gz overview-desktop$(EXE)

test: test-go test-web ## Run all tests (Go + frontend)

test-go: ## Run Go tests
	go test ./...

test-web: ## Type-check and unit-test the frontend
	cd web && npm run lint && npm test

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

site: ## Run the documentation site (site/) in read-only render mode on :5230
	OVERVIEW_RENDER=true OVERVIEW_SITE_TITLE="Overview Docs" OVERVIEW_DATA_DIR=./site go run ./cmd/overview

site-export: ## Export the documentation site to a static site in _site/
	OVERVIEW_DATA_DIR=./site OVERVIEW_SITE_TITLE="Overview Docs" OVERVIEW_EXPORT_DIR=./_site OVERVIEW_EXPORT_BASE=/ go run ./cmd/overview export

