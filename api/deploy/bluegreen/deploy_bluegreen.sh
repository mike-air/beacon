#!/usr/bin/env bash
# Chapter 46 — one deploy, one flip.
#
#   ./deploy/bluegreen/deploy_bluegreen.sh <image-sha>
#
# Prerequisite that has nothing to do with this script and everything to do with
# whether it works: both stacks share ONE database. So every schema change must
# be compatible with the old code AND the new code for the whole swap and the
# whole rollback window. That is what expand-contract is for:
#
#   1. expand   add the new shape; write to both; read from the old
#   2. migrate  backfill; switch reads to the new shape
#   3. contract drop the old shape — in a LATER deploy, once rollback is closed
#
# Never change schema and code in the same deploy. Additive changes are free;
# destructive ones are what turn a rollback into an outage.
#
# [verbatim ch46]
set -euo pipefail

IMAGE="${1:?usage: $0 <image-sha>}"

# 1. Figure out which color is dark.
LIVE=$(wrangler kv:key get live_color --binding=BEACON_CONFIG)
DARK=$([ "$LIVE" = "blue" ] && echo "green" || echo "blue")
echo "live=$LIVE  dark=$DARK  image=$IMAGE"

# 2. Deploy the new image to the dark color.
flyctl deploy --app "beacon-api-$DARK" \
  --image "$IMAGE" \
  --strategy immediate \
  --wait-timeout 300

# 3. Smoke-test the dark stack from outside.
./scripts/smoke.sh "https://beacon-api-$DARK.fly.dev"

# 4. Flip the live pointer.
wrangler kv:key put live_color "$DARK" --binding=BEACON_CONFIG
echo "swap complete — $DARK is now live, $LIVE is the rollback"
