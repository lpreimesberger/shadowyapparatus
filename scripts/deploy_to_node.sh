#!/bin/bash

# Shadowy Node Deployment Script
# This script builds the binary and deploys it to a remote node

set -e

# Configuration
REMOTE_USER="nanocat"
REMOTE_HOST="192.168.68.62"
REMOTE_PATH="/home/nanocat/shadowy"
BINARY_NAME="shadowyapparatus"
SERVICE_NAME="shadowy-node"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Parse command line arguments
CLEAN_BUILD=false
RESTART_SERVICE=true
INSTALL_SERVICE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --clean)
            CLEAN_BUILD=true
            shift
            ;;
        --no-restart)
            RESTART_SERVICE=false
            shift
            ;;
        --install-service)
            INSTALL_SERVICE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  --clean         Clean build (removes existing build artifacts)"
            echo "  --no-restart    Don't restart the service after deployment"
            echo "  --install-service Install systemd service"
            echo "  --help          Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Check if we're in the correct directory
if [[ ! -f "go.mod" ]] || [[ ! -f "main.go" ]]; then
    log_error "This script must be run from the shadowyapparatus project root directory"
    exit 1
fi

# Check if SSH key authentication is working
log_info "Testing SSH connection to ${REMOTE_USER}@${REMOTE_HOST}..."
if ! ssh -o ConnectTimeout=10 -o BatchMode=yes "${REMOTE_USER}@${REMOTE_HOST}" exit 2>/dev/null; then
    log_error "Cannot connect to ${REMOTE_USER}@${REMOTE_HOST} via SSH"
    log_error "Please ensure SSH key authentication is set up"
    exit 1
fi
log_success "SSH connection successful"

# Clean build if requested
if [[ "$CLEAN_BUILD" == "true" ]]; then
    log_info "Cleaning build artifacts..."
    go clean -cache
    go clean -modcache
    rm -f "${BINARY_NAME}"
fi

# Build the binary for Linux with version information
log_info "Building binary for Linux amd64..."
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

# Get version information
log_info "Generating version information..."
VERSION_LDFLAGS=$(./scripts/build_version.sh --ldflags)

# Build with version information embedded
if ! go build -o "${BINARY_NAME}" -ldflags="-s -w ${VERSION_LDFLAGS}" .; then
    log_error "Failed to build binary"
    exit 1
fi
log_success "Binary built successfully"

# Show version information
log_info "Build information:"
./scripts/build_version.sh

# Get binary info
BINARY_SIZE=$(ls -lh "${BINARY_NAME}" | awk '{print $5}')
BINARY_HASH=$(sha256sum "${BINARY_NAME}" | cut -d' ' -f1)
log_info "Binary size: ${BINARY_SIZE}"
log_info "Binary SHA256: ${BINARY_HASH}"

# Create remote directory structure
log_info "Setting up remote directory structure..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "
    mkdir -p ${REMOTE_PATH}/{bin,config,data,logs,plots}
    mkdir -p ${REMOTE_PATH}/data/{blockchain,mempool,wallets}
"

# Stop the service if it's running
if [[ "$RESTART_SERVICE" == "true" ]]; then
    log_info "Stopping remote service..."
    ssh "${REMOTE_USER}@${REMOTE_HOST}" "
        if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
            sudo systemctl stop ${SERVICE_NAME}
            echo 'Service stopped'
        else
            echo 'Service was not running'
        fi
    " || log_warning "Could not stop service (might not be installed yet)"
fi

# Deploy the binary
log_info "Deploying binary to remote host..."
if ! scp "${BINARY_NAME}" "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}/bin/"; then
    log_error "Failed to copy binary to remote host"
    exit 1
fi

# Make binary executable
ssh "${REMOTE_USER}@${REMOTE_HOST}" "chmod +x ${REMOTE_PATH}/bin/${BINARY_NAME}"
log_success "Binary deployed and made executable"

