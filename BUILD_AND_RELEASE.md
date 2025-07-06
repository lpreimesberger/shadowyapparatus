# Shadowy Blockchain - Build and Release Guide

This document describes the automated build, signing, and release process for the Shadowy blockchain project.

## 🎯 Overview

The project uses GitHub Actions for:
- **Continuous Integration**: Automated testing, linting, and security scanning
- **Automated Releases**: Building, signing, and publishing releases
- **Security**: Cosign signing with Sigstore public registry
- **Monitoring**: Dependency updates and vulnerability scanning

## 📦 Release Artifacts

Each release produces the following artifacts:

### Binary Releases
- `shadowy-linux-amd64` - Main executable for Linux x64
- `shadowy-linux-amd64.tar.gz` - Compressed tarball
- `shadowy-linux-amd64.zip` - ZIP archive

### Security Artifacts
- `shadowy-linux-amd64.sha256` - SHA256 checksum
- `shadowy-linux-amd64.sha512` - SHA512 checksum  
- `shadowy-linux-amd64.md5` - MD5 checksum
- `shadowy-linux-amd64.cosign.bundle` - Cosign signature bundle
- `shadowy-linux-amd64.sha256.cosign.bundle` - Signed checksum

### Software Bill of Materials (SBOM)
- `shadowy-sbom.spdx.json` - SPDX format SBOM
- `shadowy-sbom.cyclonedx.json` - CycloneDX format SBOM
- `*.cosign.bundle` - Signed SBOM files

### Container Images
- `ghcr.io/OWNER/REPO:latest` - Latest development build
- `ghcr.io/OWNER/REPO:v0.1+123` - Tagged release
- `ghcr.io/OWNER/REPO:sha-abc1234` - Commit-specific build

## 🔢 Versioning Scheme

**Format**: `0.1+BUILD_NUMBER`

Where:
- **Base Version**: `0.1` (current major.minor)
- **Build Number**: Total commits in repository
- **Examples**: `0.1+1`, `0.1+42`, `0.1+123`

### Version Calculation
```bash
BASE_VERSION="0.1"
BUILD_NUMBER=$(git rev-list --count HEAD)
FULL_VERSION="${BASE_VERSION}+${BUILD_NUMBER}"
```

## 🔐 Security Features

### Sigstore/Cosign Signing

All binaries are signed using keyless signing with the public Sigstore registry:

**Benefits**:
- No private key management required
- Transparency log for all signatures
- OIDC-based identity verification
- Public verification without secrets

**Verification**:
```bash
# Install cosign
go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# Verify binary signature
cosign verify-blob \
  --bundle shadowy-linux-amd64.cosign.bundle \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  shadowy-linux-amd64
```

### SBOM Generation

Software Bill of Materials (SBOM) files provide complete dependency transparency:

```bash
# Generate SBOM locally
syft packages . -o spdx-json=shadowy-sbom.spdx.json
syft packages . -o cyclonedx-json=shadowy-sbom.cyclonedx.json
```

## 🚀 Release Process

### Automatic Releases

1. **Development Releases** (pre-release):
   - Triggered on push to `main` branch
   - Version: `0.1+BUILD_NUMBER`
   - Marked as pre-release

2. **Tagged Releases** (stable):
   - Create and push a git tag: `git tag v0.1+123 && git push origin v0.1+123`
   - Marked as latest release

3. **Manual Releases**:
   - Use GitHub Actions "Run workflow" button
   - Option to force release creation

### Manual Release Script

Use the provided release script for local testing:

```bash
# Basic build
./scripts/release.sh

# Build with specific version
./scripts/release.sh -v 0.2+456

# Build, tag, and sign
./scripts/release.sh -t -s

# Dry run (show what would happen)
./scripts/release.sh --dry-run

# See all options
./scripts/release.sh --help
```

## 🔧 Build Process Details

### 1. Version Calculation
```bash
# Calculate build number
BUILD_NUMBER=$(git rev-list --count HEAD)
VERSION="0.1+${BUILD_NUMBER}"
COMMIT_SHA=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
```

### 2. Binary Build
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=${VERSION} -X main.GitCommit=${COMMIT_SHA} -X main.BuildTime=${BUILD_TIME}" \
  -o shadowy-linux-amd64 \
  .
```

### 3. Checksum Generation
```bash
sha256sum shadowy-linux-amd64 > shadowy-linux-amd64.sha256
sha512sum shadowy-linux-amd64 > shadowy-linux-amd64.sha512
md5sum shadowy-linux-amd64 > shadowy-linux-amd64.md5
```

### 4. Cosign Signing
```bash
# Sign binary
COSIGN_EXPERIMENTAL=1 cosign sign-blob \
  --bundle shadowy-linux-amd64.cosign.bundle \
  shadowy-linux-amd64

# Sign checksum
COSIGN_EXPERIMENTAL=1 cosign sign-blob \
  --bundle shadowy-linux-amd64.sha256.cosign.bundle \
  shadowy-linux-amd64.sha256
```

### 5. SBOM Creation
```bash
# Generate SBOM
syft packages . -o spdx-json=shadowy-sbom.spdx.json
syft packages . -o cyclonedx-json=shadowy-sbom.cyclonedx.json

