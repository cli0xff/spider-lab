#!/usr/bin/env bash
#
# spider-lab.sh — Build, start, stop, and manage all Precision Observer services
#
# Usage:
#   ./spider-lab.sh build          Build backend binaries and frontend bundle
#   ./spider-lab.sh start          Start all services (mongo, master, worker, frontend)
#   ./spider-lab.sh stop           Stop all running services
#   ./spider-lab.sh restart        Stop then start all services
#   ./spider-lab.sh status         Show status of all services
#   ./spider-lab.sh logs [service] Tail logs (master|worker|frontend|mongo or all)
#   ./spider-lab.sh clean          Stop services and remove build artifacts
#   ./spider-lab.sh purge-data     Delete all scraped topics and posts from MongoDB
#   ./spider-lab.sh docker-up      Start all services via docker-compose
#   ./spider-lab.sh docker-down    Stop docker-compose services
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$SCRIPT_DIR/backend"
FRONTEND_DIR="$SCRIPT_DIR/frontend"
BIN_DIR="$BACKEND_DIR/bin"
PID_DIR="$SCRIPT_DIR/.pids"
LOG_DIR="$SCRIPT_DIR/.logs"
MONGO_CONTAINER="spider-lab-mongo"

# Default config (override with environment variables)
export SPIDER_LAB_MONGO_URI="${SPIDER_LAB_MONGO_URI:-mongodb://localhost:27017}"
export SPIDER_LAB_MONGO_DB="${SPIDER_LAB_MONGO_DB:-spider_lab}"
export SPIDER_LAB_GRPC_ADDRESS="${SPIDER_LAB_GRPC_ADDRESS:-0.0.0.0:9666}"
export SPIDER_LAB_API_ADDRESS="${SPIDER_LAB_API_ADDRESS:-0.0.0.0:8080}"
export SPIDER_LAB_MASTER_GRPC="${SPIDER_LAB_MASTER_GRPC:-localhost:9666}"
export SPIDER_LAB_JWT_SECRET="${SPIDER_LAB_JWT_SECRET:-spider-lab-jwt-secret-key}"
export SPIDER_LAB_MAX_RUNNERS="${SPIDER_LAB_MAX_RUNNERS:-8}"
export SPIDER_LAB_SPIDER_DIR="${SPIDER_LAB_SPIDER_DIR:-/tmp/spider-lab/spiders}"
MONGO_PORT="${MONGO_PORT:-27017}"
FRONTEND_PORT="${FRONTEND_PORT:-3000}"

# Storage (MinIO) config
STORAGE_CONTAINER="spider-lab-storage"
STORAGE_API_PORT="${STORAGE_API_PORT:-9000}"
STORAGE_CONSOLE_PORT="${STORAGE_CONSOLE_PORT:-9001}"
export SPIDER_LAB_STORAGE_ENDPOINT="${SPIDER_LAB_STORAGE_ENDPOINT:-localhost:$STORAGE_API_PORT}"
export SPIDER_LAB_STORAGE_ACCESS_KEY="${SPIDER_LAB_STORAGE_ACCESS_KEY:-minioadmin}"
export SPIDER_LAB_STORAGE_SECRET_KEY="${SPIDER_LAB_STORAGE_SECRET_KEY:-minioadmin}"
export SPIDER_LAB_STORAGE_BUCKET="${SPIDER_LAB_STORAGE_BUCKET:-spider-lab}"
export SPIDER_LAB_STORAGE_USE_SSL="${SPIDER_LAB_STORAGE_USE_SSL:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${CYAN}${BOLD}==>${NC} ${BOLD}$*${NC}"; }

ensure_dirs() {
    mkdir -p "$BIN_DIR" "$PID_DIR" "$LOG_DIR" "$SPIDER_LAB_SPIDER_DIR"
}

