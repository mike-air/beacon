#!/usr/bin/env bash
# Chapter 46 — the rollback. The reason blue-green is worth the second stack.
#
# No build. No deploy. No image. One KV write, and traffic is back on the stack
# that was working sixty seconds ago, with its pool already open and its caches
# already warm.
#
# The window is real, though: the old stack only stays warm for about thirty
# minutes after a swap. After that this stops being instant and starts being a
# deploy like any other.
set -euo pipefail

LIVE=$(wrangler kv:key get live_color --binding=BEACON_CONFIG)
PREV=$([ "$LIVE" = "blue" ] && echo "green" || echo "blue")

echo "rolling back: $LIVE -> $PREV"
wrangler kv:key put live_color "$PREV" --binding=BEACON_CONFIG
echo "done. $PREV is live again."
echo
echo "Now go and find out what was wrong with $LIVE — a rollback buys you time,"
echo "it does not answer anything."
