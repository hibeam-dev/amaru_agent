# Cortex Agent

A Go-based agent for `cortex` servers.

## Features

- SSH-based secure connection with key authentication
- TOML configuration with live reload (SIGHUP)
- Background execution support
- JSON protocol for application integration
- Internationalization (i18n) support

## Installation

### Prerequisites

- Go 1.24 or later

### Building from source

```bash
git clone https://github.com/Erlang-Solutions/cortex_agent.git
cd cortex_agent
make build
```

The binary will be available in the `bin/` directory.

## Development

```bash
# Clone and setup
git clone https://github.com/Erlang-Solutions/cortex_agent.git
cd cortex_agent
go mod download

# Install linter (recommended)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Build and test
make build
make test
```

### Project Structure

```
├── cmd/cortex_agent/    # Main entry point
├── internal/            # Internal packages
│   ├── app/             # Application core
│   ├── config/          # Configuration
│   ├── daemon/          # Background process
│   ├── i18n/            # Internationalization
│   ├── protocol/        # Communication protocol
│   ├── service/         # Business logic
│   └── ssh/             # SSH connection
├── pkg/                 # Public packages
├── Makefile             # Build automation
└── config.toml          # Default config
```

### Make Commands

```bash
make        # Build application
make test   # Run tests
make cover  # Test coverage stats
make lint   # Run linter
make fmt    # Format code
make clean  # Clean artifacts
```

## Usage

```bash
# Basic usage
./bin/cortex_agent

# With custom config
./bin/cortex_agent -config=/path/to/config.toml

# Use Spanish language
LANG=es_ES.UTF-8 ./bin/cortex_agent

# Run in background
nohup ./bin/cortex_agent -pid > /dev/null 2>&1 &
```

### Command-line Options

- `-config`: Config file path (default: config.toml)
- `-json`: Enable JSON communication mode
- `-pid`: Write PID to ~/.cortex_agent/cortex_agent.pid

### Signal Handling

- `SIGHUP`: Reload configuration
- `SIGINT`/`SIGTERM`: Graceful shutdown

## Configuration

```toml
[ssh]
host = "localhost"       # SSH server hostname
port = 22000             # SSH server port
user = "agent"           # SSH username
timeout = "30s"          # Connection timeout
keyfile = "/path/to/key" # SSH private key path

[application]
port = 8080              # Local application port
hostname = "example.com" # Local hostname

[agent.tags]             # Custom metadata tags
service = "service-name"
version = "v1.2.3"
environment = "production"

[logging]
level = "info"           # Log level
logfile = "/path/to/log" # Log file path
```

## Architecture

Two primary operating modes:

1. **Standard mode**: Maintains persistent SSH connection to remote server
2. **JSON mode**: Facilitates message exchange between local app and server

### Design Principles

- Separation of concerns with focused packages
- Context propagation for cancellation
- Strong error handling with Result type
- Proper resource cleanup
- Testable code with mocks
- Dependency injection
- Small, focused interfaces

## Dependencies

- `github.com/BurntSushi/toml` - TOML configuration
- `golang.org/x/crypto/ssh` - SSH client functionality
- `github.com/nicksnyder/go-i18n/v2` - Internationalization
- `golang.org/x/text` - Language detection

## Contributing

1. Fork repository
2. Create branch: `git checkout -b username/issue-number_feature-name`
3. Make changes
4. Test: `make test` and `make cover`
5. Lint: `make lint` and `make fmt`
6. Commit changes
7. Push: `git push origin username/issue-number_feature-name`
8. Create Pull Request

Before submitting, ensure:

- All tests pass
- Code passes linting
- New code has appropriate test coverage
- Documentation is up to date

## License

See the `LICENSE` file for details.