# ---------------------------------------------------------------------------
# Mongo connectivity check
# ---------------------------------------------------------------------------
mongo_is_reachable() {
    # Try mongosh first, then fall back to docker exec, then raw TCP
    if command -v mongosh &>/dev/null; then
        mongosh --quiet --port "$MONGO_PORT" --eval 'db.runCommand("ping").ok' 2>/dev/null | grep -q 1
    elif docker inspect "$MONGO_CONTAINER" &>/dev/null; then
        docker exec "$MONGO_CONTAINER" mongosh --quiet --eval 'db.runCommand("ping").ok' 2>/dev/null | grep -q 1
    else
        # Raw TCP check
        (echo > /dev/tcp/127.0.0.1/"$MONGO_PORT") 2>/dev/null
    fi
}

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
build_backend() {
    cd "$BACKEND_DIR"
    log_info "Building master node..."
    go build -o "$BIN_DIR/master" ./cmd/master
    log_info "Building worker node..."
    go build -o "$BIN_DIR/worker" ./cmd/worker

    # Copy helper scripts alongside the binaries
    if [ -d "$BACKEND_DIR/scripts" ]; then
        cp -r "$BACKEND_DIR/scripts" "$BIN_DIR/../scripts" 2>/dev/null || true
    fi

    log_info "Backend binaries: $BIN_DIR/master, $BIN_DIR/worker"
}

build_frontend() {
    cd "$FRONTEND_DIR"
    if [ ! -d "node_modules" ]; then
        log_info "Installing npm dependencies..."
        npm install --silent
    fi
    log_info "Running production build..."
    npm run build --silent
    log_info "Frontend bundle: $FRONTEND_DIR/dist/"
}

cmd_build() {
    ensure_dirs
    log_step "Building backend binaries"
    build_backend

    log_step "Building frontend"
    build_frontend

    echo ""
    log_info "Build complete."
}

# ---------------------------------------------------------------------------
# Service management helpers
# ---------------------------------------------------------------------------
save_pid() {
    local name="$1" pid="$2"
    echo "$pid" > "$PID_DIR/$name.pid"
}

read_pid() {
    local name="$1"
    local pidfile="$PID_DIR/$name.pid"
    if [ -f "$pidfile" ]; then
        cat "$pidfile"
    fi
}

is_running() {
    local name="$1"
    local pid
    pid=$(read_pid "$name")
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        return 0
    fi
    return 1
}

stop_service() {
    local name="$1"
    local pid
    pid=$(read_pid "$name")
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        log_info "Stopping $name (PID $pid)..."
        # Kill child processes first (e.g. vite spawned by npx)
        pkill -P "$pid" 2>/dev/null || true
        kill "$pid" 2>/dev/null || true
        for _ in $(seq 1 50); do
            if ! kill -0 "$pid" 2>/dev/null; then
                break
            fi
            sleep 0.1
        done
        if kill -0 "$pid" 2>/dev/null; then
            log_warn "$name didn't stop gracefully, sending SIGKILL..."
            pkill -9 -P "$pid" 2>/dev/null || true
            kill -9 "$pid" 2>/dev/null || true
        fi
        rm -f "$PID_DIR/$name.pid"
        log_info "$name stopped."
    else
        rm -f "$PID_DIR/$name.pid"
    fi
}

