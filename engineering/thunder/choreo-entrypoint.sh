#!/bin/sh
# Startup script for running ThunderID on Choreo as a Web Application.
#
# Choreo web applications can't set environment variables and only offer a
# single file mount at /app/configs/config.json. This script reads that file,
# exports its settings as environment variables (used by deployment.yaml),
# writes the key material to the files ThunderID expects, and starts the server.
#
# Thunder does not serve anything from /app, so the mounted file is not
# reachable from a browser.
set -eu

CONFIG_FILE="${CONFIG_FILE:-/app/configs/config.json}"
RUNTIME_DIR="/tmp/thunderid"

if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: config file not found: $CONFIG_FILE" >&2
    exit 1
fi

# Fail early with a clear message if the file isn't valid JSON.
jq empty "$CONFIG_FILE"

get() {
    jq -er --arg k "$1" '.[$k] // empty' "$CONFIG_FILE" || {
        echo "Error: '$1' is missing from $CONFIG_FILE" >&2
        exit 1
    }
}

# Settings that deployment.yaml reads as environment variables.
for key in THUNDER_PUBLIC_URL DB_HOST DB_USERNAME DB_PASSWORD SMTP_PASSWORD ADMIN_USERNAME ADMIN_PASSWORD; do
    value="$(get "$key")"
    export "$key=$value"
done

# Key material. /opt/thunderid/config/certs and /opt/thunderid/config/secrets
# are symlinks into $RUNTIME_DIR (see Dockerfile.choreo), so this works even if
# the container's filesystem is read-only apart from /tmp.
mkdir -p "$RUNTIME_DIR/certs" "$RUNTIME_DIR/secrets"
chmod 700 "$RUNTIME_DIR"

write_file() {
    # printf '%s' writes no trailing newline; ThunderID rejects crypto.key and
    # direct_auth_secret if they end with one.
    printf '%s' "$(get "$1")" > "$2"
    chmod 600 "$2"
}

write_file CRYPTO_KEY          "$RUNTIME_DIR/certs/crypto.key"
write_file SIGNING_CERT        "$RUNTIME_DIR/certs/signing.cert"
write_file SIGNING_KEY         "$RUNTIME_DIR/certs/signing.key"
write_file ECDSA_SIGNING_CERT  "$RUNTIME_DIR/certs/ecdsa-signing.cert"
write_file ECDSA_SIGNING_KEY   "$RUNTIME_DIR/certs/ecdsa-signing.key"
write_file DIRECT_AUTH_SECRET  "$RUNTIME_DIR/secrets/direct_auth_secret"

# Set "THUNDER_BOOTSTRAP": "true" for the first deploy only, to create the
# admin user and default resources. Remove it (or set "false") afterwards.
if [ "$(jq -r '.THUNDER_BOOTSTRAP // "false"' "$CONFIG_FILE")" = "true" ]; then
    exec ./start.sh --bootstrap-and-serve
fi

exec ./start.sh
