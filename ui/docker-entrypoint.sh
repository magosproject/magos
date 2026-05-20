#!/bin/sh
set -eu

TEMPLATE=/etc/nginx/templates/nginx.conf.template
OUTPUT=/tmp/nginx.conf

if [ "${MAGOS_TLS_ENABLED:-false}" = "true" ]; then
    envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' < "$TEMPLATE" > "$OUTPUT"
else
    # Strip the optional `server { listen 443 ssl; ... }` block.
    awk '
        /listen 443 ssl;/ { skip = 1 }
        skip && /^    }$/ { skip = 0; next }
        !skip
    ' "$TEMPLATE" | envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' > "$OUTPUT"
fi

exec nginx -c "$OUTPUT" -g 'daemon off;'