# ---------------------------------------------------------------------------
# Start individual services
# ---------------------------------------------------------------------------
start_mongo() {
    # 1) Already reachable? (systemd, external docker, etc.)
    if mongo_is_reachable; then
        log_info "MongoDB is already reachable on port $MONGO_PORT"
        return 0
    fi

    # 2) Try native mongod
    if command -v mongod &>/dev/null; then
        local mongo_data="$SCRIPT_DIR/.data/mongo"
        mkdir -p "$mongo_data"
        log_info "Starting MongoDB via mongod on port $MONGO_PORT..."
        mongod --dbpath "$mongo_data" --port "$MONGO_PORT" --fork \
            --logpath "$LOG_DIR/mongo.log" --pidfilepath "$PID_DIR/mongo.pid" \
            --bind_ip 127.0.0.1 >/dev/null 2>&1

        for _ in $(seq 1 30); do
            if mongo_is_reachable; then
                log_info "MongoDB ready on port $MONGO_PORT"
                return 0
            fi
            sleep 0.5
        done
        log_error "MongoDB failed to start within 15 seconds"
        return 1
    fi

    # 3) Try Docker
    if command -v docker &>/dev/null; then
        # Check if container already exists but is stopped
        if docker inspect "$MONGO_CONTAINER" &>/dev/null; then
            local state
            state=$(docker inspect -f '{{.State.Running}}' "$MONGO_CONTAINER" 2>/dev/null)
            if [ "$state" = "true" ]; then
                log_info "MongoDB Docker container '$MONGO_CONTAINER' is already running"
                # Wait for it to be fully ready
                for _ in $(seq 1 20); do
                    if mongo_is_reachable; then
                        return 0
                    fi
                    sleep 0.5
                done
                return 0
            else
                log_info "Starting existing MongoDB container '$MONGO_CONTAINER'..."
                docker start "$MONGO_CONTAINER" >/dev/null 2>&1
            fi
        else
            log_info "Starting MongoDB via Docker (mongo:7) on port $MONGO_PORT..."
            docker run -d \
                --name "$MONGO_CONTAINER" \
                -p "$MONGO_PORT":27017 \
                -v spider-lab-mongo-data:/data/db \
                mongo:7 >/dev/null 2>&1
        fi

        # Wait for mongo to be ready
        for _ in $(seq 1 40); do
            if mongo_is_reachable; then
                log_info "MongoDB ready on port $MONGO_PORT (Docker)"
                return 0
            fi
            sleep 0.5
        done
        log_error "MongoDB Docker container failed to become ready within 20 seconds"
        docker logs "$MONGO_CONTAINER" --tail 10 2>&1 || true
        return 1
    fi

    log_error "Neither mongod nor docker found. Cannot start MongoDB."
    log_info "Install MongoDB, Docker, or start MongoDB manually on port $MONGO_PORT"
    return 1
}

stop_mongo() {
    # Stop our PID-managed mongod
    if [ -f "$PID_DIR/mongo.pid" ]; then
        stop_service "mongo"
        return
    fi

    # Stop our Docker container
    if command -v docker &>/dev/null; then
        if docker inspect "$MONGO_CONTAINER" &>/dev/null; then
            local state
            state=$(docker inspect -f '{{.State.Running}}' "$MONGO_CONTAINER" 2>/dev/null)
            if [ "$state" = "true" ]; then
                log_info "Stopping MongoDB Docker container '$MONGO_CONTAINER'..."
                docker stop "$MONGO_CONTAINER" >/dev/null 2>&1
                log_info "MongoDB container stopped."
            fi
        fi
    fi
}

# ---------------------------------------------------------------------------
# MinIO Storage
# ---------------------------------------------------------------------------
storage_is_reachable() {
    curl -sf "http://localhost:$STORAGE_API_PORT/minio/health/live" >/dev/null 2>&1
}

start_storage() {
    if storage_is_reachable; then
        log_info "MinIO storage is already reachable on port $STORAGE_API_PORT"
        return 0
    fi

    if ! command -v docker &>/dev/null; then
        log_warn "Docker not found — skipping MinIO storage (images will be served from local filesystem only)"
        export SPIDER_LAB_STORAGE_ENDPOINT=""
        return 0
    fi

    # Check if container exists but is stopped
    if docker inspect "$STORAGE_CONTAINER" &>/dev/null; then
        local state
        state=$(docker inspect -f '{{.State.Running}}' "$STORAGE_CONTAINER" 2>/dev/null)
        if [ "$state" = "true" ]; then
            log_info "MinIO container '$STORAGE_CONTAINER' is already running"
            return 0
        else
            log_info "Starting existing MinIO container '$STORAGE_CONTAINER'..."
            docker start "$STORAGE_CONTAINER" >/dev/null 2>&1
        fi
    else
        log_info "Starting MinIO via Docker on ports $STORAGE_API_PORT/$STORAGE_CONSOLE_PORT..."
        docker run -d \
            --name "$STORAGE_CONTAINER" \
            -p "$STORAGE_API_PORT":9000 \
            -p "$STORAGE_CONSOLE_PORT":9001 \
            -e "MINIO_ROOT_USER=$SPIDER_LAB_STORAGE_ACCESS_KEY" \
            -e "MINIO_ROOT_PASSWORD=$SPIDER_LAB_STORAGE_SECRET_KEY" \
            -v spider-lab-storage-data:/data \
            minio/minio:latest server /data --console-address ":9001" >/dev/null 2>&1
    fi

    # Wait for MinIO to be ready
    for _ in $(seq 1 40); do
        if storage_is_reachable; then
            log_info "MinIO storage ready on port $STORAGE_API_PORT (console: $STORAGE_CONSOLE_PORT)"
            return 0
        fi
        sleep 0.5
    done
    log_warn "MinIO did not become ready within 20s — images will use local filesystem only"
    export SPIDER_LAB_STORAGE_ENDPOINT=""
    return 0
}

