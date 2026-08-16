# Postmortems

Chapter 49. Copy [`_template.md`](_template.md) to
`YYYY-MM-DD-short-headline.md` and fill it in within 48 hours, while the details
are still recoverable.

## Blameless means one specific thing

It means the goal is learning, not punishment, and it starts from the assumption
that everyone acted reasonably given what they knew at the time. That assumption
is usually true and always more useful than the alternative, because a person
who expects to be blamed reports less, later, and less accurately — and you lose
the information you needed.

Blameless does **not** mean nobody owns the fix. Every action item has a name
and a date on it.

## It shows up in the sentences

Write about the system, not the person.

| Don't | Do |
|---|---|
| Sarah deployed a bug | A deploy introduced a regression the tests did not cover |
| Nobody checked the migration | The migration process has no review step for locking behaviour |
| The on-call missed the alert | The alert fired to a channel with no paging configured |

The second column is not politeness. It is more accurate, and it points at
something you can change.

## The five whys

Keep asking until you reach something structural, then stop.

> The API returned 500s.
> **Why?** The database connection pool was exhausted.
> **Why?** A migration held a lock on the tasks table for 40 seconds.
> **Why?** It added a NOT NULL column with a default to a large table.
> **Why?** The author did not know that rewrites the whole table.
> **Why?** Migration review has no checklist for locking behaviour.

The first answer would have produced "add more connections". The fifth produces
a checklist that prevents the whole class. Stop at five: past that you are
philosophising, and "why do we build software at all" fixes nothing.

## Action items are the only section that changes anything

An action item is a concrete change to the system: a test, an alert, a refactor,
a checklist, a document. "Be more careful" is not an action item — it has no
owner, no due date, and no way to tell whether it happened.

Track them weekly like any other work, and theme them quarterly: five separate
postmortems that each produced "add an alert" are really one finding about
monitoring coverage, and that finding is worth a sprint.

## Running the meeting

- **The facilitator sits outside the incident chain.** Someone who was up all
  night fixing it cannot also chair the discussion of what went wrong with it.
- **Senior engineers volunteer their own past mistakes**, early and
  specifically. It is the fastest way to make the room safe, and nothing else
  works as well.
- **Read the timeline through in full before anyone analyses anything.** Half
  the disagreements dissolve once everybody has the same facts.