# Deploy configuration files
log_info "Deploying configuration files..."
scp -r config/* "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}/config/" 2>/dev/null || log_warning "No config directory found"

# Deploy scripts
log_info "Deploying management scripts..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "mkdir -p ${REMOTE_PATH}/scripts"
scp scripts/*.sh "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_PATH}/scripts/" 2>/dev/null || log_warning "No scripts directory found"
ssh "${REMOTE_USER}@${REMOTE_HOST}" "chmod +x ${REMOTE_PATH}/scripts/*.sh" 2>/dev/null || true

# Install systemd service if requested
if [[ "$INSTALL_SERVICE" == "true" ]]; then
    log_info "Installing systemd service..."
    
    # Create systemd service file
    ssh "${REMOTE_USER}@${REMOTE_HOST}" "cat > /tmp/${SERVICE_NAME}.service << 'EOF'
[Unit]
Description=Shadowy Blockchain Node
After=network.target
Wants=network.target

[Service]
Type=simple
User=${REMOTE_USER}
Group=${REMOTE_USER}
WorkingDirectory=${REMOTE_PATH}
ExecStart=${REMOTE_PATH}/bin/${BINARY_NAME} start --config ${REMOTE_PATH}/config/node.json
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=append:${REMOTE_PATH}/logs/shadowy.log
StandardError=append:${REMOTE_PATH}/logs/shadowy-error.log
LimitNOFILE=1048576
LimitNPROC=1048576
LimitCORE=infinity
Environment=GOMAXPROCS=4

[Install]
WantedBy=multi-user.target
EOF"

    # Install the service
    ssh "${REMOTE_USER}@${REMOTE_HOST}" "
        sudo mv /tmp/${SERVICE_NAME}.service /etc/systemd/system/
        sudo systemctl daemon-reload
        sudo systemctl enable ${SERVICE_NAME}
    "
    log_success "Systemd service installed and enabled"
fi

# Create or update node configuration
log_info "Creating node configuration..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "cat > ${REMOTE_PATH}/config/node.json << 'EOF'
{
  \"http_port\": 8080,
  \"grpc_port\": 9090,
  \"enable_http\": true,
  \"enable_grpc\": true,
  \"enable_farming\": true,
  \"enable_timelord\": false,
  \"enable_mining\": true,
  \"enable_consensus\": true,
  \"bootstrap_peers\": [
    \"192.168.68.90:8888\",
    \"192.168.68.62:8888\"
  ],
  \"consensus_config\": {
    \"listen_addr\": \"0.0.0.0:8888\",
    \"max_peers\": 50,
    \"enable_bootstrap\": true
  },
  \"data_dir\": \"${REMOTE_PATH}/data\",
  \"plots_dir\": \"${REMOTE_PATH}/plots\",
  \"logs_dir\": \"${REMOTE_PATH}/logs\"
}
EOF"

# Create startup script
log_info "Creating startup script..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "cat > ${REMOTE_PATH}/start-node.sh << 'EOF'
#!/bin/bash
cd ${REMOTE_PATH}
export SHADOWY_CONFIG_PATH=${REMOTE_PATH}/config/node.json
export SHADOWY_DATA_DIR=${REMOTE_PATH}/data
export SHADOWY_PLOTS_DIR=${REMOTE_PATH}/plots
export SHADOWY_LOGS_DIR=${REMOTE_PATH}/logs

# Ensure directories exist
mkdir -p \${SHADOWY_DATA_DIR}/{blockchain,mempool,wallets}
mkdir -p \${SHADOWY_PLOTS_DIR}
mkdir -p \${SHADOWY_LOGS_DIR}

# Start the node
exec ./bin/${BINARY_NAME} start --config \${SHADOWY_CONFIG_PATH}
EOF"

ssh "${REMOTE_USER}@${REMOTE_HOST}" "chmod +x ${REMOTE_PATH}/start-node.sh"

# Start the service if requested
if [[ "$RESTART_SERVICE" == "true" ]]; then
    log_info "Starting remote service..."
    ssh "${REMOTE_USER}@${REMOTE_HOST}" "
        if systemctl is-enabled --quiet ${SERVICE_NAME} 2>/dev/null; then
            sudo systemctl start ${SERVICE_NAME}
            echo 'Service started via systemd'
        else
            echo 'Systemd service not enabled, you can start manually with:'
            echo '  cd ${REMOTE_PATH} && ./start-node.sh'
        fi
    "
    
    # Wait a moment and check status
    sleep 3
    log_info "Checking service status..."
    ssh "${REMOTE_USER}@${REMOTE_HOST}" "
        if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
            echo 'Service is running'
            systemctl status ${SERVICE_NAME} --no-pager -l
        else
            echo 'Service is not running via systemd'
        fi
    " || log_warning "Could not check service status"
fi

# Clean up local binary
rm -f "${BINARY_NAME}"

log_success "Deployment completed!"
log_info "Remote node details:"
log_info "  Host: ${REMOTE_USER}@${REMOTE_HOST}"
log_info "  Path: ${REMOTE_PATH}"
log_info "  HTTP API: http://${REMOTE_HOST}:8080"
log_info "  P2P Port: ${REMOTE_HOST}:8888"
log_info ""
log_info "Useful commands:"
log_info "  Check status: ssh ${REMOTE_USER}@${REMOTE_HOST} 'sudo systemctl status ${SERVICE_NAME}'"
log_info "  View logs: ssh ${REMOTE_USER}@${REMOTE_HOST} 'tail -f ${REMOTE_PATH}/logs/shadowy.log'"
log_info "  Connect to API: curl http://${REMOTE_HOST}:8080/api/v1/health"