# Sign SBOM
COSIGN_EXPERIMENTAL=1 cosign sign-blob \
  --bundle shadowy-sbom.spdx.json.cosign.bundle \
  shadowy-sbom.spdx.json
```

## 🧪 Quality Assurance

### Continuous Integration Pipeline

1. **Linting**: golangci-lint with comprehensive rule set
2. **Testing**: Unit tests with coverage reporting
3. **Security Scanning**: Trivy vulnerability scanner
4. **Secret Detection**: TruffleHog for exposed secrets
5. **Build Verification**: Cross-platform build testing
6. **Integration Testing**: End-to-end functionality testing

### Security Scanning

- **Static Analysis**: gosec for Go-specific security issues
- **Dependency Scanning**: Trivy for known vulnerabilities
- **Secret Detection**: TruffleHog for exposed credentials
- **Container Scanning**: Trivy for container vulnerabilities

## 📥 Installation Instructions

### Download and Verify

```bash
# Download latest release
LATEST_VERSION=$(curl -s https://api.github.com/repos/OWNER/REPO/releases/latest | jq -r .tag_name)
wget https://github.com/OWNER/REPO/releases/download/${LATEST_VERSION}/shadowy-linux-amd64

# Download checksum
wget https://github.com/OWNER/REPO/releases/download/${LATEST_VERSION}/shadowy-linux-amd64.sha256

# Verify checksum
sha256sum -c shadowy-linux-amd64.sha256

# Make executable
chmod +x shadowy-linux-amd64
```

### Verify Signature (Optional but Recommended)

```bash
# Download signature bundle
wget https://github.com/OWNER/REPO/releases/download/${LATEST_VERSION}/shadowy-linux-amd64.cosign.bundle

# Install cosign
go install github.com/sigstore/cosign/v2/cmd/cosign@latest

# Verify signature
cosign verify-blob \
  --bundle shadowy-linux-amd64.cosign.bundle \
  --certificate-identity-regexp ".*" \
  --certificate-oidc-issuer-regexp ".*" \
  shadowy-linux-amd64
```

### Container Usage

```bash
# Pull container image
docker pull ghcr.io/OWNER/REPO:latest

# Run container
docker run -p 8080:8080 -p 8888:8888 ghcr.io/OWNER/REPO:latest

# Run with custom configuration
docker run -v $(pwd)/config:/config ghcr.io/OWNER/REPO:latest node --config /config/node.yaml
```

## 🔄 Maintenance

### Updating Dependencies

Dependencies are automatically updated via Dependabot:
- **Go modules**: Daily checks
- **GitHub Actions**: Weekly checks

### Manual Updates

```bash
# Update Go version in workflows
find .github/workflows -name "*.yml" -exec sed -i 's/go-version: .*/go-version: "1.22"/' {} \;

# Update golangci-lint version
sed -i 's/golangci-lint-action@.*/golangci-lint-action@v4/' .github/workflows/ci.yml

# Update action versions
# Check each action's repository for latest versions
```

### Security Updates

1. **Monitor GitHub Security Advisories**
2. **Review Dependabot PRs promptly**
3. **Update base images for containers**
4. **Regenerate SBOMs after updates**

## 🐛 Troubleshooting

### Common Build Issues

**Go Module Issues**:
```bash
go mod tidy
go mod verify
```

**Version Conflicts**:
- Check Go version in workflows matches local development
- Ensure all workflows use the same Go version

**Checksum Mismatches**:
- Verify file integrity
- Check for line ending differences (Windows/Unix)

### Signing Issues

**Cosign Errors**:
- Ensure `COSIGN_EXPERIMENTAL=1` environment variable is set
- Check internet connectivity to Sigstore services
- Verify GitHub Actions has `id-token: write` permission

**Certificate Issues**:
- Sigstore certificates are short-lived (valid for 10 minutes)
- Re-run the signing process if certificates expire

### Release Issues

**Tag Conflicts**:
```bash
# Delete local tag
git tag -d v0.1+123

# Delete remote tag
git push --delete origin v0.1+123

# Create new tag
git tag v0.1+124
git push origin v0.1+124
```

**Asset Upload Failures**:
- Check file sizes (GitHub has limits)
- Verify network connectivity
- Check repository permissions

## 📞 Support

For issues with the build and release process:

1. **Check GitHub Actions logs** in the Actions tab
2. **Review workflow files** in `.github/workflows/`
3. **Test locally** using the release script
4. **Check dependencies** and update if needed
5. **Create an issue** if problems persist

## 🎉 Success Indicators

A successful release should have:

- ✅ All CI checks passing
- ✅ Binary builds successfully
- ✅ All checksums generated
- ✅ Cosign signatures created and verified
- ✅ SBOM files generated and signed
- ✅ GitHub release created with all artifacts
- ✅ Container image built and pushed
- ✅ No security vulnerabilities detected

## 🔮 Future Enhancements

Planned improvements:
- **Multi-architecture builds** (ARM64, Windows)
- **Release automation** via conventional commits
- **Performance regression testing**
- **Deployment automation**
- **Release notes generation** from commit history