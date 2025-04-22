# Cortex Agent

Cortex Agent is a Go-based SSH client that establishes a secure connection to a
remote server running a custom subsystem. It acts as a communication bridge
between local applications and a remote service, supporting JSON-based message
passing.

## Features

- SSH-based secure connection using key authentication
- Configurable via TOML configuration
- Can run in the background using standard Unix commands
- Support for dynamic configuration reloading via SIGHUP
- Graceful connection handling and reconnection capabilities
- JSON protocol support for integration with external applications

## Installation

### Prerequisites

- Go 1.24 or later

### Building from source

```bash
git clone https://github.com/Erlang-Solutions/cortex_agent.git
cd cortex_agent
make build
```

The build output will be in the `bin/` directory.

## Development Workflow

### Getting Started

To get started with development:

1. Clone the repository:
   ```bash
   git clone https://github.com/Erlang-Solutions/cortex_agent.git
   cd cortex_agent
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Install the linter (optional but recommended):
   ```bash
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

4. Build and test:
   ```bash
   make build
   make test
   ```

### Project Structure

```
├── cmd/
│   └── cortex_agent/       # Main application entry point
├── internal/
│   ├── app/                # Core application logic
│   ├── config/             # Configuration handling
│   ├── daemon/             # Background process functionality
│   ├── protocol/           # Communication protocol implementation
│   └── ssh/                # SSH connection management
├── pkg/
│   └── errors/             # Error types and utilities
├── Makefile                # Build automation
├── config.toml             # Default configuration
└── README.md               # Documentation
```

### Makefile Commands

The project includes a Makefile with several helpful commands:

```bash
# Build the application
make

# Run tests
make test

# Run tests with coverage statistics
make cover

# Generate HTML coverage report
make cover-html

# Format code
make fmt

# Run linter
make lint

# Clean build artifacts
make clean
```

## Usage

After building, you can run the agent:

```bash
# Run with custom config, otherwise config.toml must be in the same directory
./bin/cortex_agent -config=/path/to/config.toml

# Run in background using standard Unix commands
nohup ./bin/cortex_agent -pid > /dev/null 2>&1 &
```

The agent provides a clean separation between the command-line interface and the
application logic, making it easier to embed the core functionality in other
applications if needed.

### Command-line options

- `-json`: Enable JSON communication mode for application integration
- `-pid`: Write PID file to ~/.cortex_agent/cortex_agent.pid
- `-config`: Path to TOML configuration file (default: config.toml)

### Signal handling

The agent can be controlled with signals:
- `SIGHUP`: Reload configuration
- `SIGINT`/`SIGTERM`: Graceful shutdown

## Configuration

Configuration is handled via a TOML file with the following sections:

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

[agent]
[agent.tags]             # Custom metadata tags
service = "service-name"
version = "v1.2.3"
environment = "production"
region = "us-east-1"

[logging]
level = "info"           # Log level
logfile = "/path/to/log" # Log file path
```

## Architecture

The application has two primary operating modes:

1. **Standard mode**: Establishes and maintains a persistent SSH connection to
   the remote server
2. **JSON mode**: Facilitates JSON message exchange between a local application
   and the remote server

### Code Structure

The codebase is organized in layers:

- `cmd/cortex_agent`: Command-line interface and entry point
- `internal/app`: Core application logic and coordination
- `internal/config`: Configuration handling
- `internal/daemon`: PID file management
- `internal/protocol`: Protocol handlers (JSON and standard modes)
- `internal/ssh`: SSH connection management
- `pkg/errors`: Common error types and utilities

### Design Principles

The code follows these key design principles:

1. **Separation of concerns**: Each package has a specific responsibility
2. **Context propagation**: All long-running operations respect context
   cancellation
3. **Error handling**: Explicit error types and proper error wrapping
4. **Resource cleanup**: Proper resource management with defer statements
5. **Testability**: Code is structured to be testable with mock implementations

## Dependencies

- `github.com/BurntSushi/toml` - For parsing TOML configuration files
- `golang.org/x/crypto/ssh` - For SSH client functionality

## License

This project is licensed under the terms of the `LICENSE` file included in the
repository.

## Contributing

Contributions are welcome! Please follow these steps when contributing:

1. Fork the repository
2. Create a feature branch: `git checkout -b username/issue-number_your-feature-name`
3. Make your changes
4. Run tests: `make test`
5. Check test coverage: `make cover`
6. Run linter: `make lint`
7. Format code: `make fmt`
8. Commit your changes
9. Push to your branch: `git push origin username/issue-number_your-feature-name`
10. Create a Pull Request

### Code Quality Standards

Before submitting your code, please ensure:

- All tests pass with `make test`
- Code passes all linting checks with `make lint`
- All new code has appropriate test coverage (verify with `make cover`)
- Documentation is updated to reflect any changes
