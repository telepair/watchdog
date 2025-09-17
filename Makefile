.PHONY: all test bench tidy fmt vet staticcheck lint markdownlint clean help

# Variables
GOIMPORTS := $(shell go env GOPATH)/bin/goimports
STATICCHECK := $(shell go env GOPATH)/bin/staticcheck
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
GOLANGCI_LINT_VERSION=latest
COVERAGE_FILE := coverage.out
MARKDOWNLINT := $(shell which markdownlint 2>/dev/null)

# Default target
all: lint markdownlint test bench

# Run tests
test:
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

# Tidy and verify dependencies
tidy:
	@echo "🧹 Tidying and verifying dependencies..."
	go mod tidy
	go mod verify
	@echo "✅ Dependencies tidied and verified\n"

# Format code
fmt: tidy
	@echo "🎨 Formatting code with gofmt and goimports..."
	@echo "Running gofmt..."
	go fmt ./...

	@echo "Running goimports..."
	@if [ ! -x "$(GOIMPORTS)" ]; then \
		echo "📦 Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	fi
	$(GOIMPORTS) -l -w .
	@if [ -n "$$($(GOIMPORTS) -l .)" ]; then \
		echo "❌ Some files were reformatted. Please review the changes."; \
		echo "Reformatted files:"; \
		$(GOIMPORTS) -l .; \
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
	@if [ ! -x "$(STATICCHECK)" ]; then \
		echo "📦 Installing staticcheck..."; \
		go install honnef.co/go/tools/cmd/staticcheck@latest; \
	fi
	$(STATICCHECK) ./...
	@echo "✅ Staticcheck analysis completed\n"

# Run comprehensive linting
lint: fmt vet staticcheck 
	@echo "🔍 Running golangci-lint with comprehensive ruleset..."
	@if [ ! -x "$(GOLANGCI_LINT)" ]; then \
		echo "📦 Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	$(GOLANGCI_LINT) run --config .golangci.yml ./...
	@echo "✅ Linting completed\n"

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
	$(MARKDOWNLINT) .
	@echo "✅ Markdown linting completed\n"



# Clean build files
clean:
	@echo "🧹 Cleaning build artifacts ..."
	go clean ./...
	rm -f $(COVERAGE_FILE) coverage.html
	rm -f watchdog watchdog-agent
	rm -rf build/
	@echo "✅ Cleanup completed\n"

# Display help
help:
	@echo "Available targets:"
	@echo "  test        - Run tests"
	@echo "  bench       - Run benchmarks"
	@echo "  tidy        - Tidy and verify dependencies"	
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet static analysis"	
	@echo "  staticcheck - Run staticcheck analysis"	
	@echo "  lint        - Run comprehensive linting"
	@echo "  markdownlint - Run markdown linting"
	@echo "  clean       - Clean build artifacts"
	@echo "  help        - Display this help"