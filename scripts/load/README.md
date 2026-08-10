# Load tests (Chapter 50)

The workflow the chapter insists on, in order. Doing it backwards — guessing at
a fix, then measuring — is how you spend a week optimising the wrong function.

1. **Load test** until something bends.
   ```bash
   k6 run scripts/load/list_tasks.js
   ```
2. **Profile under that load**, not at rest. A profile of an idle server tells
   you nothing about a busy one.
   ```bash
   go tool pprof http://localhost:9090/debug/pprof/profile?seconds=30
   ```
3. **Form a hypothesis the profile supports.** In `top`, `flat` is time spent
   inside the function itself and `cum` is the function plus everything it
   calls. A high `flat` is the real hot spot. In a flame graph (`web`), read
   width, not height: wide means CPU time, tall just means a deep call stack.
4. **Fix one small thing.**
5. **Re-run the load test**, then re-profile to confirm the hot spot moved.

## Getting the environment variables

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"password12345"}' | jq -r .token)

ORG_ID=$(curl -s http://localhost:8080/v1/orgs -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')

PROJECT_ID=$(curl -s http://localhost:8080/v1/orgs/$ORG_ID/projects \
  -H "Authorization: Bearer $TOKEN" | jq -r '.items[0].id')

BEACON_TOKEN=$TOKEN BEACON_ORG_ID=$ORG_ID BEACON_PROJECT_ID=$PROJECT_ID \
  k6 run scripts/load/list_tasks.js
```

## The profiles worth knowing

| Endpoint | Answers |
|---|---|
| `/debug/pprof/profile?seconds=30` | where CPU time goes |
| `/debug/pprof/heap` | where memory is allocated, and what is still held |
| `/debug/pprof/goroutine` | goroutines leaking, or all stuck in one place |
| `/debug/pprof/mutex` | lock contention (needs `runtime.SetMutexProfileFraction`) |

All of them live on the **internal** port (`:9090`), never the public one — see
`internal/http/admin.go` for why.

To compare two heap profiles before and after a change:

```bash
go tool pprof -base heap_before.pb.gz heap_after.pb.gz
```