stop_storage() {
    if command -v docker &>/dev/null; then
        if docker inspect "$STORAGE_CONTAINER" &>/dev/null; then
            local state
            state=$(docker inspect -f '{{.State.Running}}' "$STORAGE_CONTAINER" 2>/dev/null)
            if [ "$state" = "true" ]; then
                log_info "Stopping MinIO container '$STORAGE_CONTAINER'..."
                docker stop "$STORAGE_CONTAINER" >/dev/null 2>&1
                log_info "MinIO container stopped."
            fi
        fi
    fi
}

start_master() {
    if is_running "master"; then
        log_warn "Master node is already running (PID $(read_pid master))"
        return 0
    fi

    local binary="$BIN_DIR/master"
    if [ ! -x "$binary" ]; then
        log_error "Master binary not found. Run '$0 build' first."
        return 1
    fi

    # Verify MongoDB is reachable before starting
    if ! mongo_is_reachable; then
        log_error "MongoDB is not reachable on port $MONGO_PORT. Cannot start master."
        return 1
    fi

    log_info "Starting master node (API :8080, gRPC :9666)..."
    export SPIDER_LAB_MASTER=true
    nohup "$binary" > "$LOG_DIR/master.log" 2>&1 &
    local master_pid=$!
    save_pid "master" "$master_pid"

    # Wait for master to be ready (check process is alive + API responds)
    for _ in $(seq 1 40); do
        if ! kill -0 "$master_pid" 2>/dev/null; then
            log_error "Master node crashed on startup. Check $LOG_DIR/master.log"
            rm -f "$PID_DIR/master.pid"
            tail -5 "$LOG_DIR/master.log" 2>/dev/null || true
            return 1
        fi
        if curl -sf http://localhost:8080/api/ping >/dev/null 2>&1; then
            log_info "Master node ready (PID $master_pid)"
            return 0
        fi
        sleep 0.5
    done

    # Process alive but no ping endpoint — that's okay
    if is_running "master"; then
        log_info "Master node started (PID $master_pid)"
        return 0
    fi
    log_error "Master node failed to start. Check $LOG_DIR/master.log"
    return 1
}

start_worker() {
    if is_running "worker"; then
        log_warn "Worker node is already running (PID $(read_pid worker))"
        return 0
    fi

    local binary="$BIN_DIR/worker"
    if [ ! -x "$binary" ]; then
        log_error "Worker binary not found. Run '$0 build' first."
        return 1
    fi

    # Verify master is running
    if ! is_running "master"; then
        log_error "Master node is not running. Cannot start worker."
        return 1
    fi

    log_info "Starting worker node (connecting to master at $SPIDER_LAB_MASTER_GRPC)..."
    export SPIDER_LAB_MASTER=false
    nohup "$binary" > "$LOG_DIR/worker.log" 2>&1 &
    local worker_pid=$!
    save_pid "worker" "$worker_pid"

    # Wait a moment and check if it's still alive
    sleep 2
    if ! kill -0 "$worker_pid" 2>/dev/null; then
        log_error "Worker node crashed on startup. Check $LOG_DIR/worker.log"
        rm -f "$PID_DIR/worker.pid"
        tail -5 "$LOG_DIR/worker.log" 2>/dev/null || true
        return 1
    fi
    log_info "Worker node started (PID $worker_pid)"
}

