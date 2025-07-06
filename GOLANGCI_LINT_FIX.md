# golangci-lint Configuration Fix

## Issue

The GitHub Actions workflow was failing with:
```
Error: can't load config: unsupported version of the configuration
```

## Root Cause

The `.golangci.yml` configuration file was using deprecated linters and outdated configuration format that is not compatible with golangci-lint v1.55+.

## Solution

### 1. Updated Configuration File

**File**: `.golangci.yml`

**Key Changes**:
- Removed deprecated linters: `golint`, `deadcode`, `varcheck`, `structcheck`, `interfacer`, `maligned`, `scopelint`
- Replaced with modern equivalents: `revive` (replaces `golint`), `unused` (replaces `deadcode`, etc.)
- Updated configuration format to be compatible with latest golangci-lint versions
- Added proper exclusion rules for different file types
- Simplified linter settings with better defaults

### 2. Updated GitHub Actions

**File**: `.github/workflows/ci.yml`

**Changes**:
- Updated to `golangci/golangci-lint-action@v6`
- Pinned to specific golangci-lint version: `v1.59.1`
- Added caching configuration
- Added verbose output for better debugging

### 3. Development Tools

**Added Scripts**:
- `scripts/lint.sh` - Local linting with automatic installation
- `scripts/test-config.sh` - Configuration validation
- Updated `scripts/release.sh` - Includes linting in release process

## Key Configuration Highlights

### Enabled Linters
```yaml
linters:
  enable:
    # Essential linters
    - errcheck      # Check for unchecked errors
    - gosimple      # Simplify code
    - govet         # Go vet tool
    - staticcheck   # Advanced static analysis
    - unused        # Find unused code
    
    # Code quality
    - gocyclo       # Cyclomatic complexity
    - funlen        # Function length
    - goconst       # Repeated strings
    
    # Style and formatting
    - gofmt         # Check formatting
    - goimports     # Check imports
    - revive        # Modern golint replacement
    
    # Security
    - gosec         # Security checker
```

### Disabled Linters
```yaml
  disable:
    # Too strict for this codebase
    - gochecknoglobals  # Allow globals (needed for CLI)
    - exhaustive        # Don't require exhaustive switches
    - godox             # Allow TODO comments
```

### Smart Exclusions
```yaml
issues:
  exclude-rules:
    # Relax rules for test files
    - path: _test\.go
      linters: [gocyclo, funlen, dupl, gosec]
    
    # Allow init functions in cmd package
    - path: cmd/
      linters: [gochecknoinits]
```

## Testing the Fix

### Local Testing
```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1

# Test configuration
./scripts/test-config.sh

# Run linting
./scripts/lint.sh
```

### CI/CD Testing

The updated configuration is tested in GitHub Actions:
1. **Configuration validation** during CI
2. **Linting** with specific version
3. **Caching** for faster builds
4. **Error reporting** with context

## Benefits

### 1. **Modern Compatibility**
- Works with latest golangci-lint versions
- Future-proof configuration format
- Deprecated linter migration

### 2. **Better Developer Experience**
- Local development scripts
- Clear error messages
- Faster linting with caching

### 3. **Flexible Rules**
- Context-aware exclusions
- Test file exceptions
- CLI-specific allowances

### 4. **Maintainability**
- Well-documented configuration
- Automatic dependency updates
- Version pinning for stability

## Configuration Reference

### Timeout Settings
```yaml
run:
  timeout: 5m  # Maximum runtime
  go: "1.21"   # Go version requirement
```

### Security Exclusions
```yaml
gosec:
  excludes:
    - G404  # Allow weak RNG for non-crypto use
    - G204  # Allow subprocess execution (needed for scripts)
```

### Performance Settings
```yaml
funlen:
  lines: 100      # Maximum function length
  statements: 50  # Maximum statements per function

gocyclo:
  min-complexity: 15  # Cyclomatic complexity threshold
```

## Troubleshooting

### Common Issues

**Configuration Errors**:
```bash
# Validate YAML syntax
./scripts/test-config.sh

# Check specific linter
golangci-lint help linters
```

**Version Conflicts**:
```bash
# Check installed version
golangci-lint version

# Install specific version
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
```

**Cache Issues**:
```bash
# Clear golangci-lint cache
golangci-lint cache clean

# Disable cache for debugging
golangci-lint run --no-config
```

### GitHub Actions Debugging

If the action still fails:

1. **Check logs** in GitHub Actions tab
2. **Verify Go version** compatibility
3. **Test locally** with same version
4. **Check for breaking changes** in golangci-lint releases

## Migration Guide

### From Old Configuration

**Before** (deprecated):
```yaml
linters:
  enable:
    - golint     # Deprecated
    - deadcode   # Deprecated
    - varcheck   # Deprecated
```

**After** (modern):
```yaml
linters:
  enable:
    - revive     # Replaces golint
    - unused     # Replaces deadcode/varcheck
```

### Update Process

1. **Backup** existing `.golangci.yml`
2. **Replace** with new configuration
3. **Test locally** with `./scripts/lint.sh`
4. **Commit and push** to trigger CI
5. **Monitor** GitHub Actions for success

## Future Maintenance

### Regular Updates

- **Monitor** golangci-lint releases
- **Update** pinned version in workflows
- **Test** configuration compatibility
- **Review** new linter additions

### Dependency Management

Dependabot automatically updates:
- GitHub Actions versions
- Go module dependencies

### Performance Optimization

- **Profile** linting performance
- **Optimize** exclusion rules
- **Cache** results effectively
- **Parallelize** where possible

## Conclusion

The golangci-lint configuration has been modernized to work with the latest versions while maintaining code quality standards appropriate for the Shadowy blockchain project. The new setup provides:

- ✅ **Compatibility** with golangci-lint v1.55+
- ✅ **Comprehensive** code quality checks
- ✅ **Developer-friendly** local tooling
- ✅ **CI/CD integration** with proper caching
- ✅ **Future-proof** configuration format

The fix ensures that code quality checks work reliably in both development and CI/CD environments.