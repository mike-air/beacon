package http

// The event stream, documented but not registered.
//
// GET /v1/orgs/{orgID}/events stays a plain chi handler. It is the one route
// that is not a request and a response: it holds a connection open for minutes
// and writes `event:`/`data:` frames until the client leaves or the server
// stops. huma can model that, but everything huma gives an operation — a typed
// body, a schema, a generated method that resolves a promise — is either
// meaningless or actively wrong for a stream.
//
// The contract still has to describe it. A client that cannot discover the
// realtime endpoint from the document has to be told about it out of band,
// which is exactly the situation this whole design exists to avoid. So the
// path is added to the emitted document by hand, right here, next to the
// reason it is a special case.
//
// This is the only hand-written entry in openapi.json, and it isthe only one, and it is here
// because it describes a stream rather than a shape.

import "github.com/danielgtaylor/huma/v2"

func documentEventStream(api huma.API) {
	doc := api.OpenAPI()

	doc.Paths["/v1/orgs/{orgID}/events"] = &huma.PathItem{
		Get: &huma.Operation{
			OperationID: "events",
			Summary:     "Server-sent events for the organisation",
			Description: "A `text/event-stream`. Each message is `event: <type>` with a " +
				"JSON `data:` payload; comment lines (`: keep-alive`) are heartbeats. " +
				"Types are `task.created`, `task.updated` and `task.deleted`, and the " +
				"payload is the task.\n\n" +
				"A slow consumer is DROPPED rather than allowed to block the publisher, " +
				"so a client must treat the stream as a hint to refetch and never as the " +
				"only source of truth.\n\n" +
				"Note that the token goes in the Authorization header, which `EventSource` " +
				"cannot set — consume this with `fetch` and read the body stream.",
			Tags:     []string{"events"},
			Security: []map[string][]string{{"bearerAuth": {}}},
			Parameters: []*huma.Param{{
				Name:     "orgID",
				In:       "path",
				Required: true,
				Schema:   &huma.Schema{Type: "string", Format: "uuid"},
			}},
			Responses: map[string]*huma.Response{
				"200": {
					Description: "The stream",
					Content: map[string]*huma.MediaType{
						"text/event-stream": {Schema: &huma.Schema{Type: "string"}},
					},
				},
				"403": {Description: "Not a member of this organisation"},
			},
		},
	}
}
