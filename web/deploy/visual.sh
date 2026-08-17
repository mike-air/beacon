#!/usr/bin/env bash
# Run the visual regression suite against the production container.
#
# Both halves run in Linux containers so the pixels are the same on a laptop
# and on CI. Rendering the app on macOS and comparing against Linux baselines
# fails on font rasterisation alone, and the usual "fix" — a loose pixel
# threshold — is worse than no test, because it also hides real regressions.
#
#   ./visual.sh           compare against the committed baselines
#   ./visual.sh --update  re-record them after an intended change
#
# The browser container gets its OWN node_modules, in a named volume. Mounting
# the host's would overwrite the Mac's native binaries with Linux ones and
# leave the laptop unable to run anything until a reinstall.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NET=beacon-visual
WEB=beacon-visual-web
PW_VERSION="$(node -p "require('$ROOT/web/package.json').devDependencies['@playwright/test'].replace(/[^0-9.]/g,'')")"
IMAGE="mcr.microsoft.com/playwright:v${PW_VERSION}-noble"

UPDATE=""
[ "${1:-}" = "--update" ] && UPDATE="--update-snapshots"

cleanup() {
    docker rm -f "$WEB" >/dev/null 2>&1 || true
    docker network rm "$NET" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> building the production image"
docker build -q -f "$ROOT/web/Dockerfile" -t beacon-web:visual "$ROOT" >/dev/null

cleanup
docker network create "$NET" >/dev/null

echo "==> serving it"
# No API base: these screens are static and must not reach a server. If one
# ever does, the request fails loudly instead of silently rendering live data
# into a baseline.
docker run -d --name "$WEB" --network "$NET" --network-alias web \
    -e BEACON_API_BASE=http://api.invalid \
    beacon-web:visual >/dev/null

for _ in $(seq 1 30); do
    docker run --rm --network "$NET" "$IMAGE" \
        sh -c 'curl -sf http://web:8080/healthz >/dev/null' && break
    sleep 1
done

echo "==> running the visual suite in $IMAGE"
docker run --rm --network "$NET" \
    -v "$ROOT:/work" \
    -v beacon-visual-modules:/work/web/node_modules \
    -w /work/web \
    -e CI="${CI:-}" \
    "$IMAGE" \
    sh -c "npm ci --ignore-scripts --no-audit --no-fund >/dev/null && \
           npx playwright test --config=playwright.visual.config.ts $UPDATE"
