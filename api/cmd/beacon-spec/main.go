// Command beacon-spec emits the OpenAPI document and exits.
//
// It builds the router — which is what registers every huma operation — then
// serializes the resulting document. Nothing listens and nothing connects to
// Postgres, so this runs in CI in about a second.
//
// The point is the diff. `make contract` runs this and fails if the output
// differs from the committed openapi.json, which turns "changed a handler,
// forgot to regenerate the client" into a red build rather than a runtime
// surprise somebody reports a week later.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"beacon/internal/config"
	beaconhttp "beacon/internal/http"
)

func main() {
	out := "openapi.json"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	// Silent unless something breaks: this command's output is the file.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	cfg := config.ForSpecGeneration()

	// A nil pool. Registering routes never touches the database; if that ever
	// stops being true this panics loudly, which is far better than emitting a
	// document that quietly disagrees with the running server.
	srv, err := beaconhttp.NewServer(cfg, logger, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "spec: build server: %v\n", err)
		os.Exit(1)
	}
	_ = srv.Routes() // registering every operation is the side effect we want

	if err := srv.WriteSpec(out); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\n", out)
}
