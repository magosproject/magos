#!/bin/sh
set -eu

if [ "${MAGOS_TLS_ENABLED:-false}" = "true" ]; then
    TEMPLATE=/etc/nginx/templates/nginx.tls.conf.template
else
    TEMPLATE=/etc/nginx/templates/nginx.conf.template
fi

envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' < "$TEMPLATE" > /tmp/nginx.conf

exec nginx -c /tmp/nginx.conf -g 'daemon off;'