start_frontend() {
    if is_running "frontend"; then
        log_warn "Frontend is already running (PID $(read_pid frontend))"
        return 0
    fi

    cd "$FRONTEND_DIR"

    # Find a free port starting from FRONTEND_PORT
    local port="$FRONTEND_PORT"
    while (echo >/dev/tcp/127.0.0.1/"$port") 2>/dev/null; do
        log_warn "Port $port is in use, trying $((port + 1))..."
        port=$((port + 1))
    done
    FRONTEND_PORT="$port"

    # Use Vite dev server — it handles proxying /api to the backend
    log_info "Starting frontend dev server on port $FRONTEND_PORT..."
    nohup npx vite --port "$FRONTEND_PORT" --strictPort --host 0.0.0.0 > "$LOG_DIR/frontend.log" 2>&1 &
    local fe_pid=$!
    save_pid "frontend" "$fe_pid"

    # Wait for it to come up
    for _ in $(seq 1 20); do
        if ! kill -0 "$fe_pid" 2>/dev/null; then
            log_error "Frontend crashed on startup. Check $LOG_DIR/frontend.log"
            rm -f "$PID_DIR/frontend.pid"
            tail -5 "$LOG_DIR/frontend.log" 2>/dev/null || true
            return 1
        fi
        if curl -sf http://localhost:"$FRONTEND_PORT" >/dev/null 2>&1; then
            log_info "Frontend ready (PID $fe_pid) — http://localhost:$FRONTEND_PORT"
            return 0
        fi
        sleep 0.5
    done

    if is_running "frontend"; then
        log_info "Frontend started (PID $fe_pid) — http://localhost:$FRONTEND_PORT"
    else
        log_error "Frontend failed to start. Check $LOG_DIR/frontend.log"
        return 1
    fi
}

# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------
cmd_start() {
    ensure_dirs

    # Auto-rebuild backend + frontend before starting
    log_step "Rebuilding before start"
    build_backend
    build_frontend

    echo ""
    echo -e "${BOLD}  精准观察者 Precision Observer${NC}"
    echo -e "  ─────────────────────────────"
    echo ""

    log_step "Starting MongoDB"
    start_mongo

    log_step "Starting Storage (MinIO)"
    start_storage

    log_step "Starting Master Node"
    start_master

    log_step "Starting Worker Node"
    start_worker

    log_step "Starting Frontend"
    start_frontend

    echo ""
    log_step "All services started"
    echo ""
    cmd_status
    echo ""
    echo -e "  ${BOLD}Dashboard:${NC}  http://localhost:$FRONTEND_PORT"
    echo -e "  ${BOLD}API:${NC}        http://localhost:8080"
    echo -e "  ${BOLD}Login:${NC}      admin / admin123"
    echo ""
}

cmd_stop() {
    ensure_dirs
    log_step "Stopping all services"

    stop_service "frontend"
    stop_service "worker"
    stop_service "master"
    stop_storage
    stop_mongo

    log_info "All services stopped."
}

cmd_restart() {
    cmd_stop
    echo ""
    cmd_start
}

cmd_status() {
    local fmt="  %-12s %-10s %s\n"
    printf "${BOLD}${fmt}${NC}" "SERVICE" "STATUS" "DETAILS"
    printf "  %-12s %-10s %s\n" "───────────" "─────────" "───────────────────────"

    # MongoDB
    if mongo_is_reachable; then
        local mongo_detail="port $MONGO_PORT"
        if [ -f "$PID_DIR/mongo.pid" ]; then
            mongo_detail="PID $(read_pid mongo) — port $MONGO_PORT"
        elif docker inspect -f '{{.State.Running}}' "$MONGO_CONTAINER" 2>/dev/null | grep -q true; then
            mongo_detail="(docker: $MONGO_CONTAINER) port $MONGO_PORT"
        else
            mongo_detail="(external) port $MONGO_PORT"
        fi
        printf "  %-12s ${GREEN}%-10s${NC} %s\n" "mongo" "running" "$mongo_detail"
    else
        printf "  %-12s ${RED}%-10s${NC} %s\n" "mongo" "stopped" ""
    fi

    # Storage (MinIO)
    if storage_is_reachable; then
        local storage_detail="port $STORAGE_API_PORT"
        if docker inspect -f '{{.State.Running}}' "$STORAGE_CONTAINER" 2>/dev/null | grep -q true; then
            storage_detail="(docker: $STORAGE_CONTAINER) API :$STORAGE_API_PORT, console :$STORAGE_CONSOLE_PORT"
        fi
        printf "  %-12s ${GREEN}%-10s${NC} %s\n" "storage" "running" "$storage_detail"
    else
        printf "  %-12s ${YELLOW}%-10s${NC} %s\n" "storage" "off" "(optional — images served from local FS)"
    fi

    # Other services
    for svc in master worker frontend; do
        if is_running "$svc"; then
            local pid
            pid=$(read_pid "$svc")
            local detail=""
            case "$svc" in
                master)   detail="PID $pid — API :8080, gRPC :9666" ;;
                worker)   detail="PID $pid — -> master $SPIDER_LAB_MASTER_GRPC" ;;
                frontend) detail="PID $pid — http://localhost:$FRONTEND_PORT" ;;
            esac
            printf "  %-12s ${GREEN}%-10s${NC} %s\n" "$svc" "running" "$detail"
        else
            printf "  %-12s ${RED}%-10s${NC} %s\n" "$svc" "stopped" ""
        fi
    done
}

