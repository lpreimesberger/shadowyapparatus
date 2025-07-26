#!/bin/bash

# Shadowy Remote Node Management Script
# This script provides easy management of remote Shadowy nodes

set -e

# Configuration
REMOTE_USER="nanocat"
REMOTE_HOST="192.168.68.62"
REMOTE_PATH="/home/nanocat/shadowy"
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

show_help() {
    echo "Shadowy Remote Node Management Script"
    echo ""
    echo "Usage: $0 <command> [options]"
    echo ""
    echo "Commands:"
    echo "  status          Show node status"
    echo "  start           Start the node service"
    echo "  stop            Stop the node service"
    echo "  restart         Restart the node service"
    echo "  logs            Show recent logs"
    echo "  tail            Tail logs in real-time"
    echo "  health          Check node health via API"
    echo "  peers           Show connected peers"
    echo "  blockchain      Show blockchain status"
    echo "  mempool         Show mempool status"
    echo "  mining          Show mining status"
    echo "  shell           Open SSH shell to remote node"
    echo "  cleanup         Clean up old logs and data"
    echo "  backup          Backup node data"
    echo "  restore <file>  Restore node data from backup"
    echo ""
    echo "Options:"
    echo "  --host <host>   Override remote host (default: ${REMOTE_HOST})"
    echo "  --user <user>   Override remote user (default: ${REMOTE_USER})"
    echo "  --help          Show this help message"
}

# Parse command line arguments
COMMAND=""
BACKUP_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --host)
            REMOTE_HOST="$2"
            shift 2
            ;;
        --user)
            REMOTE_USER="$2"
            shift 2
            ;;
        --help)
            show_help
            exit 0
            ;;
        restore)
            COMMAND="restore"
            BACKUP_FILE="$2"
            shift 2
            ;;
        *)
            if [[ -z "$COMMAND" ]]; then
                COMMAND="$1"
            else
                log_error "Unknown option: $1"
                exit 1
            fi
            shift
            ;;
    esac
done

if [[ -z "$COMMAND" ]]; then
    show_help
    exit 1
fi

# Check SSH connection
check_ssh() {
    if ! ssh -o ConnectTimeout=5 -o BatchMode=yes "${REMOTE_USER}@${REMOTE_HOST}" exit 2>/dev/null; then
        log_error "Cannot connect to ${REMOTE_USER}@${REMOTE_HOST} via SSH"
        exit 1
    fi
}

