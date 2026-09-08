#!/usr/bin/env bash
# Generates a throwaway self-signed CA + server + client cert/key pair (PEM)
# for the mTLS listener in docker-compose.integration.yaml. Re-run any time;
# output is gitignored and safe to regenerate.
set -euo pipefail

# Git-Bash-for-Windows rewrites a leading "/CN=..." as a filesystem path;
# MSYS_NO_PATHCONV disables that mangling. No-op on Linux/macOS.
export MSYS_NO_PATHCONV=1

OUT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.kafka-certs"
mkdir -p "$OUT_DIR"
cd "$OUT_DIR"

openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
  -keyout ca.key -out ca.pem -subj "/CN=kafka-integration-test-ca"

openssl req -newkey rsa:2048 -nodes -keyout server.key -out server.csr \
  -subj "/CN=localhost"
printf "subjectAltName=DNS:localhost,DNS:kafka" > server.ext
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out server.pem -days 3650 -extfile server.ext
rm -f server.ext

openssl req -newkey rsa:2048 -nodes -keyout client.key -out client.csr \
  -subj "/CN=testuser"
openssl x509 -req -in client.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
  -out client.pem -days 3650

rm -f server.csr client.csr ca.srl

# The apache/kafka image's own entrypoint script only wires up SSL from a
# PKCS12/JKS keystore dropped in /etc/kafka/secrets (KAFKA_SSL_KEYSTORE_FILENAME
# + a password-file sidecar) — it doesn't read raw PEM values from env, so we
# also package the same server/CA material as PKCS12 for it to pick up.
mkdir -p secrets
STOREPASS=changeit
openssl pkcs12 -export -in server.pem -inkey server.key -certfile ca.pem \
  -name kafka -out secrets/kafka.server.keystore.p12 -passout "pass:$STOREPASS"
# The truststore specifically goes through keytool, not openssl: a PKCS12
# cert-only entry built by `openssl pkcs12 -export -nokeys` round-trips fine
# through openssl itself but Java's PKIXValidator loads it with zero trust
# anchors (silently — no error until a client cert actually needs checking),
# so the broker rejects every mTLS client with "trustAnchors ... must be
# non-empty". keytool (same JDK the broker runs on) doesn't have this gap.
rm -f secrets/kafka.truststore.p12
docker run --rm -v "$OUT_DIR:/certs" apache/kafka:3.9.0 keytool -importcert \
  -alias caroot -file /certs/ca.pem -keystore /certs/secrets/kafka.truststore.p12 \
  -storetype PKCS12 -storepass "$STOREPASS" -noprompt
printf "%s" "$STOREPASS" > secrets/keystore_creds
printf "%s" "$STOREPASS" > secrets/key_creds
printf "%s" "$STOREPASS" > secrets/truststore_creds

echo "Certs written to $OUT_DIR"
