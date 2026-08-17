# How to read the React client

Read `../ARCHITECTURE.md` first. Then this, or `../api/READING.md` — the two
halves are independent reads, and this one is shorter.

The structure is the same as the server's: questions to orient, then traces
you do yourself, because doing them is the exercise.

---

## Question 1 — What does this thing do?

It is the board people actually use: sign in, pick an organization, drag cards
between three columns, open a task, search, invite members, register webhooks.
Nothing more. Every rule lives on the server — this app renders state and
sends intent.

Three facts shape everything else:

1. **It knows no URLs.** Every call goes through `@beacon/sdk`, which is
   generated from the server's OpenAPI document. `grep -rn "fetch(" src/`
   returns exactly two hits, and both are deliberate: `api/sse.ts`, because
   `EventSource` cannot send an Authorization header, and
   `features/board/task-detail.tsx`, which PUTs a file to a presigned storage
   URL that never goes near the API. Each carries a comment saying so. A third
   hit would be a bug.
2. **The server is the source of truth, always.** The cache goes ahead of it
   during a drag, and the SSE stream hints that something changed, but neither
   is ever believed. What is on screen at rest came from a refetch.
3. **Colors, contrast and bundle size are enforced at build time.** All three
   are the kind of thing that decays silently, so none of them is left to
   review.

## Question 2 — How is it arranged?

```
src/
  api/         the boundary: endpoints, parsers, query keys, session, SSE
  design/      tokens.source.ts — every color in the product, and why
  components/
    ui/        the primitives — button, dialog, select, toast, …
    layout/    the app shell
  features/    one folder per thing a user does
    auth/ onboarding/ org/ projects/ board/ members/ search/ settings/ webhooks/
  routes/      the router, and the two pages that are not features
  lib/         cn, theme, use-debounced. Genuinely generic, and nothing else.
scripts/       gen-tokens, contrast, check-budget — the build-time guards
e2e/           Playwright, against a real Beacon
```

The rule for `features/`: a folder owns its screens, its hooks and its local
state, and reaches down into `api/`, `components/` and `lib/`.

Sideways imports between features are the thing to watch, and there are three
in here. Two are deliberate:

- `features/org/org-gate.tsx` exports `useOrgContext`, and every screen inside
  the shell reads it. That is a provider, not a sibling dependency.
- `features/board/position.ts` exports `GAP`, which onboarding uses to seed a
  first project. It is a domain constant that happens to live next to the code
  that uses it most.

One is not. `features/auth/form-error.tsx` and `use-submit.ts` are imported by
four other features, which means they are not auth's — they are the app's form
primitives sitting in the wrong folder. They belong in `components/ui/` and
`lib/`. Nothing breaks today; it is on the work-list below, and it is a good
first change to make in a codebase you are learning, because the compiler
finds every call site for you.

## Question 3 — Where does execution start?

`src/main.tsx`, and it is 70 lines. Read it top to bottom once. It does four
things, in this order, and the order is the point:

```
watchTheme()          before paint, so there is no flash of the wrong theme
configureBeacon()     the SDK's one configuration call, before any component
new QueryClient()     the retry policy and the staleness budget
createRoot().render() the provider stack, then the router
```

`configureBeacon` is the one worth pausing on. Every Beacon-specific thing
about a request — the bearer token, the Idempotency-Key on mutations,
Retry-After on a 429, the timeout, what to do when the token expires — is
declared there and nowhere else. No component assembles a request, so no
component can forget one of those.

Run it:

```bash
make up && make api     # in one terminal — the client needs a real server
make web                # in another; http://localhost:5180
```

## Question 4 — Trace one path: moving a card

This is the trace worth doing, because it crosses every boundary in the app.

Drag a card between columns, then follow it:

1. `features/board/board-page.tsx` — the drop handler. **Read where it gets
   the dragged card's ID from.** It comes off `dataTransfer`, not React state,
   and the comment says why: state read inside a native drag handler is the
   state from when the handler was created.
