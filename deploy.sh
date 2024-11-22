#!/bin/bash

# deploy.sh
set -e  # Exit on error

# Configuration
APP_DIR="/var/www/dawhub"
BACKUP_DIR="$APP_DIR/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR:${NC} $1"
}

warning() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING:${NC} $1"
}

# Create backup of current database
create_backup() {
    log "Creating database backup..."
    mkdir -p "$BACKUP_DIR"
    source .env
    docker exec dawhub_db_1 pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$BACKUP_DIR/db_backup_$TIMESTAMP.sql.gz"
    log "Backup created at $BACKUP_DIR/db_backup_$TIMESTAMP.sql.gz"
}

# Health check function
health_check() {
    local retries=5
    local wait_time=10
    local count=0
    
    log "Performing health check..."
    
    while [ $count -lt $retries ]; do
        if curl -f "http://localhost:8080/health" &>/dev/null; then
            log "Health check passed!"
            return 0
        fi
        
        count=$((count + 1))
        warning "Health check failed, attempt $count of $retries"
        sleep $wait_time
    done
    
    error "Health check failed after $retries attempts"
    return 1
}

# Main deployment process
main() {
    log "Starting deployment process..."
    
    # Navigate to app directory
    cd "$APP_DIR"

    # Stash any local changes (if any)
    git stash

    # Pull latest changes
    log "Pulling latest changes..."
    git fetch origin main
    git reset --hard origin/main
    
    # Create backup
    create_backup
    
    # Build and deploy
    log "Building and deploying containers..."
    docker-compose build app
    
    # Store the current container ID for rollback
    OLD_CONTAINER_ID=$(docker-compose ps -q app)
    
    # Bring down the service gracefully
    log "Stopping current services..."
    docker-compose down
    
    # Start the new version
    log "Starting new version..."
    docker-compose up -d
    
    # Health check
    if ! health_check; then
        error "Deployment failed health check, initiating rollback..."
        docker-compose down
        if [ ! -z "$OLD_CONTAINER_ID" ]; then
            docker start $OLD_CONTAINER_ID
            warning "Rolled back to previous version"
        fi
        exit 1
    fi
    
    # Clean up old backups (keep last 5)
    log "Cleaning up old backups..."
    ls -t "$BACKUP_DIR"/db_backup_*.sql.gz | tail -n +6 | xargs -r rm
    
    # Clean up old images
    log "Cleaning up old Docker images..."
    docker image prune -f
    
    log "Deployment completed successfully!"
    
    # Print container status
    log "Current container status:"
    docker-compose ps
}

# Execute main function with error handling
if main; then
    log "Deployment finished successfully!"
else
    error "Deployment failed!"
    exit 1
fi