#!/bin/sh
# Copies a renewed Let's Encrypt certificate where the FlowLight containers can
# read it (they run as uid 10001) and restarts them so they pick it up.
#
# Install:  cp certbot-deploy-hook.sh /etc/letsencrypt/renewal-hooks/deploy/flowlight.sh
#           chmod +x /etc/letsencrypt/renewal-hooks/deploy/flowlight.sh
# Then run it once by hand with RENEWED_LINEAGE set to the live directory, e.g.
#           RENEWED_LINEAGE=/etc/letsencrypt/live/control.example.com sh flowlight.sh
set -eu
TARGET=/etc/flowlight/tls
COMPOSE_DIR=${COMPOSE_DIR:-/opt/flowlight}
mkdir -p "$TARGET"
install -o 10001 -g 10001 -m 0644 "$RENEWED_LINEAGE/fullchain.pem" "$TARGET/fullchain.pem"
install -o 10001 -g 10001 -m 0640 "$RENEWED_LINEAGE/privkey.pem" "$TARGET/privkey.pem"
if [ -f "$COMPOSE_DIR/docker-compose.yml" ]; then
  docker compose -f "$COMPOSE_DIR/docker-compose.yml" restart control-api probe
fi
