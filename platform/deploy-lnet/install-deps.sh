#!/usr/bin/env bash
#
# install-deps.sh — install the HOST prerequisites for the cbweb3 sample deploys.
#
# Dependencies (authoritative list from scenario-a/samples/README.md and
# scenario-b/samples/README.md):
#   • Docker + Docker Compose v2   — every entity's Besu/backend/frontend/NOC stack
#   • Go 1.26+                     — building the toolkit CLIs (cbweb3 / cbweb3b)
#   • Foundry (forge / cast)       — compiling and deploying the Solidity contracts
#   • jq, openssl, curl            — verification, TLS material, RPC/health probes
# (Node.js is NOT a host prerequisite: the launcher, relay and frontend images are
#  built inside Docker.)
#
# Docker is installed via Docker's official CONVENIENCE SCRIPT (https://get.docker.com),
# which also pulls in the Compose v2 plugin. Everything is idempotent: a dependency is
# skipped when already present at a sufficient version.
#
# Usage:
#   ./install-deps.sh           # install whatever is missing
#   ./install-deps.sh --check   # only report present/missing (make no changes)
#
# Linux only. Primary path is Debian/Ubuntu (apt); dnf/yum, pacman and zypper are
# supported for the OS packages (jq/openssl/curl). Docker/Go/Foundry use their own
# official installers and work across distros.
#
set -euo pipefail

GO_MIN_MAJOR=1
GO_MIN_MINOR=26
GO_INSTALL_VERSION="1.26.0"   # installed from go.dev/dl when Go is absent/too old

CHECK_ONLY=false
[[ "${1:-}" == "--check" ]] && CHECK_ONLY=true

# ---- helpers -----------------------------------------------------------------
c_cyan=$'\033[1;36m'; c_green=$'\033[1;32m'; c_yellow=$'\033[1;33m'; c_red=$'\033[1;31m'; c_off=$'\033[0m'
log()  { printf '\n%s[deps]%s %s\n' "$c_cyan" "$c_off" "$*"; }
ok()   { printf '  %s✓%s %s\n' "$c_green" "$c_off" "$*"; }
warn() { printf '  %s!%s %s\n' "$c_yellow" "$c_off" "$*"; }
err()  { printf '  %s✗%s %s\n' "$c_red" "$c_off" "$*"; }
have() { command -v "$1" >/dev/null 2>&1; }

if [[ "$(uname -s)" != "Linux" ]]; then
  err "this installer targets Linux only (detected $(uname -s))."; exit 1
fi

# sudo wrapper (empty when already root)
if [[ "$(id -u)" -eq 0 ]]; then SUDO=""; else
  if have sudo; then SUDO="sudo"; else
    err "not root and 'sudo' not found — re-run as root or install sudo."; exit 1
  fi
fi

# package manager detection (for the OS packages: jq, openssl, curl)
PM=""
for m in apt-get dnf yum pacman zypper; do have "$m" && { PM="$m"; break; }; done

pm_install() { # pm_install <pkg...>
  case "$PM" in
    apt-get) $SUDO apt-get update -y && $SUDO apt-get install -y "$@" ;;
    dnf|yum) $SUDO "$PM" install -y "$@" ;;
    pacman)  $SUDO pacman -Sy --noconfirm "$@" ;;
    zypper)  $SUDO zypper install -y "$@" ;;
    *) err "no supported package manager found (apt/dnf/yum/pacman/zypper)"; return 1 ;;
  esac
}

arch_go() { case "$(uname -m)" in x86_64) echo amd64 ;; aarch64|arm64) echo arm64 ;; *) uname -m ;; esac; }

go_ok() { # true when a Go >= GO_MIN is on PATH
  have go || return 1
  local v; v="$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | tr -d 'go')"
  local maj="${v%%.*}" min="${v##*.}"
  [[ -z "$v" ]] && return 1
  (( maj > GO_MIN_MAJOR )) && return 0
  (( maj == GO_MIN_MAJOR && min >= GO_MIN_MINOR ))
}

