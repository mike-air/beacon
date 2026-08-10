# Beacon — Product Requirements Document

**Status:** Foundations, scale features, and the operational story are built and
running. Core loop works end to end.
**Audience:** Anyone deciding what Beacon should do next — not a code
walkthrough. For "where is the code," see [READING.md](READING.md).

---

## 1. What Beacon is

Beacon is a team task board — the same shape of product as Linear, Trello,
or Asana, stripped to its skeleton. A team creates an account, invites
people, organises work into projects, and tracks tasks through a workflow
with comments and file attachments.

It exists as the practice project for a course on building production Go
services, so it is deliberately built to the standard of a real product,
not a demo: the auth is real, the multi-tenancy is real, the failure
handling is real. What's missing is missing because it hasn't been built
yet, not because it was cut for a demo.

## 2. Who it's for

Internally: it is the codebase used to learn two things — writing
production backend code, and reading a codebase somebody else wrote. The
"customer" is whoever is doing that learning.

If it were a real product, it would be for small teams (5–50 people) who
want task tracking without the weight of a full project-management suite.

## 3. The core concept, in plain language

Everything in Beacon is one of seven kinds of thing:

```
organization
  └── membership (a user, tied to this org, with a role)
  └── project
        └── task
              ├── comment
              └── attachment
```

**A user is not "in" an organization.** A user account exists on its own —
same as a person exists whether or not they belong to a company. A
**membership** is the separate fact that connects a user to one
organization and says what they're allowed to do there (owner, admin, or
member). This distinction is the single most important modelling decision
in the product: every permission question is really two questions —
*who are you*, and *what's your role in this specific org* — and getting
that separation right is what makes multi-tenant security tractable.

**Everything nests and nothing floats free.** An organization owns
projects. A project owns tasks. A task owns comments and attachments. You
can always answer "who does this belong to" by walking up the chain, which
is what makes it possible to guarantee one team can never see another
team's data.

## 4. What's built and working

Verified running end to end against real Postgres, real S3-compatible
storage (MinIO), and a real mail catcher (Mailpit) as of this document.

### Accounts and access

- Sign up, log in. Passwords hashed with argon2id (the current
  recommended standard — not a legacy hash).
- JWT-based sessions: a short-lived access token (default 1 hour) plus a
  refresh token, so a stolen access token has a short shelf life.
- Create an organization, invite members, assign roles (owner / admin /
  member). Owner and admin actions — adding members, managing webhooks —
  are gated by role; a plain member cannot do them, at the API level, not
  just hidden in the UI.

### Work tracking

- Projects within an org. Tasks within a project. Comments and file
  attachments on tasks.
- Every list endpoint is scoped to the org the caller belongs to — there
  is no way to query another organisation's data by guessing an ID.
- Deletions are soft: nothing is destroyed, records are marked deleted
  and excluded from normal views, so accidental deletion is recoverable.
- Concurrent edits are protected: if two people edit the same task at
  once, the second save is rejected rather than silently overwriting the
  first person's change.

### File attachments

- Files are uploaded directly to storage via a short-lived signed URL —
  the file never passes through the API server itself, which is how
  production systems handle uploads at any real scale.

### Notifications and integrations

- Transactional email (welcome, invites) — sent for real via SMTP in
  production, or just logged in development, so nothing requires a paid
  email account to run locally.
- Outgoing webhooks: an organisation can register a URL and get a signed
  HTTP callback when things happen in their account, the same mechanism
  Stripe and GitHub use for their webhooks. Signed with a secret so a
  receiver can verify the call really came from Beacon.
- A live event stream (Server-Sent Events) so a connected browser tab
  sees updates as they happen, without polling.

### Behind the scenes

- A background job queue backed by Postgres (no separate queue service to
  run) and a scheduler for recurring jobs.
- Every request is logged in structured form (who, what, how long, what
  happened) rather than free-text log lines, which is what makes an
  incident debuggable instead of a guessing game.
- Errors returned to clients follow one consistent shape everywhere —
  a machine-readable code plus a human message — rather than fifty
  handlers each inventing their own error format.

### Scale and safety

- **Safe retries.** A client can attach an idempotency key to any request that
  changes something. Retry it after a dropped connection and the second attempt
  returns the first one's answer rather than doing the work twice — the
  difference between "the train went into a tunnel" and "the customer was
  charged twice".
- **Fairness limits.** Each customer gets their own request budget, and the
  login endpoints get a much tighter one per network address. One customer's
  runaway script can no longer slow the service down for everybody else.
- **Caching.** Frequent reads are answered from memory or a shared cache before
  they reach the database, and a change to the underlying record clears the copy
  immediately rather than waiting for it to expire.
- **Search.** People can search their organisation's tasks, projects and
  comments by words rather than by scrolling. It understands that "authenticating"
  and "authentication" are the same word, and word order does not matter. With
  the optional search engine switched on it also tolerates typos; with it
  switched off, or broken, search keeps working with slightly different ranking
  rather than returning nothing.
- **Gradual rollout.** New features can be switched on for one customer, or one
  person, without a deploy, and switched off just as fast — and a feature can be
  split so half the eligible people see the new version, for measuring whether
  it is actually better.
- **Local formats.** Dates, times and currency render in the reader's language
  and time zone. Money is never stored as a decimal fraction, which is the usual
  way rounding errors get into a ledger.

### Operations

