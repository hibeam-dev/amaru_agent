# Amaru Agent

Go reference implementation of the agent component for the Amaru
proxy system.

## Features

- Secure server connection with reliable reconnection
- TOML configuration with live reload
- Background execution support
- Internationalization (i18n) support
- Wireguard tunneling for local application proxying

## Installation

### Prerequisites

- Go 1.24 or later

### Building from source

```bash
git clone https://github.com/hibeam-dev/amaru_agent.git
cd amaru_agent
make build
```

The binary will be available in the `bin/` directory.

### Installation

```bash
# Install to $GOPATH/bin (requires GOPATH to be set)
make install

# Uninstall from system
make uninstall
```

## Development

```bash
# Clone and setup
git clone https://github.com/hibeam-dev/amaru_agent.git
cd amaru_agent

# Install linter (recommended)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install test mocking tool (recommended for development)
go install go.uber.org/mock/mockgen@latest

# Build and test
make build
make test

# Hot reload development
make dev                           # Uses default config.toml
make dev CONFIG=config.prod.toml   # Uses custom config
make dev CONFIG=config.staging.toml

# Show all available make targets
make help
```

### Project Structure

```
├── cmd/amaru/           # Main entry point
├── internal/            # Internal packages
│   ├── app/             # Application core
│   ├── config/          # Configuration
│   ├── daemon/          # Background process
│   ├── event/           # Event pub/sub system
│   ├── i18n/            # Internationalization
│   ├── protocol/        # Communication protocol
│   ├── registry/        # Transport registration
│   ├── service/         # Business logic
│   ├── ssh/             # SSH connection implementation
│   ├── transport/       # Connection interfaces
│   └── util/            # Shared utilities
├── Makefile             # Build automation
└── config.toml          # Default config
```

### Make Commands

```bash
make help       # Show all available targets with descriptions
make build      # Build application
make install    # Install to $GOPATH/bin
make uninstall  # Remove from $GOPATH/bin
make test       # Run tests
make cover      # Test coverage stats
make cover-html # Generate HTML coverage report
make lint       # Run linter
make fmt        # Format code
make mocks      # Generate interface mocks
make vendor     # Tidy and verify dependencies
make dev        # Hot reload development (requires CONFIG=file.toml)
make clean      # Clean artifacts
```

## Usage

```bash
# Basic usage
./bin/amaru -h

# Registration
./bin/amaru -register $token
./bin/amaru -register $token -hostname example.com
./bin/amaru -register $token -key /path/to/custom/key.pub

# With custom config
./bin/amaru -config=/path/to/config.toml

# Use Spanish language
LANG=es_ES.UTF-8 ./bin/amaru

# Generate SSH key pair
./bin/amaru -genkey > server_public_key.txt

# Run in background
nohup ./bin/amaru -pid > /dev/null 2>&1 &
```

### Command-line Options

- `-config`: Path to TOML configuration file (default: config.toml)
- `-genkey`: Generate SSH key pair and exit
- `-register`: Register public SSH key with backend server using token (default: app.amaru.cloud)
- `-key`: Custom SSH key file path for registration (default uses system default)
- `-hostname`: Custom domain for registration (only valid with -register)
- `-pid`: Write PID file to ~/.amaru/amaru.pid

### Signal Handling

- `SIGHUP`: Reload configuration
- `SIGINT`/`SIGTERM`: Graceful shutdown

## Configuration

```toml
[connection]
host = "localhost"       # Server hostname
port = 22000             # Server port
timeout = "30s"          # Connection timeout
keyfile = "/path/to/key" # Private key path (optional: defaults to OS-specific location)
                         # Windows: %LOCALAPPDATA%\amaru\amaru_agent.key
                         # macOS: ~/Library/Application Support/amaru/amaru_agent.key
                         # Linux: ~/.local/share/amaru/amaru_agent.key

[application]
port = 8080              # Local application port
hostname = "example.com" # Local hostname
ip = "127.0.0.1"         # IP address to connect to for local application

[application.tags]       # Custom metadata tags
service = "service-name"
version = "v1.2.3"
environment = "production"
region = "us-east-1"
tier = "backend"

[application.security]   # Security configuration
tls = false
mtls = false

[logging]
level = "info"           # Log level
logfile = "/path/to/log" # Log file path (optional: defaults to OS-specific location)
```

## Architecture

The agent maintains a persistent and reliable connection to the remote server:

- **Automatic reconnection**: Implements exponential backoff when connections fail
- **Connection health monitoring**: Actively checks connection health
- **Graceful recovery**: Handles network interruptions seamlessly
- **Wireguard tunneling**: Proxies traffic between local applications and remote services

### Design Principles

- Separation of concerns with focused packages
- Dependency injection via interfaces
- Context propagation for cancellation
- Proper resource cleanup
- Testable code with autogenerated mocks
- Small, focused interfaces

### Testing

This project uses [mockgen](https://github.com/uber-go/mock) from Uber
to generate mocks for interfaces.

To generate a mock for the current interfaces:

```bash
make mocks
```

## Dependencies

- `github.com/BurntSushi/toml` - TOML configuration
- `golang.org/x/crypto/ssh` - Connection functionality
- `golang.zx2c4.com/wireguard` - Tunneling capabilities
- `github.com/nicksnyder/go-i18n/v2` - Internationalization
- `golang.org/x/text` - Language detection
- `github.com/maniartech/signals` - Event handling system
- `go.uber.org/mock` - Mock generation for testing

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
