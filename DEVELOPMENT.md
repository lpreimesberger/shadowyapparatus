# Development Guide

This guide covers the development workflow, tools, and best practices for the Shadowy blockchain project.

## 🚀 Quick Start

### Prerequisites

- **Go 1.21+**: [Install Go](https://golang.org/doc/install)
- **Git**: For version control
- **golangci-lint**: For code quality (optional for development)

### Setup

```bash
# Clone the repository
git clone https://github.com/USER/shadowyapparatus.git
cd shadowyapparatus

# Install dependencies
go mod tidy

# Build the project
go build -o shadowy .

# Run tests
go test ./...

# Run the node
./shadowy node
```

## 🔧 Development Tools

### Code Quality

**golangci-lint** is used for code quality checks:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1

# Run linting
golangci-lint run

# Or use the provided script
./scripts/lint.sh
```

**Configuration**: See `.golangci.yml` for linting rules and exclusions.

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific tests
go test ./cmd -v

# Run benchmarks
go test -bench=. ./...
```

### Building

```bash
# Development build
go build -o shadowy .

# Production build with version info
./scripts/release.sh

# Cross-platform build
GOOS=linux GOARCH=amd64 go build -o shadowy-linux-amd64 .
```

## 📁 Project Structure

```
shadowyapparatus/
├── cmd/                    # Command-line interface and core packages
│   ├── blockchain.go       # Blockchain core
│   ├── consensus.go        # P2P consensus engine
│   ├── miner.go           # Mining system
│   ├── mempool.go         # Transaction pool
│   ├── wallet.go          # Wallet management
│   ├── farming.go         # Plot farming
│   ├── node.go            # Node orchestration
│   └── version.go         # Version information
├── scripts/               # Development and deployment scripts
│   ├── lint.sh           # Local linting
│   ├── release.sh        # Release automation
│   ├── health.sh         # API health checks
│   └── test_multinode.sh # Multi-node testing
├── .github/              # GitHub Actions workflows
│   ├── workflows/        # CI/CD pipelines
│   └── dependabot.yml   # Dependency automation
├── main.go              # Application entry point
├── go.mod               # Go module definition
└── .golangci.yml        # Linting configuration
```

## 🔄 Development Workflow

### 1. Feature Development

```bash
# Create feature branch
git checkout -b feature/my-feature

# Make changes
# ... code changes ...

# Run quality checks
./scripts/lint.sh
go test ./...

# Commit changes
git add .
git commit -m "feat: add new feature"

# Push and create PR
git push origin feature/my-feature
```

### 2. Code Quality

Before committing, ensure:

- [ ] Code passes linting: `./scripts/lint.sh`
- [ ] All tests pass: `go test ./...`
- [ ] Code is formatted: `gofmt -w .`
- [ ] No security issues: `gosec ./...` (if installed)

### 3. Testing

**Unit Tests**:
```bash
go test ./cmd -v
```

**Integration Tests**:
```bash
# Start a test node
./shadowy node &
NODE_PID=$!

# Run API tests
./scripts/health.sh
./scripts/blockchain.sh

# Stop test node
kill $NODE_PID
```

**Multi-node Tests**:
```bash
./test_multinode.sh
```

## 🧪 Testing Strategy

### Unit Tests
- Test individual functions and methods
- Mock external dependencies
- Focus on business logic

### Integration Tests
- Test API endpoints
- Test multi-node communication
- Test mining and consensus

### End-to-End Tests
- Test complete user workflows
- Test deployment scenarios
- Test upgrade paths

## 📋 Code Standards

### Go Style Guide

Follow the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments):

- Use `gofmt` for formatting
- Write clear, descriptive function names
- Add comments for exported functions
- Keep functions small and focused
- Use meaningful variable names

### Error Handling

```go
// Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process block: %w", err)
}

// Good: Handle specific error types
if errors.Is(err, ErrBlockNotFound) {
    // Handle specific case
}
```

### Logging

