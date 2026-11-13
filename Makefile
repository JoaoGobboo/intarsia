# Every recipe is a plain command that works in both sh and cmd.exe, so the
# same Makefile runs on Linux, macOS and Windows.

# Release builds pass the tag: make build VERSION=v0.1.0
VERSION ?= dev

BIN := bin/intarsia$(shell go env GOEXE)
WEB_DIST := cmd/intarsia/dist/index.html

# Pinned so that everyone, including CI, lints with the same version.
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

# Read by Vite, so the page shows the same version as `intarsia version`.
export VITE_APP_VERSION := $(VERSION)

.PHONY: build test lint dev dev-server dev-web demo

build: web/node_modules
	npm --prefix web run build
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o $(BIN) ./cmd/intarsia

# The Go code embeds the compiled interface, so it must exist before any Go
# command runs.
test: $(WEB_DIST)
	go test ./...
	npm --prefix web test

lint: $(WEB_DIST)
	$(GOLANGCI_LINT) run ./...
	npm --prefix web run typecheck
	npm --prefix web run lint

dev:
	$(MAKE) -j2 dev-server dev-web

dev-server: $(WEB_DIST)
	go run ./cmd/intarsia serve

dev-web: web/node_modules
	npm --prefix web run dev

# Starts the fictional company. Needs Docker; stop it with `intarsia demo down`.
demo: $(WEB_DIST)
	go run ./cmd/intarsia demo up

$(WEB_DIST): | web/node_modules
	npm --prefix web run build

web/node_modules: web/package-lock.json
	npm --prefix web ci
