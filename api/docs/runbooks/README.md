# Runbooks

Chapter 48. A runbook is written for the version of you that is awake at 3am,
has been paged, and is frightened. That reader does not read; they scan. So
every runbook in this directory has the same four sections in the same order:

| Section | Answers |
|---|---|
| **Symptom** | Am I in the right document? |
| **Diagnostic** | What is actually happening? — **read-only commands only** |
| **Mitigation** | How do I stop the bleeding? — the commands that change things |
| **Escalation** | Who do I wake, and when do I stop trying? |

The split between Diagnostic and Mitigation is not tidiness. Diagnostic commands
are read-only *by definition*, so a tired reader can run every one of them
without thinking, and cannot cause a second incident while investigating the
first. Anything that changes state lives in Mitigation, where it is read
deliberately.

Two more rules the chapter is firm about:

- **A specific trigger.** Every runbook opens with an alert name or a concrete
  symptom. "Some users report issues" is not a trigger and nobody can act on it.
- **Full, copy-pasteable commands.** Not "check the connection count" — the
  exact query, with the exact connection string shape. At 3am, the gap between
  a described command and a runnable one is ten minutes.

Link the alert straight to its runbook, so being paged and opening the right
page is one click, not a search.

## The runbooks

| Runbook | Trigger |
|---|---|
| [Database slow / connection pool exhausted](db-slow.md) | `BeaconDatabaseLatencyHigh`, `BeaconPoolExhausted` |
| [Job queue backlog](queue-backlog.md) | `BeaconQueueBacklog` |
| [Upstream rate-limiting us (429s)](upstream-429.md) | `BeaconUpstream429` |

## Two habits that keep these alive

**Fire drills.** Once a quarter, pick a runbook and have somebody who has never
used it follow it on a staging incident. Every gap they hit is a bug in the
document.

**Count the runs.** Every time a runbook is used, that is a vote to spend a
sprint fixing the thing underneath it. The measure of a good net is that it gets
used *less* over time, not faster.