# Execute commands based on the command argument
case "$COMMAND" in
    status)
        log_info "Checking node status on ${REMOTE_HOST}..."
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            echo '=== System Status ==='
            echo 'Uptime:' \$(uptime)
            echo 'Load:' \$(cat /proc/loadavg)
            echo 'Memory:' \$(free -h | grep '^Mem:')
            echo ''
            echo '=== Service Status ==='
            if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
                echo 'Service: RUNNING'
                systemctl status ${SERVICE_NAME} --no-pager -l | head -20
            else
                echo 'Service: NOT RUNNING'
            fi
            echo ''
            echo '=== Process Status ==='
            ps aux | grep shadowyapparatus | grep -v grep || echo 'No shadowyapparatus processes found'
            echo ''
            echo '=== Port Status ==='
            netstat -tlpn | grep -E ':(8080|8888|9090)' || echo 'No services listening on expected ports'
        "
        ;;
        
    start)
        log_info "Starting node service on ${REMOTE_HOST}..."
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            if systemctl is-enabled --quiet ${SERVICE_NAME} 2>/dev/null; then
                sudo systemctl start ${SERVICE_NAME}
                echo 'Service started via systemd'
            else
                echo 'Starting manually...'
                cd ${REMOTE_PATH}
                nohup ./start-node.sh > logs/nohup.log 2>&1 &
                echo 'Node started in background'
            fi
        "
        ;;
        
    stop)
        log_info "Stopping node service on ${REMOTE_HOST}..."
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
                sudo systemctl stop ${SERVICE_NAME}
                echo 'Service stopped via systemd'
            else
                echo 'Stopping manually...'
                pkill -f shadowyapparatus || echo 'No processes to stop'
            fi
        "
        ;;
        
    restart)
        log_info "Restarting node service on ${REMOTE_HOST}..."
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            if systemctl is-enabled --quiet ${SERVICE_NAME} 2>/dev/null; then
                sudo systemctl restart ${SERVICE_NAME}
                echo 'Service restarted via systemd'
            else
                echo 'Restarting manually...'
                pkill -f shadowyapparatus || true
                sleep 2
                cd ${REMOTE_PATH}
                nohup ./start-node.sh > logs/nohup.log 2>&1 &
                echo 'Node restarted in background'
            fi
        "
        ;;
        
    logs)
        log_info "Showing recent logs from ${REMOTE_HOST}..."
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            if [[ -f ${REMOTE_PATH}/logs/shadowy.log ]]; then
                echo '=== Recent Application Logs ==='
                tail -50 ${REMOTE_PATH}/logs/shadowy.log
            fi
            if systemctl is-active --quiet ${SERVICE_NAME} 2>/dev/null; then
                echo ''
                echo '=== Recent Service Logs ==='
                sudo journalctl -u ${SERVICE_NAME} --no-pager -l -n 20
            fi
        "
        ;;
        
    tail)
        log_info "Tailing logs from ${REMOTE_HOST}... (Press Ctrl+C to exit)"
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            if [[ -f ${REMOTE_PATH}/logs/shadowy.log ]]; then
                tail -f ${REMOTE_PATH}/logs/shadowy.log
            else
                echo 'No log file found, trying systemd logs...'
                sudo journalctl -u ${SERVICE_NAME} -f
            fi
        "
        ;;
        
    health)
        log_info "Checking node health via API on ${REMOTE_HOST}..."
        if curl -s --connect-timeout 5 "http://${REMOTE_HOST}:8080/api/v1/health" | jq . 2>/dev/null; then
            log_success "Node API is responding"
        else
            log_warning "Node API is not responding, trying basic HTTP check..."
            curl -s --connect-timeout 5 "http://${REMOTE_HOST}:8080/api/v1/health" || log_error "API not reachable"
        fi
        ;;
        
    peers)
        log_info "Checking connected peers on ${REMOTE_HOST}..."
        if curl -s --connect-timeout 5 "http://${REMOTE_HOST}:8080/api/v1/consensus/peers" | jq . 2>/dev/null; then
            log_success "Peer information retrieved"
        else
            log_error "Could not retrieve peer information"
        fi
        ;;
        
    blockchain)
        log_info "Checking blockchain status on ${REMOTE_HOST}..."
        if curl -s --connect-timeout 5 "http://${REMOTE_HOST}:8080/api/v1/blockchain" | jq . 2>/dev/null; then
            log_success "Blockchain information retrieved"
        else
            log_error "Could not retrieve blockchain information"
        fi
        ;;
        
    mempool)
        log_info "Checking mempool status on ${REMOTE_HOST}..."
        if curl -s --connect-timeout 5 "http://${REMOTE_HOST}:8080/api/v1/mempool" | jq . 2>/dev/null; then
            log_success "Mempool information retrieved"
        else
            log_error "Could not retrieve mempool information"
        fi
        ;;
        
    mining)
        log_info "Checking mining status on ${REMOTE_HOST}..."
        if curl -s --connect-timeout 5 "http://${REMOTE_HOST}:8080/api/v1/mining/status" | jq . 2>/dev/null; then
            log_success "Mining information retrieved"
        else
            log_error "Could not retrieve mining information"
        fi
        ;;
        
    shell)
        log_info "Opening SSH shell to ${REMOTE_HOST}..."
        ssh "${REMOTE_USER}@${REMOTE_HOST}" -t "cd ${REMOTE_PATH}; exec bash -l"
        ;;
        
    cleanup)
        log_info "Cleaning up old logs and temporary data on ${REMOTE_HOST}..."
        check_ssh
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            cd ${REMOTE_PATH}
            echo 'Cleaning up logs older than 7 days...'
            find logs/ -name '*.log' -mtime +7 -delete 2>/dev/null || true
            echo 'Cleaning up temporary files...'
            rm -f logs/nohup.log logs/*.tmp 2>/dev/null || true
            echo 'Cleanup completed'
            du -sh data/ logs/ 2>/dev/null || true
        "
        ;;
        
    backup)
        log_info "Creating backup of node data on ${REMOTE_HOST}..."
        check_ssh
        BACKUP_NAME="shadowy-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
        ssh "${REMOTE_USER}@${REMOTE_HOST}" "
            cd ${REMOTE_PATH}
            echo 'Creating backup: ${BACKUP_NAME}'
            tar -czf backups/${BACKUP_NAME} data/ config/ 2>/dev/null || mkdir -p backups && tar -czf backups/${BACKUP_NAME} data/ config/
            echo 'Backup created: backups/${BACKUP_NAME}'
            ls -lh backups/${BACKUP_NAME}
        "
        log_success "Backup created: ${BACKUP_NAME}"
        ;;
        
    restore)
        if [[ -z "$BACKUP_FILE" ]]; then
            log_error "Please specify backup file to restore"
            exit 1
        fi
        log_warning "This will overwrite existing data. Continue? (y/N)"
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            log_info "Restoring from backup: ${BACKUP_FILE}"
            check_ssh
            ssh "${REMOTE_USER}@${REMOTE_HOST}" "
                cd ${REMOTE_PATH}
                if [[ -f 'backups/${BACKUP_FILE}' ]]; then
                    echo 'Stopping service...'
                    sudo systemctl stop ${SERVICE_NAME} 2>/dev/null || pkill -f shadowyapparatus || true
                    echo 'Restoring backup...'
                    tar -xzf backups/${BACKUP_FILE}
                    echo 'Restore completed'
                else
                    echo 'Backup file not found: backups/${BACKUP_FILE}'
                    exit 1
                fi
            "
        else
            log_info "Restore cancelled"
        fi
        ;;
        
    *)
        log_error "Unknown command: $COMMAND"
        show_help
        exit 1
        ;;
esac