#!/usr/bin/env bash
#
# Spider Lab - Worker Node Setup Script
#
# This script installs the runtime dependencies required for the worker node
# to execute Python, Node.js, and Go spiders.
#
# Usage:
#   sudo ./scripts/setup-worker.sh
#
# Supported: Ubuntu/Debian, CentOS/RHEL/Fedora, Alpine, macOS
#
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; }

check_cmd() { command -v "$1" &>/dev/null; }

# Detect OS
detect_os() {
  if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "macos"
  elif check_cmd apt-get; then
    echo "debian"
  elif check_cmd dnf; then
    echo "fedora"
  elif check_cmd yum; then
    echo "centos"
  elif check_cmd apk; then
    echo "alpine"
  else
    echo "unknown"
  fi
}

OS=$(detect_os)
info "Detected OS: $OS"

# ===================== Python =====================
install_python() {
  if check_cmd python3; then
    info "Python3 already installed: $(python3 --version)"
  else
    info "Installing Python3..."
    case $OS in
      debian)  apt-get update && apt-get install -y python3 python3-pip python3-venv ;;
      fedora)  dnf install -y python3 python3-pip ;;
      centos)  yum install -y python3 python3-pip ;;
      alpine)  apk add --no-cache python3 py3-pip ;;
      macos)   brew install python3 ;;
      *)       error "Cannot install Python on this OS"; return 1 ;;
    esac
  fi

  if ! check_cmd pip3 && ! check_cmd pip; then
    info "Installing pip..."
    python3 -m ensurepip --upgrade 2>/dev/null || true
  fi
  info "pip: $(pip3 --version 2>/dev/null || pip --version 2>/dev/null || echo 'not found')"
}

# ===================== Node.js =====================
install_nodejs() {
  if check_cmd node; then
    info "Node.js already installed: $(node --version)"
  else
    info "Installing Node.js..."
    case $OS in
      debian)  apt-get update && apt-get install -y nodejs npm ;;
      fedora)  dnf install -y nodejs npm ;;
      centos)  yum install -y nodejs npm ;;
      alpine)  apk add --no-cache nodejs npm ;;
      macos)   brew install node ;;
      *)       error "Cannot install Node.js on this OS"; return 1 ;;
    esac
  fi

  if check_cmd npm; then
    info "npm: $(npm --version)"
  else
    warn "npm not found after Node.js installation"
  fi
}

# ===================== Go =====================
install_go() {
  if check_cmd go; then
    info "Go already installed: $(go version)"
  else
    info "Installing Go 1.22..."
    case $OS in
      debian|fedora|centos)
        local GO_VERSION="1.22.6"
        local ARCH
        ARCH=$(uname -m)
        case $ARCH in
          x86_64)  ARCH="amd64" ;;
          aarch64) ARCH="arm64" ;;
        esac
        curl -sL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" | tar -C /usr/local -xzf -
        ln -sf /usr/local/go/bin/go /usr/local/bin/go
        ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt
        ;;
      alpine)  apk add --no-cache go ;;
      macos)   brew install go ;;
      *)       error "Cannot install Go on this OS"; return 1 ;;
    esac
  fi
  info "Go: $(go version 2>/dev/null || echo 'not found')"
}

# ===================== Playwright =====================
install_playwright() {
  if ! check_cmd python3; then
    warn "Python3 not installed, skipping Playwright"
    return 0
  fi

  info "Installing Playwright and Chromium browser..."

  # Install playwright Python package
  pip3 install --quiet playwright 2>/dev/null || pip install --quiet playwright 2>/dev/null || {
    warn "Failed to install playwright pip package"
    return 0
  }

  # Playwright system dependencies (Linux only)
  if [[ "$OSTYPE" != "darwin"* ]]; then
    case $OS in
      debian)
        # Chromium deps for headless mode
        apt-get install -y --no-install-recommends \
          libnss3 libnspr4 libatk1.0-0 libatk-bridge2.0-0 libcups2 \
          libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 libxfixes3 \
          libxrandr2 libgbm1 libpango-1.0-0 libcairo2 libasound2 \
          libatspi2.0-0 libwayland-client0 2>/dev/null || true
        ;;
      fedora)
        dnf install -y nss nspr atk at-spi2-atk cups-libs libdrm \
          libxkbcommon libXcomposite libXdamage libXrandr mesa-libgbm \
          pango cairo alsa-lib 2>/dev/null || true
        ;;
      centos)
        yum install -y nss nspr atk at-spi2-atk cups-libs libdrm \
          libxkbcommon libXcomposite libXdamage libXrandr mesa-libgbm \
          pango cairo alsa-lib 2>/dev/null || true
        ;;
      alpine)
        apk add --no-cache nss nspr mesa-gbm libdrm pango cairo \
          alsa-lib eudev ttf-freefont 2>/dev/null || true
        ;;
    esac
  fi

  # Install Chromium browser binary
  python3 -m playwright install chromium 2>/dev/null || {
    warn "Failed to install Playwright Chromium. Run manually: python3 -m playwright install chromium"
    return 0
  }

  info "Playwright + Chromium installed"
}

# ===================== Additional Tools =====================
install_extras() {
  info "Installing additional tools (git, curl)..."
  case $OS in
    debian)  apt-get update && apt-get install -y git curl ca-certificates ;;
    fedora)  dnf install -y git curl ca-certificates ;;
    centos)  yum install -y git curl ca-certificates ;;
    alpine)  apk add --no-cache git curl ca-certificates ;;
    macos)   : ;; # git and curl come with macOS
  esac
}

# ===================== Main =====================
main() {
  echo ""
  echo "========================================="
  echo "  Spider Lab - Worker Node Setup"
  echo "========================================="
  echo ""

  install_extras
  install_python
  install_nodejs
  install_go
  install_playwright

  echo ""
  echo "========================================="
  info "Worker runtime setup complete!"
  echo "========================================="
  echo ""
  echo "  Installed runtimes:"
  echo "    Python:     $(python3 --version 2>/dev/null || echo 'NOT FOUND')"
  echo "    Node.js:    $(node --version 2>/dev/null || echo 'NOT FOUND')"
  echo "    Go:         $(go version 2>/dev/null || echo 'NOT FOUND')"
  echo "    pip:        $(pip3 --version 2>/dev/null || echo 'NOT FOUND')"
  echo "    npm:        $(npm --version 2>/dev/null || echo 'NOT FOUND')"
  echo "    Playwright: $(python3 -m playwright --version 2>/dev/null || echo 'NOT FOUND')"
  echo ""
  echo "  The worker node can now execute Python, Node.js, and Go spiders."
  echo "  Pre-built spiders use Playwright for anti-bot browser automation."
  echo "  Dependencies are auto-installed from requirements.txt / package.json / go.mod."
  echo ""
}

main "$@"
