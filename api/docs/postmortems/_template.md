<!-- docs/postmortems/_template.md -->
# Postmortem: <short headline>

**Status:** draft | in review | final
**Author:** <name>
**Date:** <YYYY-MM-DD>
**Incident:** <link to incident ticket>

## 1. Summary

<Two sentences. What broke, when, how long, how bad.>

## 2. Timeline

All times UTC.

- `HH:MM` — <event>
- `HH:MM` — <event>
- ...

## 3. Root cause

### What technically broke

<One paragraph describing the surface failure.>

### Why that happened (five whys)

1. <First why>
2. <Second why>
3. <Third why>
4. <Fourth why>
5. <Fifth why — stop here>

## 4. Impact

- **Users affected:** <number, with breakdown if useful>
- **Duration:** <start UTC to end UTC>
- **Failed requests:** <number>
- **SLO budget consumed:** <% of monthly budget>
- **Data loss:** <yes/no, details if yes>
- **Money:** <refunds, contract penalties, etc.>

## 5. What went well

<Bullets of things the team did right during the incident. Always
include at least three — there are always at least three.>

## 6. What went poorly

<Bullets of things that didn't go well. Frame as system issues,
not personal failings.>

## 7. Action items

| # | Item | Owner | Due | Ticket |
|---|------|-------|-----|--------|
| 1 | <Concrete change> | @alice | YYYY-MM-DD | BEA-1234 |
| 2 | <Concrete change> | @bob   | YYYY-MM-DD | BEA-1235 |
