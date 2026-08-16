# openapi.json is generated. Do not edit it.

Run `make spec` from the repository root, or `make -C api spec`.

It is produced by `cmd/beacon-spec`, which builds the router — registering
every operation as a side effect — and serializes the resulting document. No
listener starts and no database connection opens, so it runs in about a second.

The previous `openapi.yaml` was written by hand from reading the handlers, and
it was wrong: it claimed `PATCH /v1/me/preferences` answered `200 → Preferences`
when the server answered `204 No Content`. The generated client believed the
document, parsed an empty body, and threw — and nothing caught it, because the
compiler only ever saw the spec.

That file is gone. This one cannot disagree with the handlers, because it is
made out of them.

One entry is hand-written and says so in the code: the server-sent events
stream, in `internal/http/ops_events.go`. It is a long-lived `text/event-stream`
rather than a request and a response, so it is not a huma operation — but a
client still has to be able to discover it, so it is added to the document
explicitly, next to the reason.
