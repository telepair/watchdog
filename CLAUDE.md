# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working
with code in this repository.

## Project Architecture

This is a distributed monitoring system with two main binaries:

### Binary Architecture
- **watchdog**: Main server binary that can run standalone or with embedded agent
- **watchdog-agent**: Standalone agent binary for remote deployment

### Core Components
- **Server** (`internal/server/`): Main server orchestrating NATS infrastructure, health checks, and embedded agent
- **Agent** (`internal/agent/`): Monitoring agent with collector and executor components
  - **Collector** (`collector/`): System metrics collection and reporting via NATS
  - **Executor** (`executor/`): Command execution engine with NATS messaging
- **Configuration** (`internal/config/`): Unified config management for server, agent, and NATS storage
- **NATS Integration** (`pkg/natsx/`): Custom NATS client with JetStream KV and embedded server support
- **Common Packages** (`pkg/`): Reusable components (health, logger, shutdown, version)

### Key Technologies
- **NATS JetStream**: Message streaming and KV storage for agent communication
- **Cobra + Viper**: CLI framework with YAML configuration support
- **Prometheus**: Metrics collection and system monitoring via gopsutil
- **Structured Logging**: slog-based component logging

## Development Commands

### Testing

```bash
# Run all tests with race detection and coverage
make test
# Or directly:
go test -v -race -timeout=10m -cover -coverprofile=coverage.out ./...

# Run single test or package
go test -v -race ./internal/agent/
go test -v -race -run TestAgentStart ./internal/agent/

# View coverage report
go tool cover -func=coverage.out | tail -1
go tool cover -html=coverage.out  # Open in browser
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

### Local Development

```bash
# Run watchdog server with embedded NATS and agent
go run ./cmd/watchdog start

# Run standalone agent (requires external NATS)
go run ./cmd/watchdog-agent start

# Show version information
go run ./cmd/watchdog version
go run ./cmd/watchdog-agent version

# Generate default configuration
go run ./cmd/watchdog config generate
go run ./cmd/watchdog-agent config generate
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

## Development Notes

### NATS Infrastructure
- JetStream KV buckets are used for agent state storage
- JetStream streams handle agent communication and command execution
- Embedded NATS server can be enabled via `server.enable_embed_nats` config
- NATS client includes health checks and automatic reconnection

### Agent Architecture
- Agents report system metrics via collector components
- Command execution is handled through executor components with NATS messaging
- Agent lifecycle events (startup/shutdown) are published to streams
- All agent communication goes through JetStream for reliability

### Configuration Management
- YAML-based configuration with extensive validation
- Server and agent configs are unified but can run independently
- Default configs can be generated via CLI commands
- Configuration supports environment variable substitution
