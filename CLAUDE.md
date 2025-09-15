# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working
with code in this repository.

## Project Architecture

This is a Go CLI application built using Cobra for command-line interface
and Viper for configuration management. The project follows a modular design:

- `main.go`: Entry point that delegates to the cmd package
- `cmd/root.go`: Root command configuration with Cobra, handles CLI setup
  and configuration file management
- Configuration management via Viper with support for `.watchdog.yaml` files
  in home directory

## Development Commands

### Testing

```bash
# Run all tests with race detection and coverage
make test
# Or directly:
go test -v -race -timeout=10m -cover -coverprofile=coverage.out ./...

# View coverage report
go tool cover -func=coverage.out | tail -1
```

### Code Quality

```bash
# Run comprehensive linting and formatting
make lint

# Individual commands:
make fmt        # Format code with gofmt and goimports
make vet        # Run go vet static analysis
make staticcheck # Run staticcheck analysis
```

### Building

```bash
# Build for current platform
go build -o watchdog .

# Build for multiple platforms (as done in CI)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
  -o build/watchdog-linux-amd64 .
```

### Dependency Management

```bash
make tidy       # Tidy and verify dependencies
```

### Benchmarking

```bash
make bench      # Run benchmarks with stable single-core configuration
```

## Code Style and Standards

- Uses Go 1.25+ with strict golangci-lint configuration
- Import grouping: standard library → third-party → local packages
  (github.com/telepair/watchdog)
- Maximum line length: 120 characters
- Comprehensive linter rules defined in `.golangci.yml`
- All code must pass race detection tests

## CI/CD Pipeline

The project uses GitHub Actions with three main jobs:

- **test**: Runs tests with race detection and coverage reporting
- **lint**: Validates code formatting, imports, and runs comprehensive linting
- **build**: Cross-compiles for linux/darwin/windows on amd64/arm64

Quality gates require all tests to pass and code to meet strict linting
standards before merge.
