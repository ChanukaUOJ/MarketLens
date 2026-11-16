#!/bin/sh
# One-off setup of ThunderID's four PostgreSQL databases for the Choreo
# deployment (what the thunder-setup container does for docker-compose).
#
# Usage (from engineering/thunder):
#   ./init-choreo-db.sh [config-file]      default: config.json
#
# For each database in deployment-choreo.yaml it:
#   - creates the database if it doesn't exist
#   - loads ThunderID's schema (dbscripts/<db>/postgres.sql from the image)
#     only if the database has no tables yet
# so it is safe to run again. It never drops or changes existing tables.
#
# Requires: docker, jq. psql runs inside a postgres container.
set -eu

cd "$(dirname "$0")"
CONFIG_FILE="${1:-config.json}"
THUNDER_IMAGE="ghcr.io/thunder-id/thunderid:1.0.0"
PSQL_IMAGE="postgres:16-alpine"

for cmd in docker jq; do
    command -v "$cmd" >/dev/null || { echo "Error: $cmd is required" >&2; exit 1; }
done
[ -f "$CONFIG_FILE" ] || { echo "Error: $CONFIG_FILE not found (run ./generate-choreo-config.sh first)" >&2; exit 1; }

DB_HOST="$(jq -er '.DB_HOST' "$CONFIG_FILE")"
DB_USERNAME="$(jq -er '.DB_USERNAME' "$CONFIG_FILE")"
DB_PASSWORD="$(jq -er '.DB_PASSWORD' "$CONFIG_FILE")"
# Neon's pooled endpoint (-pooler) can't run CREATE DATABASE reliably; use the
# direct endpoint for setup. ThunderID itself can keep using the pooled one.
SETUP_HOST="$(printf '%s' "$DB_HOST" | sed 's/-pooler\././')"

# database name in deployment-choreo.yaml : schema folder in the image
DATABASES="thunderid_config:configdb thunderid_runtime_transient:runtime_transient thunderid_entity:entitydb thunderid_runtime_persistent:runtime_persistent"

SQL_DIR="$(mktemp -d)"
trap 'rm -rf "$SQL_DIR"' EXIT
cid="$(docker create "$THUNDER_IMAGE")"
docker cp -q "$cid:/opt/thunderid/dbscripts" "$SQL_DIR/"
docker rm "$cid" >/dev/null

psql_run() {
    # $1 = database, rest = psql args. The password is passed through the
    # environment, never on the command line.
    # (sh has no local variables, so this name must not clash with the loop's.)
    _psql_db="$1"; shift
    docker run --rm -i -e PGPASSWORD="$DB_PASSWORD" "$PSQL_IMAGE" \
        psql "host=$SETUP_HOST port=5432 dbname=$_psql_db user=$DB_USERNAME sslmode=require" \
        -v ON_ERROR_STOP=1 -q "$@"
}

# Find a database to connect to for CREATE DATABASE (Neon's default is neondb).
ADMIN_DB=""
for candidate in postgres neondb defaultdb; do
    if psql_run "$candidate" -Atc "SELECT 1" >/dev/null 2>&1; then
        ADMIN_DB="$candidate"
        break
    fi
done
[ -n "$ADMIN_DB" ] || { echo "Error: could not connect to $SETUP_HOST as $DB_USERNAME" >&2; exit 1; }
echo "Connected to $SETUP_HOST (via database '$ADMIN_DB')"

for entry in $DATABASES; do
    db="${entry%%:*}"
    schema="${entry##*:}"

    exists="$(psql_run "$ADMIN_DB" -Atc "SELECT 1 FROM pg_database WHERE datname = '$db'")"
    if [ "$exists" = "1" ]; then
        echo "$db: database exists"
    else
        psql_run "$ADMIN_DB" -c "CREATE DATABASE \"$db\""
        echo "$db: database created"
    fi

    tables="$(psql_run "$db" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'")"
    if [ "$tables" -gt 0 ]; then
        echo "$db: already has $tables tables, schema not reloaded"
    else
        psql_run "$db" < "$SQL_DIR/dbscripts/$schema/postgres.sql" >/dev/null
        tables="$(psql_run "$db" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'")"
        echo "$db: schema loaded ($tables tables)"
    fi
done

echo "Done. Redeploy Thunder on Choreo (with THUNDER_BOOTSTRAP=true for the first run)."