```go
// Use structured logging
log.Printf("Block mined: height=%d, hash=%s", height, hash)

// Include context in error logs
log.Printf("Failed to connect to peer %s: %v", peerID, err)
```

## 🔐 Security Considerations

### Development Security

- **Never commit secrets** (use `.gitignore`)
- **Validate all inputs** from external sources
- **Use crypto/rand** for cryptographic randomness
- **Sanitize file paths** and user inputs

### Production Security

- **Enable all security checks** in CI/CD
- **Keep dependencies updated** (automated via Dependabot)
- **Sign all releases** with Cosign
- **Scan for vulnerabilities** regularly

## 🚀 Release Process

### Development Releases

Automatic releases are created on every push to `main`:

1. Version is calculated as `0.1+BUILD_NUMBER`
2. Binary is built and signed
3. Release is created on GitHub

### Tagged Releases

For stable releases:

```bash
# Calculate next version
BUILD_NUMBER=$(git rev-list --count HEAD)
VERSION="0.1+${BUILD_NUMBER}"

# Create and push tag
git tag "v${VERSION}"
git push origin "v${VERSION}"
```

### Manual Releases

Use the release script:

```bash
# Build and test locally
./scripts/release.sh --dry-run

# Create release with signing
./scripts/release.sh -t -s
```

## 🐛 Debugging

### Common Issues

**Build Failures**:
```bash
# Clean module cache
go clean -modcache
go mod download
```

**Test Failures**:
```bash
# Run tests with verbose output
go test -v ./...

# Run specific test
go test -run TestSpecificFunction ./cmd
```

**Linting Issues**:
```bash
# See what would be fixed
golangci-lint run --fix

# Fix formatting
gofmt -w .
```

### Logging and Monitoring

**Enable Debug Logging**:
```bash
./shadowy node --log-level debug
```

**Monitor Node Health**:
```bash
# Check health endpoint
curl http://localhost:8080/api/v1/health

# Monitor continuously
watch -n 5 ./scripts/health.sh
```

### Development Utilities

**API Testing**:
```bash
# Test all endpoints
./scripts/test_all.sh

# Test specific service
./scripts/mining.sh
./scripts/consensus.sh
```

**Performance Profiling**:
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./...

# Memory profiling
go test -memprofile=mem.prof -bench=. ./...

# View profiles
go tool pprof cpu.prof
```

## 📚 Documentation

### API Documentation

API endpoints are documented inline and tested via scripts:

- **Health**: `./scripts/health.sh`
- **Blockchain**: `./scripts/blockchain.sh`
- **Mining**: `./scripts/mining.sh`
- **Consensus**: `./scripts/consensus.sh`

### Code Documentation

- Use `godoc` to generate documentation: `godoc -http=:6060`
- Write package-level documentation
- Document complex algorithms
- Include examples for public APIs

## 🤝 Contributing

### Pull Request Process

1. **Fork the repository**
2. **Create a feature branch**
3. **Make your changes**
4. **Add tests** for new functionality
5. **Update documentation** if needed
6. **Ensure CI passes**
7. **Submit pull request**

### Code Review Checklist

- [ ] Code follows style guidelines
- [ ] Tests are included and passing
- [ ] Documentation is updated
- [ ] Security considerations addressed
- [ ] Performance impact considered
- [ ] Backward compatibility maintained

## 📞 Support

### Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and community discussions
- **Documentation**: Check existing docs first

### Development Environment Issues

1. **Check Go version**: `go version` (should be 1.21+)
2. **Update dependencies**: `go mod tidy`
3. **Clear caches**: `go clean -cache -modcache`
4. **Check environment**: `go env`

## 🔮 Roadmap

### Upcoming Features

- [ ] Multi-architecture builds (ARM64, Windows)
- [ ] Enhanced monitoring and metrics
- [ ] Configuration management improvements
- [ ] Performance optimizations
- [ ] Extended API functionality

### Long-term Goals

- [ ] Formal security audit
- [ ] Comprehensive benchmarking
- [ ] Production deployment guides
- [ ] Advanced consensus mechanisms
- [ ] Ecosystem tools and integrations