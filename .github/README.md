# GitHub Actions Workflows

This directory contains automated CI/CD workflows for the Shadowy blockchain project.

## Workflows

### 🔄 Continuous Integration (`ci.yml`)

**Triggers**: Push to main/develop branches, pull requests

**Jobs**:
- **Lint and Format Check**: Code quality validation with golangci-lint
- **Test Suite**: Unit tests across Go 1.20 and 1.21 with coverage reporting
- **Build Test**: Cross-platform build verification
- **Integration Tests**: End-to-end functionality testing
- **Security Scan**: Vulnerability scanning with Trivy and secret detection
- **Performance Benchmarks**: Automated performance testing

### 🚀 Build and Release (`build-release.yml`)

**Triggers**: Push to main, tagged releases, manual dispatch

**Features**:
- **Automated Versioning**: `0.1+BUILD_NUMBER` format
- **Secure Signing**: Cosign integration with Sigstore public registry
- **Multi-format Releases**: Binary, tar.gz, zip with checksums
- **SBOM Generation**: Software Bill of Materials with Syft
- **Container Images**: Docker builds pushed to GitHub Container Registry
- **Security Attestations**: Signed metadata and provenance

**Build Process**:
1. **Version Calculation**: `0.1+$(git rev-list --count HEAD)`
2. **Binary Build**: Linux x64 with embedded version info
3. **Checksum Generation**: SHA256, SHA512, MD5
4. **Cosign Signing**: Keyless signing with Sigstore
5. **SBOM Creation**: SPDX and CycloneDX formats
6. **Release Creation**: GitHub release with all artifacts
7. **Container Build**: Minimal scratch-based image
8. **Security Scanning**: Vulnerability assessment

## Security Features

### 🔐 Sigstore/Cosign Integration

All binaries are signed using [Sigstore](https://www.sigstore.dev/)'s keyless signing:

```bash
# Verify binary signature
cosign verify-blob \
  --bundle shadowy-linux-amd64.cosign.bundle \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  shadowy-linux-amd64
```

### 📋 Software Bill of Materials (SBOM)

Each release includes SBOM files in multiple formats:
- `shadowy-sbom.spdx.json` - SPDX format
- `shadowy-sbom.cyclonedx.json` - CycloneDX format

### 🛡️ Security Scanning

- **Trivy**: Vulnerability scanning for code and binaries
- **TruffleHog**: Secret detection in source code
- **Gosec**: Go-specific security analysis
- **Dependabot**: Automated dependency updates

## Version Scheme

**Format**: `0.1+BUILD_NUMBER`

**Examples**:
- `0.1+1` - First build
- `0.1+42` - Build number 42
- `0.1+123` - Build number 123

**Build Number**: Total commits in repository (`git rev-list --count HEAD`)

## Release Process

### Automatic Releases

1. **Push to main**: Creates pre-release with current build number
2. **Tagged release**: `git tag v0.1+123 && git push origin v0.1+123`
3. **Manual trigger**: Use GitHub Actions "Run workflow" button

### Manual Releases

Use the provided release script:

```bash
# Build with auto-calculated version
./scripts/release.sh

# Build with specific version and tag
./scripts/release.sh -v 0.2+456 -t -s

# Dry run to see what would happen
./scripts/release.sh --dry-run
```

## Artifacts

Each release includes:

### Binaries
- `shadowy-linux-amd64` - Main executable
- `shadowy-linux-amd64.tar.gz` - Compressed archive
- `shadowy-linux-amd64.zip` - ZIP archive

### Checksums
- `shadowy-linux-amd64.sha256` - SHA256 checksum
- `shadowy-linux-amd64.sha512` - SHA512 checksum
- `shadowy-linux-amd64.md5` - MD5 checksum

### Signatures
- `shadowy-linux-amd64.cosign.bundle` - Binary signature
- `shadowy-linux-amd64.sha256.cosign.bundle` - Checksum signature

### SBOM
- `shadowy-sbom.spdx.json` - SPDX Software Bill of Materials
- `shadowy-sbom.cyclonedx.json` - CycloneDX SBOM
- `*.cosign.bundle` - Signed SBOM files

### Container Images
- `ghcr.io/OWNER/REPO:latest` - Latest build
- `ghcr.io/OWNER/REPO:v0.1+123` - Tagged version
- `ghcr.io/OWNER/REPO:sha-abc1234` - Commit SHA

## Environment Variables

### Required for Releases
- `GITHUB_TOKEN` - Automatically provided by GitHub Actions
- `COSIGN_EXPERIMENTAL=1` - Enable keyless signing

### Optional Configuration
- `REGISTRY` - Container registry (default: ghcr.io)
- `IMAGE_NAME` - Container image name (default: repository name)

## Permissions

The workflows require these GitHub token permissions:

```yaml
permissions:
  contents: write      # Create releases
  packages: write      # Push container images
  id-token: write      # Sigstore/Cosign signing
  attestations: write  # Security attestations
```

## Dependencies

### Automated Updates

Dependabot is configured to automatically update:
- **Go modules**: Daily
- **GitHub Actions**: Weekly

### Manual Updates

Update major dependencies:

```bash
# Update Go version in workflows
sed -i 's/go-version: .*/go-version: "1.22"/' .github/workflows/*.yml

# Update action versions
# Check each action's repository for latest versions
```

## Troubleshooting

### Build Failures

1. **Go version mismatch**: Update `go-version` in workflows
2. **Dependency issues**: Check go.mod and run `go mod tidy`
3. **Test failures**: Run tests locally: `go test ./...`

### Signing Issues

1. **Cosign errors**: Ensure `COSIGN_EXPERIMENTAL=1` is set
2. **Permission denied**: Check `id-token: write` permission
3. **Network issues**: Sigstore services may be temporarily unavailable

### Release Problems

1. **Tag already exists**: Use a different version number
2. **Asset upload fails**: Check file sizes and GitHub limits
3. **Permission errors**: Verify `contents: write` permission

## Best Practices

1. **Test locally** before pushing
2. **Use descriptive commit messages** for better release notes
3. **Tag releases properly** for stable versions
4. **Monitor security alerts** from scans
5. **Keep dependencies updated** via Dependabot
6. **Verify signatures** after releases

## Support

For issues with the CI/CD pipeline:

1. Check the [Actions tab](../../actions) for detailed logs
2. Review this documentation
3. Check individual workflow files for configuration
4. Create an issue if problems persist