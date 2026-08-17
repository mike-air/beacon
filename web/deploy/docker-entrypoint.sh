#!/bin/sh
# Write the runtime config the app reads before it boots.
#
# Vite inlines import.meta.env at build time, so an API base chosen at build
# time is welded into the bundle and every environment needs its own image —
# which means the artefact tested in staging is not the artefact shipped to
# production. This writes it at START-UP instead, so one image runs anywhere.
#
# Runs from /docker-entrypoint.d, which the nginx base image executes before
# starting nginx. It is numbered 99 so it runs after the envsubst step that
# renders the nginx template with the same variable.
set -eu

: "${BEACON_API_BASE:=}"

if [ -z "$BEACON_API_BASE" ]; then
    echo "beacon: BEACON_API_BASE is not set; the app will fall back to its build-time default" >&2
else
    echo "beacon: API base is $BEACON_API_BASE"
fi

# Written OUTSIDE the document root on purpose. The web root stays owned by
# root and unwritable by the nginx user, so a compromised worker cannot edit
# the files it serves; nginx reaches this one through an alias instead.
mkdir -p /tmp/beacon
cat > /tmp/beacon/config.js <<JS
/* Written at container start-up. Do not cache. */
window.__BEACON__ = { apiBase: "${BEACON_API_BASE}" };
JS
