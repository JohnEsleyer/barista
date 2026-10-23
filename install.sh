#!/usr/bin/env bash
#
# Barista Installer
# https://github.com/JohnEsleyer/barista
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/JohnEsleyer/barista/main/install.sh | bash
#   or locally:
#   ./install.sh
#

set -euo pipefail

# --- Color and Styling ---
if [[ -t 1 ]] && [[ -z "${NO_COLOR:-}" ]]; then
    BOLD="\033[1m"
    DIM="\033[2m"
    GREEN="\033[0;32m"
    CYAN="\033[0;36m"
    YELLOW="\033[0;33m"
    RED="\033[0;31m"
    RESET="\033[0m"
else
    BOLD=""
    DIM=""
    GREEN=""
    CYAN=""
    YELLOW=""
    RED=""
    RESET=""
fi

info() {
    echo -e "${CYAN}==>${RESET} ${BOLD}$*${RESET}"
}

success() {
    echo -e "${GREEN}==>${RESET} ${BOLD}$*${RESET}"
}

warn() {
    echo -e "${YELLOW}warning:${RESET} $*"
}

error() {
    echo -e "${RED}error:${RESET} $*" >&2
}

abort() {
    error "$@"
    exit 1
}

# --- Banner ---
echo -e "${BOLD}"
echo "    (  )   (   )  )"
echo "     ) (   )  (  ("
echo "     ( )  (    ) )"
echo "    _____________"
echo "   <_____________> ===[ ☕ Barista ]==="
echo "   |             |/ ]"
echo "   |   BARISTA   | /"
echo "   |_____________|/"
echo -e "${RESET}"
echo -e "${DIM}Compile JVM bytecode into stripped static Go binaries.${RESET}"
echo ""

# --- System Detection ---
OS_RAW="$(uname -s)"
case "${OS_RAW}" in
    Linux*)   OS="linux" ;;
    Darwin*)  OS="darwin" ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *) abort "Unsupported operating system: ${OS_RAW}" ;;
esac

ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
    x86_64|amd64)   ARCH="amd64" ;;
    arm64|aarch64)  ARCH="arm64" ;;
    armv7*|armhf)   ARCH="arm" ;;
    i386|i686)      ARCH="386" ;;
    *) abort "Unsupported architecture: ${ARCH_RAW}" ;;
esac

info "Detected platform: ${BOLD}${OS}/${ARCH}${RESET}"

# --- Determine Install Directory ---
if [[ -n "${INSTALL_DIR:-}" ]]; then
    TARGET_DIR="${INSTALL_DIR}"
