# Development

## Setting up a developer environment

### Download Go

Download the latest version of Go, for package options you may review the go website: https://go.dev/dl/.

### Project dependencies

Clone the repository.

```bash
git clone https://github.com/Erlang-Solutions/amaru_agent.git
cd amaru_agent
```

Download go package dependencies at the root of `amaru_agent`.

```bash
go mod download
```

Install optional tooling as self contained packages. 

```bash
# Install linter (recommended)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install test mocking generator (recommended for development)
go install go.uber.org/mock/mockgen@latest
```

## Available make commands

These are the available commands for building, testing and operating on the project files.

```bash
make        # Build application
make test   # Run tests
make cover  # Test coverage stats
make lint   # Run linter
make fmt    # Format code
make clean  # Clean artifacts
```

## Testing

This project uses [mockgen](https://github.com/uber-go/mock) from Uber
to generate mocks for interfaces.

To generate a mock for the current interfaces:

```bash
make mocks
```

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
