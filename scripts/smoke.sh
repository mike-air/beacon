#!/usr/bin/env bash
# The external smoke test the blue-green deploy runs against the DARK stack
# before flipping any traffic to it (Chapter 46, step 3).
#
# It talks to the stack the way a client does — over the public interface, no
# shortcuts — because "the process started" is not the same claim as "the thing
# works". Any failure exits non-zero, which stops the swap.
#
#   ./scripts/smoke.sh https://beacon-api-green.fly.dev
set -euo pipefail

BASE="${1:?usage: $0 <base-url>}"
fail() { echo "SMOKE FAIL: $*" >&2; exit 1; }

echo "smoke-testing $BASE"

# 1. Liveness: the process is answering at all.
code=$(curl -fsS -o /dev/null -w '%{http_code}' "$BASE/healthz") || fail "healthz unreachable"
[ "$code" = "200" ] || fail "healthz returned $code"
echo "  healthz 200"

# 2. Readiness: it can actually reach its dependencies. This is the check that
#    catches the deploy shipped with the wrong DATABASE_URL.
code=$(curl -fsS -o /dev/null -w '%{http_code}' "$BASE/readyz") || fail "readyz unreachable"
[ "$code" = "200" ] || fail "readyz returned $code"
echo "  readyz 200"

# 3. The API root answers, and answers as the service we expect.
body=$(curl -fsS "$BASE/v1/") || fail "/v1/ unreachable"
echo "$body" | grep -q 'beacon-api' || fail "/v1/ did not identify as beacon-api: $body"
echo "  /v1/ identifies as beacon-api"

# 4. Auth is wired: a request with no token must be refused, not accepted and
#    not crash. A 500 here means the stack is up and broken.
code=$(curl -fsS -o /dev/null -w '%{http_code}' "$BASE/v1/me" || true)
[ "$code" = "401" ] || fail "/v1/me without a token returned $code, expected 401"
echo "  /v1/me refuses an anonymous caller (401)"

echo "SMOKE PASS"
