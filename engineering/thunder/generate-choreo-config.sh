#!/bin/sh
# Builds config.json for the Choreo file mount at /app/configs/config.json.
#
# Usage (from engineering/thunder):
#   ./generate-choreo-config.sh [env-file] [output-file]
#   defaults: thunder.env -> config.json
#
# Reads settings from the env file (KEY=VALUE lines, see config.example.json
# for the keys) and adds ThunderID's key material from choreo-keys/.
# The keys are generated on the first run and reused afterwards: they must stay
# the same between deploys, or issued tokens and encrypted data stop working.
# Keep choreo-keys/ somewhere safe and never commit it or config.json.
#
# Requires: openssl, jq
set -eu

cd "$(dirname "$0")"
ENV_FILE="${1:-thunder.env}"
OUT_FILE="${2:-config.json}"
KEY_DIR="choreo-keys"

for cmd in openssl jq; do
    command -v "$cmd" >/dev/null || { echo "Error: $cmd is required" >&2; exit 1; }
done
[ -f "$ENV_FILE" ] || { echo "Error: env file not found: $ENV_FILE" >&2; exit 1; }

# Read KEY=VALUE lines (ignoring blanks and comments) into a JSON object.
settings="$(grep -vE '^[[:space:]]*(#|$)' "$ENV_FILE" | sed 's/\r$//' |
    jq -Rn '[inputs | capture("^(?<k>[A-Za-z_][A-Za-z0-9_]*)=(?<v>.*)$")] | map({(.k): .v}) | add // {}')"

for key in THUNDER_PUBLIC_URL DB_HOST DB_USERNAME DB_PASSWORD SMTP_PASSWORD ADMIN_USERNAME ADMIN_PASSWORD; do
    [ -n "$(printf '%s' "$settings" | jq -r --arg k "$key" '.[$k] // empty')" ] ||
        { echo "Error: $key is missing or empty in $ENV_FILE" >&2; exit 1; }
done

# ThunderID requires a DNS subjectAltName on its signing certificates.
host="$(printf '%s' "$settings" | jq -r '.THUNDER_PUBLIC_URL' | sed -E 's#^[a-z]+://##; s#[:/].*$##')"
san="subjectAltName=DNS:${host}"
subj="/O=ThunderID/OU=ThunderID/CN=${host}"

mkdir -p "$KEY_DIR"
chmod 700 "$KEY_DIR"
umask 077

if [ ! -f "$KEY_DIR/signing.key" ]; then
    echo "Generating RSA signing key for $host"
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout "$KEY_DIR/signing.key" -out "$KEY_DIR/signing.cert" \
        -subj "$subj" -addext "$san" 2>/dev/null
fi
if [ ! -f "$KEY_DIR/ecdsa-signing.key" ]; then
    echo "Generating ECDSA signing key for $host"
    openssl ecparam -name prime256v1 -genkey -noout -out "$KEY_DIR/ecdsa-signing.key"
    openssl req -new -x509 -nodes -days 3650 -key "$KEY_DIR/ecdsa-signing.key" \
        -out "$KEY_DIR/ecdsa-signing.cert" -subj "$subj" -addext "$san" 2>/dev/null
fi
[ -f "$KEY_DIR/crypto.key" ] || printf '%s' "$(openssl rand -hex 32)" > "$KEY_DIR/crypto.key"
[ -f "$KEY_DIR/direct_auth_secret" ] || printf '%s' "$(openssl rand -hex 32)" > "$KEY_DIR/direct_auth_secret"

printf '%s' "$settings" | jq \
    --rawfile crypto "$KEY_DIR/crypto.key" \
    --rawfile direct "$KEY_DIR/direct_auth_secret" \
    --rawfile scert  "$KEY_DIR/signing.cert" \
    --rawfile skey   "$KEY_DIR/signing.key" \
    --rawfile ecert  "$KEY_DIR/ecdsa-signing.cert" \
    --rawfile ekey   "$KEY_DIR/ecdsa-signing.key" \
    '. + {
        CRYPTO_KEY: $crypto,
        DIRECT_AUTH_SECRET: $direct,
        SIGNING_CERT: $scert,
        SIGNING_KEY: $skey,
        ECDSA_SIGNING_CERT: $ecert,
        ECDSA_SIGNING_KEY: $ekey
    }' > "$OUT_FILE"

echo "Wrote $OUT_FILE. Paste its contents into the Choreo file mount at /app/configs/config.json."
