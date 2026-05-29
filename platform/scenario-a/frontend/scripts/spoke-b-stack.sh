#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/frontend/docker-compose.spoke-b.yml"
ENV_FILE="${ROOT_DIR}/frontend/.env"
ENV_EXAMPLE_FILE="${ROOT_DIR}/frontend/.env.example"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "Compose file not found: ${COMPOSE_FILE}" >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  if [[ -f "${ENV_EXAMPLE_FILE}" ]]; then
    cp "${ENV_EXAMPLE_FILE}" "${ENV_FILE}"
    echo "Created ${ENV_FILE} from ${ENV_EXAMPLE_FILE}"
  else
    echo "Environment file not found: ${ENV_FILE}" >&2
    echo "Fallback file not found: ${ENV_EXAMPLE_FILE}" >&2
    exit 1
  fi
fi

ACTION="${1:-up}"

case "${ACTION}" in
  up)
    (
      cd "${ROOT_DIR}/frontend"
      npm install
    )
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up --build -d
    ;;
  down)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" down --remove-orphans
    ;;
  logs)
    docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" logs -f
    ;;
  *)
    echo "Usage: $0 {up|down|logs}" >&2
    exit 1
    ;;
esac