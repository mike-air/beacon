# @beacon/sdk

The typed client for the Beacon API.

```ts
import { configureBeacon, beacon } from "@beacon/sdk";

configureBeacon({
  baseUrl: import.meta.env.VITE_API_BASE,
  getToken: () => localStorage.getItem("beacon-token"),
  onUnauthenticated: () => signOut(),
});

const me = await beacon.getMe();
const tasks = await beacon.listTasks({ path: { orgID, projectID } });
```

## What is generated and what is not

`src/generated/` is produced by `make sdk` from `../api/openapi.json`, which
the Go handlers emit. **Do not edit it** — the next regeneration will overwrite
you. Each method carries the documentation its Go handler declared, so hovering
`beacon.search` in an editor tells you why a `postgres` source is worth showing
the user.

Everything else in `src/` is hand-written and permanent:

| File | Why it exists |
|---|---|
| `client.ts` | The behaviour the contract does not describe: bearer token, Idempotency-Key held across retries, Retry-After honoured, timeouts, single-flight sign-out on 401. |
| `errors.ts` | `BeaconError`, carrying the `code` callers branch on. |
| `index.ts` | The public surface, and `unwrap`. |

The split is the point. Regenerating must never overwrite behaviour, and
behaviour must never be duplicated at a call site.

## The contract cannot drift

```
Go struct  ->  api/openapi.json  ->  sdk/src/generated  ->  your code compiles or does not
```

`make contract` regenerates the whole chain and fails if the result differs
from what is committed. A handler changed without a regenerated SDK is a red
build, not a runtime surprise.
