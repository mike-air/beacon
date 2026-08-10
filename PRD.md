# Beacon — Product Requirements Document

**Status:** Foundations built and running. Core loop works end to end.
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

## 5. What's explicitly not built yet

Real gaps, not oversights, and not hidden. Ranked roughly by how soon a
real product would need them:

| Missing | Why it eventually matters | Effort |
|---|---|---|
| **Idempotency keys** | Right now, retrying a failed "create task" request after a network blip can create it twice. | Small |
| **Rate limiting** | One customer (or one bug in their integration) can currently hammer the API with no limit. | Small |
| **Caching** | Every read hits Postgres directly. Fine at this scale, not at 10x. | Medium |
| **Full-text search** | Finding a task means listing and scanning; there's no "search for the word X". | Medium |
| **Feature flags** | Every feature is live for every customer the moment it ships — no gradual rollout. | Small |
| **Metrics, tracing, health dashboards** | We can read logs after something breaks; we can't watch the system in real time. | Medium |
| **Deployment pipeline** | This runs on a laptop against Docker. There is no story yet for a real server, CI gating, or backups. | Large |

None of these are architecturally blocked — the system was built with
room for all of them. They're sequenced work, not redesign work.

## 6. Key product decisions, and the trade-off behind each

**One shared database, filtered by an `org_id` column** rather than a
separate database per customer, or a database feature that auto-filters
rows. The alternatives give stronger isolation guarantees but cost far
more to operate at this stage — every schema change would need to run
once per customer, or every engineer would need to remember an invisible
rule on every query. The column approach is boring, and boring was the
right call for the first version. It can be hardened later without a
rewrite if a customer's contract ever requires it.

**Deletions are soft, not hard.** A customer who deletes something by
mistake — which happens constantly — can be recovered. The cost is a
small amount of extra housekeeping in every query. Worth it; "we
permanently deleted your data by accident" is not a sentence a product
recovers from.

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
  request twice has the same effect as sending it once. Not yet built
  (see §5).
- **Webhook** — an outbound HTTP call Beacon makes to a URL a customer
  registers, notifying them that something happened in their account.

---

*This document describes product behaviour, not implementation. For "what
file is this in," see [READING.md](READING.md), which is written for
someone reading the code rather than deciding what it should do.*