elif [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    TARGET_DIR="/usr/local/bin"
elif [[ -w "/usr/local/bin" ]]; then
    TARGET_DIR="/usr/local/bin"
else
    TARGET_DIR="${HOME}/.local/bin"
fi

BIN_NAME="barista"
if [[ "${OS}" == "windows" ]]; then
    BIN_NAME="barista.exe"
fi
TARGET_BIN="${TARGET_DIR}/${BIN_NAME}"

mkdir -p "${TARGET_DIR}"

# --- Build / Download ---
REPO_ROOT=""
# Check if current directory or script directory is barista repo
if [[ -f "go.mod" ]] && grep -q "github.com/JohnEsleyer/barista" "go.mod" 2>/dev/null; then
    REPO_ROOT="$(pwd)"
elif [[ -f "$(dirname "$0")/go.mod" ]] && grep -q "github.com/JohnEsleyer/barista" "$(dirname "$0")/go.mod" 2>/dev/null; then
    REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
fi

TEMP_DIR=""
cleanup() {
    if [[ -n "${TEMP_DIR}" && -d "${TEMP_DIR}" ]]; then
        rm -rf "${TEMP_DIR}"
    fi
}
trap cleanup EXIT

if [[ -n "${REPO_ROOT}" ]]; then
    info "Found local Barista source at ${REPO_ROOT}"
    if ! command -v go >/dev/null 2>&1; then
        abort "Go is required to build Barista from source, but 'go' was not found on PATH."
    fi

    info "Compiling native Barista binary from source..."
    (
        cd "${REPO_ROOT}"
        go build -trimpath -ldflags="-s -w" -o "${TARGET_BIN}" ./cmd/barista
    )
else
    # Remote execution or standalone script: attempt release download, then source build
    DOWNLOAD_SUCCESS=false
    RELEASE_URL="https://github.com/JohnEsleyer/barista/releases/latest/download/barista_${OS}_${ARCH}"
    if [[ "${OS}" == "windows" ]]; then
        RELEASE_URL="${RELEASE_URL}.exe"
    fi

    TEMP_DIR="$(mktemp -d -t barista-install-XXXXXX)"
    DL_BIN="${TEMP_DIR}/${BIN_NAME}"

    if command -v curl >/dev/null 2>&1; then
        info "Attempting to download prebuilt release from GitHub..."
        if curl -fsSL "${RELEASE_URL}" -o "${DL_BIN}" 2>/dev/null; then
            chmod +x "${DL_BIN}"
            cp "${DL_BIN}" "${TARGET_BIN}"
            DOWNLOAD_SUCCESS=true
        fi
    fi

    if [[ "${DOWNLOAD_SUCCESS}" == "false" ]]; then
        info "Prebuilt binary not found for ${OS}/${ARCH}; building from source repository..."
        if ! command -v go >/dev/null 2>&1; then
            abort "Neither a prebuilt binary nor Go was found. Please install Go 1.24+ (https://golang.org) to build Barista."
        fi

        if ! command -v git >/dev/null 2>&1; then
            abort "Git is required to clone the repository for installation."
        fi

        SRC_DIR="${TEMP_DIR}/src"
        info "Cloning Barista repository..."
        git clone --depth 1 https://github.com/JohnEsleyer/barista.git "${SRC_DIR}" >/dev/null 2>&1
        
        info "Compiling native Barista static binary..."
        (
            cd "${SRC_DIR}"
            go build -trimpath -ldflags="-s -w" -o "${TARGET_BIN}" ./cmd/barista
        )
    fi
fi

chmod +x "${TARGET_BIN}"

# --- Verification ---
if ! "${TARGET_BIN}" version >/dev/null 2>&1; then
    abort "Installed binary at ${TARGET_BIN} failed verification check."
fi

VERSION="$("${TARGET_BIN}" version)"

echo ""
success "Barista installed successfully to ${BOLD}${TARGET_BIN}${RESET}"
echo -e "   Version: ${GREEN}${VERSION}${RESET}"
echo ""

# --- PATH Check ---
case ":${PATH}:" in
    *:"${TARGET_DIR}":*) ;;
    *)
        warn "${TARGET_DIR} is not in your PATH."
        echo ""
        echo "   Add it to your profile by running:"
        echo ""
        SHELL_NAME="$(basename "${SHELL:-bash}")"
        if [[ "${SHELL_NAME}" == "zsh" ]]; then
            echo -e "     ${CYAN}echo 'export PATH=\"${TARGET_DIR}:\$PATH\"' >> ~/.zshrc${RESET}"
            echo -e "     ${CYAN}source ~/.zshrc${RESET}"
        elif [[ "${SHELL_NAME}" == "fish" ]]; then
            echo -e "     ${CYAN}fish_add_path ${TARGET_DIR}${RESET}"
        else
            echo -e "     ${CYAN}echo 'export PATH=\"${TARGET_DIR}:\$PATH\"' >> ~/.bashrc${RESET}"
            echo -e "     ${CYAN}source ~/.bashrc${RESET}"
        fi
        echo ""
        ;;
esac

# --- Quick Start ---
echo -e "${BOLD}Quick Start:${RESET}"
echo -e "  Compile and run a controller:  ${CYAN}barista serve OrderController.class${RESET}"
echo -e "  Build a static binary:         ${CYAN}barista build -o my-service OrderController.class${RESET}"
echo -e "  Inspect intermediate representation: ${CYAN}barista grind OrderController.class${RESET}"
echo ""
echo -e "${GREEN}☕ Freshly brewed! Happy coding with Barista.${RESET}"
