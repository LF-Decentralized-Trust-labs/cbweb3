#!/bin/sh
set -eu

POSTGRES_DB_SPOKE_A="${POSTGRES_DB_SPOKE_A:-cbweb3_spoke_a}"
POSTGRES_DB_SPOKE_B="${POSTGRES_DB_SPOKE_B:-cbweb3_spoke_b}"
POSTGRES_DB_HUB="${POSTGRES_DB_HUB:-cbweb3_hub}"
POSTGRES_DB_KEYCLOAK="${POSTGRES_DB_KEYCLOAK:-cbweb3_keycloak}"

create_db_if_missing() {
  db_name="$1"
  if [ -z "$db_name" ]; then
    return 0
  fi

  db_exists="$(psql -U "$POSTGRES_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='${db_name}'")"
  if [ "$db_exists" != "1" ]; then
    echo "Creating database: ${db_name}"
    psql -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE \"${db_name}\";"
  else
    echo "Database already exists: ${db_name}"
  fi
}

create_db_if_missing "$POSTGRES_DB_SPOKE_A"
create_db_if_missing "$POSTGRES_DB_SPOKE_B"
create_db_if_missing "$POSTGRES_DB_HUB"
create_db_if_missing "$POSTGRES_DB_KEYCLOAK"
