BINARY := go-http-bench
BIN_DIR := bin

# Version bump level for release-version: major, minor or patch
BUMP ?= patch

.PHONY: all build test vet fmt format lint lint-fix clean release-version

all: lint test build

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

format: fmt

# Auto-fix what can be fixed (formatting, plus golangci-lint --fix if installed), then lint
lint-fix: fmt
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --fix ./...; \
	fi
	@$(MAKE) --no-print-directory lint

# Fails if any file is not gofmt-formatted, then runs vet (and golangci-lint if installed)
lint:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping"; \
	fi

clean:
	rm -f $(BIN_DIR)/$(BINARY)

# Bump the latest vX.Y.Z tag (BUMP=major|minor|patch, default patch), tag HEAD and push the tag.
# Usage: make release-version [BUMP=minor]
release-version: lint build
	@set -e; \
	case "$(BUMP)" in major|minor|patch) ;; *) echo "BUMP must be major, minor or patch"; exit 1;; esac; \
	git fetch --tags --quiet; \
	current="$$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"; \
	current="$${current:-v0.0.0}"; \
	ver="$${current#v}"; \
	major="$${ver%%.*}"; rest="$${ver#*.}"; minor="$${rest%%.*}"; patch="$${rest#*.}"; \
	case "$(BUMP)" in \
		major) major=$$((major + 1)); minor=0; patch=0;; \
		minor) minor=$$((minor + 1)); patch=0;; \
		patch) patch=$$((patch + 1));; \
	esac; \
	next="v$$major.$$minor.$$patch"; \
	echo "Releasing $$current -> $$next"; \
	git tag -a "$$next" -m "Release $$next"; \
	git push origin "$$next"
