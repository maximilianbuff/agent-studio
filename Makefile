BINARY  := studio
PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
VERSION := $(shell git -C $(CURDIR) describe --tags --dirty --always 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: build build-all install uninstall clean test help

build: ## Compile the studio binary into ./studio
	cd cli && go build $(LDFLAGS) -o ../$(BINARY) .

install: build ## Build and install studio to $(BINDIR) [PREFIX=~/.local]
	install -d "$(BINDIR)"
	install -m 0755 $(BINARY) "$(BINDIR)/$(BINARY)"
	@echo "Installed $(BINDIR)/$(BINARY)"
	@echo "Make sure $(BINDIR) is on your PATH."

uninstall: ## Remove studio from $(BINDIR)
	rm -f "$(BINDIR)/$(BINARY)"
	@echo "Removed $(BINDIR)/$(BINARY)"

build-all: ## Cross-compile for all supported platforms into dist/
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux  GOARCH=amd64 go build $(LDFLAGS) -C cli -o ../dist/studio_linux_amd64
	CGO_ENABLED=0 GOOS=linux  GOARCH=arm64 go build $(LDFLAGS) -C cli -o ../dist/studio_linux_arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -C cli -o ../dist/studio_darwin_amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -C cli -o ../dist/studio_darwin_arm64
	cd dist && sha256sum studio_* > checksums.txt
	@echo "Built: $$(ls dist/studio_*)"

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf dist/

test: ## Run all tests
	cd cli && go test ./...

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'
