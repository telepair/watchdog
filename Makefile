.PHONY: all test bench build build-watchdog build-agent tidy fmt vet staticcheck lint markdownlint clean help tools version

# Variables
GOIMPORTS := $(shell go env GOPATH)/bin/goimports
STATICCHECK := $(shell go env GOPATH)/bin/staticcheck
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION=latest
COVERAGE_FILE := coverage.out
MARKDOWNLINT := $(shell which markdownlint 2>/dev/null)

# Build variables (computed once)
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d ' ' -f 3)
LDFLAGS := -s -w -X 'github.com/telepair/watchdog/pkg/version.Version=$(VERSION)' -X 'github.com/telepair/watchdog/pkg/version.GitCommit=$(COMMIT)' -X 'github.com/telepair/watchdog/pkg/version.BuildDate=$(BUILD_DATE)'


# Tool installation helpers
define install_tool
	@if [ ! -x "$(1)" ]; then \
		echo "📦 Installing $(2)..."; \
		go install $(3); \
	fi
endef

# Default target
all: lint markdownlint test bench

# Run tests
test: lint
	@echo "🧪 Running Go tests with race detection..."
	go test -v -race -timeout=10m -cover -coverprofile=$(COVERAGE_FILE) ./...
	@echo ""
	@echo "📊 Coverage Summary:"
	go tool cover -func=coverage.out | tail -1
	@echo "✅ Tests completed successfully\n"

# Run benchmarks (stable single-core, repeated)
bench:
	@echo "🏋️ Running benchmarks with stable configuration..."
	GOMAXPROCS=1 go test -v -bench=. -benchmem -run=^$$ -count=3 ./...
	@echo "✅ Benchmarks completed\n"

# Build both binaries
build: build-watchdog build-agent

# Build watchdog binary
build-watchdog: lint test
	@echo "🔨 Building watchdog binary..."
	@echo "Version: $(VERSION), Commit: $(COMMIT)"
	@mkdir -p build
	go build -ldflags="$(LDFLAGS)" -o build/watchdog ./cmd/watchdog
	@echo "✅ Build completed: build/watchdog\n"

# Build watchdog-agent binary
build-agent: lint test
	@echo "🔨 Building watchdog-agent binary..."
	@echo "Version: $(VERSION), Commit: $(COMMIT)"
	@mkdir -p build
	go build -ldflags="$(LDFLAGS)" -o build/watchdog-agent ./cmd/watchdog-agent
	@echo "✅ Build completed: build/watchdog-agent\n"


# Run comprehensive linting
lint: fmt vet staticcheck markdownlint
	@echo "🔍 Running golangci-lint with comprehensive ruleset..."
	$(call install_tool,$(GOLANGCI_LINT),golangci-lint,github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION))
	$(GOLANGCI_LINT) run --config .golangci.yml ./...
	@echo "✅ Linting completed\n"

# Tidy and verify dependencies
tidy:
	@echo "🧹 Tidying and verifying dependencies..."
	go mod tidy
	go mod verify
	@echo "✅ Dependencies tidied and verified\n"

# Install development tools
tools:
	@echo "📦 Installing development tools..."
	$(call install_tool,$(GOIMPORTS),goimports,golang.org/x/tools/cmd/goimports@latest)
	$(call install_tool,$(STATICCHECK),staticcheck,honnef.co/go/tools/cmd/staticcheck@latest)
	$(call install_tool,$(GOLANGCI_LINT),golangci-lint,github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION))
	@echo "✅ Tools installation completed\n"

# Format code
fmt: tidy
	@echo "🎨 Formatting code with gofmt and goimports..."
	@echo "Running gofmt..."
	go fmt ./...

	@echo "Running goimports..."
	$(call install_tool,$(GOIMPORTS),goimports,golang.org/x/tools/cmd/goimports@latest)
	$(GOIMPORTS) -l -w .
	@if [ -n "$$($(GOIMPORTS) -l .)" ]; then \
		echo "❌ Some files were reformatted. Please review the changes."; \
		echo "Reformatted files:"; \
		$(GOIMPORTS) -l .; \
		exit 1; \
	else \
		echo "✅ All files are properly formatted\n"; \
	fi

# Run static analysis with go vet
vet:
	@echo "🔍 Running go vet static analysis..."
	go vet ./...
	@echo "✅ Static analysis completed\n"

# Run staticcheck analysis
staticcheck:
	@echo "🔍 Running staticcheck analysis..."
	$(call install_tool,$(STATICCHECK),staticcheck,honnef.co/go/tools/cmd/staticcheck@latest)
	$(STATICCHECK) ./...
	@echo "✅ Staticcheck analysis completed\n"

# Run markdown linting
markdownlint:
	@echo "📝 Running markdownlint on documentation..."
	@if [ -z "$(MARKDOWNLINT)" ]; then \
		echo "❌ markdownlint not found. Please install it with:"; \
		echo "   npm install -g markdownlint-cli"; \
		echo "   or"; \
		echo "   yarn global add markdownlint-cli"; \
		exit 1; \
	fi
	$(MARKDOWNLINT) . || true
	@echo "✅ Markdown linting completed\n"

# Display version information
version:
	@echo "Build Information:"
	@echo "  Version: $(VERSION)"
	@echo "  Commit: $(COMMIT)"
	@echo "  Build Date: $(BUILD_DATE)"
	@echo "  Go Version: $(GO_VERSION)"
	@echo ""

# Clean build files
clean:
	@echo "🧹 Cleaning build artifacts..."
	go clean ./...
	rm -f $(COVERAGE_FILE) coverage.html
	rm -f watchdog watchdog-agent watchdog-*
	rm -rf build/
	@echo "✅ Cleanup completed\n"

# Display help
help:
	@echo "Available targets:"
	@echo "  all           - Run lint, markdownlint, test, and bench"
	@echo "  test          - Run tests with race detection and coverage"
	@echo "  bench         - Run benchmarks"
	@echo "  build         - Build both watchdog and watchdog-agent binaries"
	@echo "  build-watchdog - Build watchdog binary only"
	@echo "  build-agent   - Build watchdog-agent binary only"
	@echo "  tidy          - Tidy and verify dependencies"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet static analysis"
	@echo "  staticcheck   - Run staticcheck analysis"
	@echo "  lint          - Run comprehensive linting"
	@echo "  markdownlint  - Run markdown linting"
	@echo "  tools         - Install development tools"
	@echo "  version       - Display build information"
	@echo "  clean         - Clean build artifacts"
	@echo "  help          - Display this help"