2. `features/board/position.ts` — the new position is the midpoint between its
   neighbours, so a move is one UPDATE of one row rather than a renumbering of
   the column.
3. `features/board/use-move-task.ts` — the optimistic update. Three things
   happen before the request goes out, and each prevents a specific bug.
   Name all three before reading the comment.
4. `api/endpoints.ts` → `@beacon/sdk` → the server.
5. `api/parsers.ts` — the response is parsed, not trusted.
6. `features/org/org-gate.tsx` — the SSE event arrives and invalidates the
   query. The refetch is what is finally on screen.

Then answer:

- The mutation sends `title` as well as `status` and `position`. Why? What
  happens to a card if it does not? (The answer is in the server's contract,
  not in this app.)
- What does the card do when the server rejects the move, and what does the
  user see? Find where that is decided.
- If the SSE stream is disconnected, what still works and what does not?

## Question 5 — Trace a value that came from Go

Open `src/api/types.ts`. Every type in it is an alias for something the SDK
generated from a Go struct. Pick `Task`, and follow it backwards:

```
web/src/api/types.ts       ->  @beacon/sdk
sdk/src/generated/         ->  api/openapi.json
api/internal/http/ops_tasks.go and internal/tasks/tasks.go
```

Now make the round trip yourself: change a field name in the Go struct, run
`make sdk` from the repository root, and run `npm run typecheck` here.

Rename `title` on `CreateTaskInput` and you get exactly one error, in
`api/endpoints.ts`. That is the point of the next paragraph: the contract
change lands at the boundary, once, instead of in every component that
happened to render a task. Put it back.

That is the whole argument for the monorepo, in one command.

**Why `types.ts` exists at all**, when the SDK already exports these: nothing
in the app imports from `@beacon/sdk`'s generated types directly. It imports
these aliases. So when the server changes, every mismatch surfaces in one
file instead of in forty components — and the aliases are where a domain fact
the contract cannot carry gets written down.

## Question 6 — Read the two guards that fail the build

Neither of these is application code. Both exist because the thing they check
is invisible until a user complains.

**`scripts/contrast.ts`.** Every text role is checked against every surface it
is allowed to sit on, in both themes — currently 46 pairs across 31 roles —
and `gen-tokens` refuses to write anything if one fails. It was not written
speculatively: it caught white-on-volt at 1.7:1 on the first run, plus two
more. Try lowering a token's contrast in `design/tokens.source.ts` and run
`npm run gen:tokens` to watch it refuse.

**`scripts/check-budget.ts`.** The number it enforces is not total bytes — it
is what a visitor downloads before the first screen paints. Entry plus vendor
chunks are budgeted at 200 KB gzipped and currently sit at 189 KB; lazily
loaded screens are counted separately, because they arrive after paint. A
budget in a document is a budget nobody checks.

---

## Five things worth reading closely

1. **`api/sse.ts`** — the only `fetch` outside the SDK, and the header
   comment says why `EventSource` cannot be used: it sends no `Authorization`
   header, and the usual workaround puts a live credential in every access
   log and `Referer` on the way. Also read what it does when the tab is
   hidden.
2. **`api/parsers.ts`** — `True<Exact<A, B>>`. A compile-time proof that the
   Zod schema and the generated type are identical. Two details in the
   comment are the difference between a guard and a decoration: the mismatch
   arm must be `false` rather than `never`, and it must be applied at each
   call site rather than wrapped in an alias. Both were found by writing a
   version that passed while being wrong.
3. **`features/board/position.ts`** — floats with a midpoint insert, and a
   threshold that is RELATIVE to the magnitude. An absolute `1e-6` looks
   careful and is dead code above about 1e12, because it is smaller than one
   ULP there. The header comment quotes exact insert counts — 53 midpoint
   inserts into one gap at position 1e3, only 23 at 1e12 — and
   `position.test.ts` pins every one of those numbers, so a comment with
   arithmetic in it cannot quietly go stale. Read the difference between the
   prepend limit and the insert-between limit; conflating them understates the
   risk by a factor of twenty.