cmd_logs() {
    local service="${1:-all}"

    case "$service" in
        master|worker|frontend|mongo)
            if [ -f "$LOG_DIR/$service.log" ]; then
                tail -f "$LOG_DIR/$service.log"
            elif [ "$service" = "mongo" ] && docker inspect "$MONGO_CONTAINER" &>/dev/null; then
                docker logs -f "$MONGO_CONTAINER"
            else
                log_error "No log file found for $service"
            fi
            ;;
        all)
            # Combine available log files
            local logfiles=()
            for f in "$LOG_DIR"/*.log; do
                [ -f "$f" ] && logfiles+=("$f")
            done
            if [ ${#logfiles[@]} -gt 0 ]; then
                tail -f "${logfiles[@]}"
            else
                log_error "No log files found in $LOG_DIR"
            fi
            ;;
        *)
            log_error "Unknown service: $service (use master|worker|frontend|mongo|all)"
            exit 1
            ;;
    esac
}

cmd_clean() {
    cmd_stop
    echo ""
    log_step "Cleaning build artifacts"
    rm -rf "$BIN_DIR"
    rm -rf "$FRONTEND_DIR/dist"
    rm -rf "$PID_DIR"
    rm -rf "$LOG_DIR"
    log_info "Cleaned: $BIN_DIR, $FRONTEND_DIR/dist, $PID_DIR, $LOG_DIR"
}

cmd_docker_up() {
    ensure_dirs

    # Stop bare-metal services to avoid port conflicts
    local any_stopped=false
    for svc in frontend worker master; do
        if is_running "$svc"; then
            any_stopped=true
            stop_service "$svc"
        fi
    done

    # Stop and remove bare-metal MongoDB container so docker compose can recreate it
    if command -v docker &>/dev/null && docker inspect "$MONGO_CONTAINER" &>/dev/null 2>&1; then
        log_info "Removing bare-metal MongoDB container '$MONGO_CONTAINER'..."
        docker rm -f "$MONGO_CONTAINER" >/dev/null 2>&1 || true
        any_stopped=true
    fi

    # Stop and remove bare-metal MinIO container
    if command -v docker &>/dev/null && docker inspect "$STORAGE_CONTAINER" &>/dev/null 2>&1; then
        log_info "Removing bare-metal MinIO container '$STORAGE_CONTAINER'..."
        docker rm -f "$STORAGE_CONTAINER" >/dev/null 2>&1 || true
        any_stopped=true
    fi

    # Kill any orphaned processes still holding Docker-required ports
    for port in 3000 8080 9666 9000 9001; do
        local pids
        pids=$(lsof -i :"$port" -t 2>/dev/null || true)
        if [ -n "$pids" ]; then
            log_info "Killing orphaned process(es) on port $port..."
            echo "$pids" | xargs kill 2>/dev/null || true
            sleep 0.5
            # Force-kill if still alive
            pids=$(lsof -i :"$port" -t 2>/dev/null || true)
            [ -n "$pids" ] && echo "$pids" | xargs kill -9 2>/dev/null || true
            any_stopped=true
        fi
    done

    if [ "$any_stopped" = true ]; then
        log_info "Stopped bare-metal services to free ports for Docker"
    fi

    log_step "Starting services with docker compose"
    cd "$SCRIPT_DIR"
    docker compose up --build -d
    echo ""
    log_info "Services starting via Docker..."
    docker compose ps
    echo ""
    echo -e "  ${BOLD}Dashboard:${NC}  http://localhost:3000"
    echo -e "  ${BOLD}API:${NC}        http://localhost:8080"
    echo -e "  ${BOLD}Login:${NC}      admin / admin123"
}

cmd_docker_down() {
    log_step "Stopping docker compose services"
    cd "$SCRIPT_DIR"
    docker compose down
    log_info "Docker services stopped."
}

cmd_purge_data() {
    log_step "Purging all scraped data (results collection)"

    local mongo_cmd=""
    local js='db.getSiblingDB("spider_lab").results.deleteMany({})'

    if command -v mongosh &>/dev/null; then
        mongo_cmd="mongosh --quiet --port $MONGO_PORT --eval '$js'"
    elif docker inspect "$MONGO_CONTAINER" &>/dev/null; then
        mongo_cmd="docker exec $MONGO_CONTAINER mongosh --quiet --eval '$js'"
    else
        log_error "Cannot reach MongoDB — neither mongosh nor Docker container '$MONGO_CONTAINER' available."
        return 1
    fi

    echo ""
    echo -e "  ${YELLOW}This will permanently delete ALL topics and posts from the results collection.${NC}"
    echo -n "  Continue? [y/N] "
    read -r confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        log_info "Aborted."
        return 0
    fi

    log_info "Deleting all documents from spider_lab.results ..."
    local output
    output=$(eval "$mongo_cmd" 2>&1)
    log_info "$output"
    log_info "Done. All scraped data has been removed."
}

cmd_help() {
    echo ""
    echo -e "${BOLD}  精准观察者 Precision Observer — Service Manager${NC}"
    echo ""
    echo "  Usage: $0 <command> [args]"
    echo ""
    echo "  Commands:"
    echo "    build           Build backend binaries and frontend bundle"
    echo "    start           Start all services (mongo, master, worker, frontend)"
    echo "    stop            Stop all running services"
    echo "    restart         Stop then start all services"
    echo "    status          Show status of all services"
    echo "    logs [service]  Tail logs (master|worker|frontend|mongo|all)"
    echo "    clean           Stop services and remove build artifacts"
    echo "    purge-data      Delete all scraped topics and posts from MongoDB"
    echo "    docker-up       Start all services via docker-compose"
    echo "    docker-down     Stop docker-compose services"
    echo "    help            Show this help message"
    echo ""
    echo "  Environment variables:"
    echo "    SPIDER_LAB_MONGO_URI             MongoDB URI                (default: mongodb://localhost:27017)"
    echo "    SPIDER_LAB_JWT_SECRET            JWT signing secret          (default: spider-lab-jwt-secret-key)"
    echo "    SPIDER_LAB_MAX_RUNNERS           Max concurrent tasks        (default: 8)"
    echo "    SPIDER_LAB_STORAGE_ENDPOINT      MinIO endpoint              (default: localhost:9000)"
    echo "    SPIDER_LAB_STORAGE_ACCESS_KEY    MinIO access key            (default: minioadmin)"
    echo "    SPIDER_LAB_STORAGE_SECRET_KEY    MinIO secret key            (default: minioadmin)"
    echo "    MONGO_PORT                       MongoDB port                (default: 27017)"
    echo "    FRONTEND_PORT                    Frontend port               (default: 3000)"
    echo "    STORAGE_API_PORT                 MinIO API port              (default: 9000)"
    echo "    STORAGE_CONSOLE_PORT             MinIO console port          (default: 9001)"
    echo ""
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
case "${1:-help}" in
    build)       cmd_build ;;
    start)       cmd_start ;;
    stop)        cmd_stop ;;
    restart)     cmd_restart ;;
    status)      cmd_status ;;
    logs)        cmd_logs "${2:-all}" ;;
    clean)       cmd_clean ;;
    purge-data)  cmd_purge_data ;;
    docker-up)   cmd_docker_up ;;
    docker-down) cmd_docker_down ;;
    help|--help|-h) cmd_help ;;
    *)
        log_error "Unknown command: $1"
        cmd_help
        exit 1
        ;;
esac
