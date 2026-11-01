# Umbra Makefile — one command per promise in the README.
#
#   make demo    kind + audit log, images, CRDs, fixtures, UI build
#   make test    go build/vet/test + UI build
#   make bench   deterministic scoreboard row (no key, no cluster needed)
#   make lint    gofmt, go vet, eslint
#
# All recipes run on bash (CI, Linux/macOS, Git Bash on Windows).
# Versions are pinned in go.mod (Go), ui/package-lock.json (Node) and
# scripts/kind/create-cluster.sh (kindest/node image).

REPO_ROOT := $(shell git rev-parse --show-toplevel 2>/dev/null || echo .)
# kind lives outside PATH on Windows (AGENTS.md): %USERPROFILE%\go\bin.
# Git Bash (or WSL) is required to run the .sh recipes on Windows.
ifeq ($(OS),Windows_NT)
KIND_BIN ?= $(USERPROFILE)/go/bin/kind.exe
else
KIND_BIN ?= $(HOME)/go/bin/kind
endif

.PHONY: demo test bench lint ui-build

demo:
	bash scripts/demo/up.sh

test:
	go build ./...
	go vet ./...
	go test ./... -count=1
	$(MAKE) ui-build

bench:
	go run ./cmd/umbra-bench --mode deterministic

lint:
	test -z "$$(gofmt -l internal cmd)" || (gofmt -l internal cmd && exit 1)
	go vet ./...
	cd ui && npx eslint src/

ui-build:
	cd ui && npm ci --no-audit --no-fund && npm run build