4. **`components/ui/highlight.tsx`** — search results arrive with `<mark>` in
   them. `dangerouslySetInnerHTML` would render server-supplied HTML, so this
   parses the markers and builds elements instead. The test feeds it
   `<img src=x onerror=alert(1)>` and asserts zero image elements.
5. **`features/org/org-gate.tsx`** — one gate resolves the active org, and
   children read it from context rather than mounting their own. Nesting it
   rendered the whole shell once per level, which is how this app briefly
   shipped with two header bars stacked on each other.

## House rules this codebase already follows

- No component assembles a request. `api/endpoints.ts` is the whole surface
  the app depends on, and it is one readable file.
- No component imports the SDK's generated **types**; it imports
  `api/types.ts`, so a server change surfaces in one file rather than forty.
  Importing the SDK's hand-written **behaviour** is fine and four components
  do — `BeaconError` is how a screen tells a rate limit from a validation
  failure.
- No hex color outside `design/tokens.source.ts`. Tailwind classes come from
  the generated tokens; charts and canvas read `tokens.gen.ts`.
- Every response is parsed at the boundary. `parser()` throws — one bad object
  means the screen cannot be drawn. `pageParser()` drops the bad row and keeps
  the page — one corrupt task must not blank a board of thirty good ones.
- Every optimistic update cancels in-flight refetches, snapshots for rollback,
  and invalidates on settle. All three, every time.
- A failure the user caused says what to do about it. A silent revert reads as
  "the drag did not register", and they try again.

## What is verified, and what is not

- `npm run lint` / `npm run typecheck` — ESLint and `tsc --noEmit`.
- `npm run test` — 45 unit and component tests, no server needed.
- `npm run e2e` — 13 Playwright tests against a **real** Beacon on `:8080`,
  not a mock. They share one account per run on purpose: a test that signs up
  per case trips Beacon's own auth rate limiter, which is the limiter working
  correctly. Read `e2e/helpers.ts` before adding one.
- `npm run build` runs `gen:tokens`, `tsc --noEmit`, the Vite build and the
  budget check, in that order. Any of the four can fail it.
- `make visual` (from the repository root) — Playwright, against the built
  **container image**, not `vite dev` and not your host OS. Both the app and
  the browser run in Linux so the baselines match CI; see
  `playwright.visual.config.ts` for why that is not optional. Read the header
  comment in `visual/styleguide.spec.ts` before trusting a green run — it
  records exactly what was broken on purpose to prove the suite catches it,
  and what it does not cover yet (the board, which needs a session).
- `.github/workflows/ci.yml` runs all of the above, plus the Go tests, on
  every push. Its job order is deliberate: `contract` first, alone, because a
  regenerated-but-uncommitted SDK is the fastest thing to catch.

Not yet verified: there is no production container image *published*
anywhere — `Dockerfile` here and `../api/Dockerfile` both build and were run
locally, but nothing pushes them to a registry, because none is configured
for this project. `DEVIATIONS.md` is the standing list of what was built
differently from the course and why.

## Small changes worth making, if you want a way in

1. Move `features/auth/form-error.tsx` to `components/ui/` and
   `features/auth/use-submit.ts` to `lib/`. Four features import them from
   auth today. One rename, and `npm run typecheck` hands you the list.
2. Beacon has no bulk-reorder endpoint, so `needsRebalance` warns and nothing
   repairs the column. Adding the endpoint is a server change and a client
   one, which makes it the first change that exercises the whole contract
   chain: Go struct, `make sdk`, then the call site.
3. The visual suite covers the primitives and the two screens that render
   without a server. Extending it to the board needs a signed-in session and
   seeded data — closer to `e2e/helpers.ts` than to `visual/`, and a good
   second exercise once the first one is done.
