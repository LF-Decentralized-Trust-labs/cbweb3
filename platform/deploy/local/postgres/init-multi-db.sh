#!/bin/sh
set -eu

POSTGRES_DB_BANK_A="${POSTGRES_DB_BANK_A:-cbweb3_bank_a}"
POSTGRES_DB_BANK_B="${POSTGRES_DB_BANK_B:-cbweb3_bank_b}"
POSTGRES_DB_CENTRAL_BANK="${POSTGRES_DB_CENTRAL_BANK:-cbweb3_central_bank}"
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

create_db_if_missing "$POSTGRES_DB_BANK_A"
create_db_if_missing "$POSTGRES_DB_BANK_B"
create_db_if_missing "$POSTGRES_DB_CENTRAL_BANK"
create_db_if_missing "$POSTGRES_DB_KEYCLOAK"
