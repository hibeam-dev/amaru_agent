# Amaru Agent

Go reference implementation of the agent component for the Amaru reverse-TCP
proxy system.

## Features

- Secure server connection with reliable reconnection
- TOML configuration with live reload
- Background execution support
- Internationalization (i18n) support
- Automatic connection health monitoring
- TCP/TLS tunneling for local application proxying

## Installation

### Prerequisites

- Go 1.24 or later

### Building from source

```bash
git clone https://github.com/Erlang-Solutions/amaru_agent.git
cd amaru_agent
make build
```

The binary will be available in the `bin/` directory.

## Usage

```bash
# Basic usage
./bin/amaru_agent

# With custom config
./bin/amaru_agent -config=/path/to/config.toml

# Use Spanish language
LANG=es_ES.UTF-8 ./bin/amaru_agent

# Run in background
nohup ./bin/amaru_agent -pid > /dev/null 2>&1 &
```

### Command-line Options

- `-config`: Config file path (default: config.toml)
- `-pid`: Write PID to ~/.amaru_agent/amaru_agent.pid

### Signal Handling

- `SIGHUP`: Reload configuration
- `SIGINT`/`SIGTERM`: Graceful shutdown

## Configuration

```toml
[connection]
host = "localhost"       # Server hostname
port = 22000             # Server port
timeout = "30s"          # Connection timeout
keyfile = "/path/to/key" # Private key path
tunnel = true            # Enable TCP tunneling

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
logfile = "/path/to/log" # Log file path
```


## License

See the `LICENSE` file for details.
