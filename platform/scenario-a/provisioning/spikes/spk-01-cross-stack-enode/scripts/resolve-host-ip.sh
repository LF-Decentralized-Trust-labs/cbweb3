#!/usr/bin/env bash
# resolve-host-ip.sh — cross-platform host IP for enode/bundle advertisement.
# Override: export HOST_IP=<addr>

set -euo pipefail

if [ -n "${HOST_IP:-}" ]; then
    echo "${HOST_IP}"
    exit 0
fi

# Linux (GNU hostname)
if ip=$(hostname -I 2>/dev/null | awk '{print $1}'); [ -n "${ip}" ]; then
    echo "${ip}"
    exit 0
fi

# macOS / BSD: default-route interface, then common interfaces
if command -v route >/dev/null 2>&1 && command -v ipconfig >/dev/null 2>&1; then
    iface=$(route -n get default 2>/dev/null | awk '/interface: / {print $2; exit}')
    if [ -n "${iface}" ]; then
        ip=$(ipconfig getifaddr "${iface}" 2>/dev/null || true)
        if [ -n "${ip}" ]; then
            echo "${ip}"
            exit 0
        fi
    fi
    for iface in en0 en1; do
        ip=$(ipconfig getifaddr "${iface}" 2>/dev/null || true)
        if [ -n "${ip}" ]; then
            echo "${ip}"
            exit 0
        fi
    done
fi

exit 1