- **Watching it live.** The system publishes its own numbers — request rates,
  latencies, error rates, queue depth — and every request can be followed as a
  single connected trace from the browser call through the database and on into
  the background job it triggered, minutes later.
- **Backups that have been restored.** A nightly encrypted copy goes to storage
  with a different provider from the database, and a drill script restores it
  into a scratch database and checks the data actually came back. A backup
  nobody has restored is a hope, not a backup.
- **Deploys with an undo.** Two identical copies of the service exist; one is
  live, one is dark. New code goes to the dark one and is tested with no users
  on it, then traffic is switched in one step. Undoing that switch takes seconds
  and needs no rebuild.
- **Deploys that watch themselves.** A new version can be given 1% of traffic,
  then 10%, then 50%, with automatic checks on error rate, latency, error budget
  and any error signature nobody has seen before. Any check failing puts traffic
  back to zero without a human.
- **Written-down responses.** Three incident runbooks (database slow, work queue
  backing up, an outside service throttling us) and a postmortem process, so
  the same problem is not solved from scratch at 3am twice.

## 5. What's explicitly not built yet

Real gaps, not oversights, and not hidden. Ranked roughly by how soon a
real product would need them:

| Missing | Why it eventually matters | Effort |
|---|---|---|
| **Undo for deletions** | Deleting a project removes it permanently. Customers delete things by mistake constantly, and "we cannot get it back" is an expensive sentence. | Medium |
| **An audit trail** | There is no record of who changed what. The first enterprise customer will ask for one, usually during a security review. | Medium |
| **API keys** | Integrations have to log in as a person. A machine credential that can be revoked on its own is the normal answer. | Small |
| **Actually running on a server** | This runs on a laptop against Docker. The deploy machinery is written and the checks are tested, but nothing has been deployed. | Medium |
| **Cursor pagination** | Paging by offset gets slow on large lists and can skip or repeat rows while data is being written. | Medium |
| **Refresh tokens** | The access token lasts an hour and then the person logs in again. Standard practice is a short token plus a refresh flow. | Medium |

None of these are architecturally blocked — the system was built with
room for all of them. They're sequenced work, not redesign work.

**One honest caveat about the deploy story.** The blue-green and canary
machinery is written, and the controller that drives it has been run against a
real metrics system and made to both approve and reject a rollout. But the
traffic routers themselves need a hosting account nobody has set up, so no
actual cutover has been performed. Everything else described in this document
has been run and watched working.

## 6. Key product decisions, and the trade-off behind each

**One shared database, filtered by an `org_id` column** rather than a
separate database per customer, or a database feature that auto-filters
rows. The alternatives give stronger isolation guarantees but cost far
more to operate at this stage — every schema change would need to run
once per customer, or every engineer would need to remember an invisible
rule on every query. The column approach is boring, and boring was the
right call for the first version. It can be hardened later without a
rewrite if a customer's contract ever requires it.

**Deletions are soft, not hard — as a decision, not yet as code.** The intent is
that a customer who deletes something by mistake can get it back, at the cost of
extra housekeeping in every query. Worth it: "we permanently deleted your data
by accident" is not a sentence a product recovers from. This is listed in §5
because the decision is made and the implementation is not.

**Search is answered by the database until it can't be.** Postgres has a
capable search engine built in, and using it means no second system to run,
keep in sync, or be woken up by. A dedicated search engine was added on top only
for what the database genuinely cannot do — typo tolerance — and the database
remains the source of truth, so the search engine failing degrades results
rather than removing them.

**The system limits its own customers.** Rate limiting is usually described as
protection against attackers. Here it is mostly about fairness: one customer's
runaway integration should not be able to make the product slow for everybody
else, and the limit is set per organisation because that is the unit customers
pay for.

**File uploads bypass the API server.** The server hands out a temporary
signed link and the browser uploads straight to storage. This means the
API's own capacity never becomes the bottleneck for how many files people
can upload at once — a mistake that has taken down real products.

**The access token is short-lived on purpose.** An hour, not a week. It
narrows the window in which a leaked token is useful, at the cost of the
refresh flow having to work reliably. That trade is standard practice for
anything handling real accounts.

## 7. Glossary

- **Organization** — a customer account. Owns everything else.
- **Membership** — the record connecting one user to one organization,
  carrying their role. Not the same thing as the user account itself.
- **Role** — owner, admin, or member. Determines what a person can do
  inside one specific organization.
- **Soft delete** — marking something as deleted without removing it,
  so it can be recovered.
- **Optimistic locking** — the mechanism that rejects a save if the
  record changed since you last read it, to prevent one person's edit
  silently overwriting another's.
- **Idempotency key** — a value a client sends so that retrying the same
  request twice has the same effect as sending it once.
- **Feature flag** — a switch that turns a feature on for some customers
  without shipping new code.
- **Canary** — giving a new version a small slice of real traffic and watching
  it, before everybody gets it.
- **Trace** — the record of one request's whole journey through the system,
  including the background work it caused afterwards.
- **RPO / RTO** — how much data you would lose, and how long you would be down,
  in a disaster. Beacon targets five minutes and one hour.
- **Webhook** — an outbound HTTP call Beacon makes to a URL a customer
  registers, notifying them that something happened in their account.

---

*This document describes product behaviour, not implementation. For "what
file is this in," see [READING.md](READING.md), which is written for
someone reading the code rather than deciding what it should do.*
