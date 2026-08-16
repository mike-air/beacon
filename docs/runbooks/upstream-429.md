# Runbook: an upstream is rate-limiting us (429s)

**Trigger:** `BeaconUpstream429` — 429 responses from any third party for 5 minutes
**Owner:** platform

## Symptom

- Webhook deliveries or emails are failing with `429 Too Many Requests`.
- A third-party call (payments, email, search) is returning 429 and our retries
  are making it worse.
- The API is healthy. This is a failure at the edge of our system, pointed
  outwards.

Note the direction. [Chapter 19's rate limiter](../../internal/http/middleware_ratelimit.go)
is us limiting *our* callers. This runbook is the opposite: somebody limiting
*us*, and the reflex that helps in one case (retry) actively harms in the other.

## Diagnostic

Read-only.

```bash
# 1. Which upstream, and what's the rate of 429s?
flyctl logs --app beacon-worker | grep -E '429|rate.?limit' | tail -40
```

```sql
-- 2. What is our send rate? Are we above the documented quota?
SELECT date_trunc('minute', created_at) AS minute, kind, count(*)
FROM jobs
WHERE created_at > now() - interval '30 minutes'
GROUP BY 1, 2
ORDER BY 1 DESC, 3 DESC
LIMIT 30;
```

Compare the per-minute number against the provider's published quota. If we are
above it, this is our bug, not theirs.

```sql
-- 3. Are we in a retry storm? Retries count against the quota too, so a
--    failing endpoint can spend the whole budget on itself.
SELECT kind, attempts, count(*)
FROM jobs
WHERE status IN ('pending','dead') AND last_error ILIKE '%429%'
GROUP BY kind, attempts
ORDER BY attempts DESC;
```

```bash
# 4. Did a recent deploy change our call pattern?
flyctl releases --app beacon-api-blue | head -5
git log --oneline -10
```

A sudden 429 with no traffic change is nearly always a code change that made one
call per item where it used to make one call per batch.

## Mitigation

**Slow down before you retry.** The instinct under a 429 is to retry harder;
that is what turns a throttle into a ban.

```bash
# a. Reduce worker concurrency so fewer calls go out at once. This is the
#    real lever: every outbound call Beacon makes — webhook deliveries and
#    email — is made by a worker, so concurrency IS the send rate.
flyctl secrets set WORKER_CONCURRENCY=1 --app beacon-worker
```

> **There is no per-upstream rate limit.** An earlier version of this runbook
> told you to set `BEACON_STRIPE_RPS`. Nothing reads that variable — no Go
> file references it, and Beacon has no payments integration at all. Setting
> it does nothing, which is the worst possible property for a mitigation step
> in an incident: it looks like you have acted, and you have not.
>
> Until a real outbound limiter exists (see the work-list in
> [READING.md](../../READING.md)), `WORKER_CONCURRENCY` and parking the retry
> storm below are the only levers that actually change the outbound rate.
> Verify that any knob a runbook names is read by
> [internal/config/config.go](../../internal/config/config.go) before you
> trust it.

**Stop the retry storm.** Park the jobs that are only burning quota, so the ones
that can succeed get through:

```sql
UPDATE jobs SET status = 'dead', last_error = 'parked during 429 incident'
WHERE status = 'pending' AND last_error ILIKE '%429%' AND attempts >= 3;
```

They are in the dead state, not deleted, so they can be revived once the quota
recovers:

```sql
UPDATE jobs SET status = 'pending', attempts = 0, run_at = now()
WHERE status = 'dead' AND last_error = 'parked during 429 incident';
```

**If a deploy caused it (diagnostic 4)**, roll back — a per-item call pattern
will not fix itself:

```bash
./deploy/bluegreen/rollback.sh
```

## Escalation

- **After 15 minutes**, or immediately if the upstream is a payment provider:
  page the platform on-call AND open a support ticket with the provider. A quota
  raise usually needs them, and their queue is long.
- **If the 429s become 403s**, we have been blocked rather than throttled. Stop
  all calls to that provider (`WORKER_CONCURRENCY=0` on the affected worker) and
  escalate to the platform lead — retrying through a block extends it.
- **Stop signal:** 429s have stopped for 10 minutes at the reduced rate. Then
  raise the rate back in steps, not in one jump, and write the postmortem: the
  quota is a fact about the system that was not being tracked, and that is the
  action item.
