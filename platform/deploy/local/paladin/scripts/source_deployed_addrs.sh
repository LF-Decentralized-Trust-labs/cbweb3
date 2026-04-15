#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  source_deployed_addrs.sh [--spoke spoke-a|spoke-b] [--file path] [--key KEY] [--raw]

Examples:
  eval "$(source_deployed_addrs.sh --spoke spoke-a)"
  source_deployed_addrs.sh --file deploy/local/paladin/spoke-a/.deployed-addrs.env --key FX_AGREEMENT_ADDRESS --raw

Options:
  --spoke   Spoke target (default: spoke-a)
  --file    Path to .deployed-addrs.env (overrides --spoke)
  --key     Return a single key from env file
  --raw     With --key, print only value (without export)
EOF
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SPOKE="spoke-a"
ENV_FILE=""
KEY=""
RAW="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --spoke)
      SPOKE="${2:-}"
      shift 2
      ;;
    --file)
      ENV_FILE="${2:-}"
      shift 2
      ;;
    --key)
      KEY="${2:-}"
      shift 2
      ;;
    --raw)
      RAW="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$ENV_FILE" ]]; then
  ENV_FILE="$ROOT_DIR/$SPOKE/.deployed-addrs.env"
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERROR: $ENV_FILE not found" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
source "$ENV_FILE"
set +a

if [[ -z "${FX_AGREEMENT_PENTE_CONTRACT_ADDRESS:-}" && -n "${FX_AGREEMENT_ADDRESS:-}" ]]; then
  FX_AGREEMENT_PENTE_CONTRACT_ADDRESS="$FX_AGREEMENT_ADDRESS"
fi

if [[ -n "$KEY" ]]; then
  VALUE="${!KEY:-}"
  if [[ -z "$VALUE" ]]; then
    echo "ERROR: $KEY not found or empty in $ENV_FILE" >&2
    exit 1
  fi
  if [[ "$RAW" == "true" ]]; then
    printf '%s\n' "$VALUE"
  else
    printf 'export %s=%q\n' "$KEY" "$VALUE"
  fi
  exit 0
fi

vars=(
  REGISTRY_CONTRACT_ADDRESS
  ZETO_FACTORY_ADDRESS
  ZETO_TOKEN_ADDRESS
  PENTE_CONTEXT_GROUP_ID
  PENTE_CONTEXT_ADDRESS
  FX_AGREEMENT_ADDRESS
  FX_AGREEMENT_DEPLOYED_AT
  FX_AGREEMENT_PENTE_CONTRACT_ADDRESS
)

for var_name in "${vars[@]}"; do
  value="${!var_name:-}"
  if [[ -n "$value" ]]; then
    printf 'export %s=%q\n' "$var_name" "$value"
  fi
done