# ---- installers --------------------------------------------------------------
install_os_pkgs() { # jq, openssl, curl
  local want=(jq openssl curl) missing=()
  for p in "${want[@]}"; do have "$p" || missing+=("$p"); done
  # ca-certificates is needed for the https downloads below
  have update-ca-certificates || have trust || missing+=(ca-certificates)
  if [[ ${#missing[@]} -eq 0 ]]; then ok "jq, openssl, curl already present"; return; fi
  log "installing OS packages: ${missing[*]}"
  pm_install "${missing[@]}"
  ok "OS packages installed"
}

install_docker() {
  if have docker; then
    ok "docker present: $(docker --version 2>/dev/null || echo '?')"
  else
    log "installing Docker via the official convenience script (https://get.docker.com)"
    local tmp; tmp="$(mktemp)"
    curl -fsSL https://get.docker.com -o "$tmp"
    $SUDO sh "$tmp"
    rm -f "$tmp"
    ok "docker installed"
  fi
  # Compose v2 plugin ships with Docker CE; verify it resolves.
  if docker compose version >/dev/null 2>&1; then
    ok "docker compose v2: $(docker compose version --short 2>/dev/null || echo present)"
  else
    warn "docker compose v2 plugin not detected — installing docker-compose-plugin"
    case "$PM" in apt-get|dnf|yum|zypper) pm_install docker-compose-plugin || warn "install docker-compose-plugin manually" ;; *) warn "install the Compose v2 plugin for your distro" ;; esac
  fi
  # enable the daemon + let the current user talk to it without sudo (next login)
  if have systemctl; then $SUDO systemctl enable --now docker >/dev/null 2>&1 || warn "could not enable the docker service"; fi
  if [[ -n "${SUDO}" ]] && ! id -nG "$USER" 2>/dev/null | grep -qw docker; then
    $SUDO groupadd -f docker >/dev/null 2>&1 || true
    $SUDO usermod -aG docker "$USER" || warn "could not add $USER to the docker group"
    warn "added $USER to the 'docker' group — log out/in (or run 'newgrp docker') to use docker without sudo"
  fi
}

install_go() {
  if go_ok; then ok "go present: $(go version | awk '{print $3}')"; return; fi
  have go && warn "go $(go version | awk '{print $3}') is older than ${GO_MIN_MAJOR}.${GO_MIN_MINOR} — installing ${GO_INSTALL_VERSION}"
  log "installing Go ${GO_INSTALL_VERSION} (official tarball → /usr/local/go)"
  local tgz="go${GO_INSTALL_VERSION}.linux-$(arch_go).tar.gz"
  local tmp; tmp="$(mktemp -d)"
  curl -fsSL "https://go.dev/dl/${tgz}" -o "${tmp}/${tgz}"
  $SUDO rm -rf /usr/local/go
  $SUDO tar -C /usr/local -xzf "${tmp}/${tgz}"
  rm -rf "$tmp"
  echo 'export PATH=$PATH:/usr/local/go/bin' | $SUDO tee /etc/profile.d/go.sh >/dev/null
  export PATH=$PATH:/usr/local/go/bin
  ok "go installed: $(/usr/local/go/bin/go version | awk '{print $3}') (open a new shell or 'source /etc/profile.d/go.sh')"
}

install_foundry() {
  if have forge && have cast; then ok "foundry present: $(forge --version 2>/dev/null | head -1)"; return; fi
  log "installing Foundry (foundryup convenience installer)"
  curl -fsSL https://foundry.paradigm.xyz | bash >/dev/null 2>&1 || true
  local fup="$HOME/.foundry/bin/foundryup"
  if [[ -x "$fup" ]]; then
    "$fup"
    export PATH="$PATH:$HOME/.foundry/bin"
    ok "foundry installed: $($HOME/.foundry/bin/forge --version 2>/dev/null | head -1) (open a new shell to get forge/cast on PATH)"
  else
    err "foundryup was not installed to ~/.foundry/bin — install Foundry manually (https://getfoundry.sh)"
  fi
}

# ---- report ------------------------------------------------------------------
report() {
  log "verification"
  have docker            && ok "docker           $(docker --version 2>/dev/null | awk '{print $3}' | tr -d ,)"           || err "docker           missing"
  docker compose version >/dev/null 2>&1 && ok "docker compose   $(docker compose version --short 2>/dev/null)"          || err "docker compose   missing"
  go_ok                  && ok "go               $(go version 2>/dev/null | awk '{print $3}')"                            || err "go 1.${GO_MIN_MINOR}+       missing/old"
  have forge             && ok "forge            $(forge --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+[^ ]*' | head -1)"  || err "forge            missing (open a new shell if just installed)"
  have jq                && ok "jq               $(jq --version 2>/dev/null)"                                             || err "jq               missing"
  have openssl           && ok "openssl          $(openssl version 2>/dev/null | awk '{print $2}')"                       || err "openssl          missing"
  have curl              && ok "curl             $(curl --version 2>/dev/null | head -1 | awk '{print $2}')"              || err "curl             missing"
}

# ---- main --------------------------------------------------------------------
if $CHECK_ONLY; then
  log "check-only mode (no changes will be made)"
  report
  exit 0
fi

log "installing host prerequisites for the cbweb3 sample deploys (pkg mgr: ${PM:-none})"
install_os_pkgs
install_docker
install_go
install_foundry
report

log "done. Notes:"
cat <<EOF
  • Docker group / PATH changes take effect in a NEW shell (or run 'newgrp docker',
    'source /etc/profile.d/go.sh', and add ~/.foundry/bin to PATH).
  • Besu image hyperledger/besu:25.8.0 is pulled automatically on the first node 'up'.
  • Next: ./deploy-minimal.sh   (minimal A+B launcher validation)
          or scenario-b/samples/deploy-all.sh / scenario-a/samples/deploy-three.sh
EOF