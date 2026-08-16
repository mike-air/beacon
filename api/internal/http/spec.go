package http

// Emitting the contract.
//
// `beacon-api spec` writes openapi.json from the registered operations and
// exits. It starts no listener and opens no database connection, so CI can run
// it in a second and diff the result against what is committed.
//
// That diff is the whole safety mechanism: a handler whose shape changed
// without a regenerated SDK becomes a failing build rather than a 400 that
// somebody eventually reports.

import (
	"encoding/json"
	"fmt"
	"os"
)

// WriteSpec renders the OpenAPI document to path.
//
// It marshals with a stable indent so the committed file diffs cleanly. A
// generated artifact that reorders itself on every run is a generated artifact
// nobody can review.
func (s *Server) WriteSpec(path string) error {
	if s.api == nil {
		return fmt.Errorf("spec: no huma API on this server (routes not built?)")
	}
	doc, err := json.MarshalIndent(s.api.OpenAPI(), "", "  ")
	if err != nil {
		return fmt.Errorf("spec: marshal: %w", err)
	}
	doc = append(doc, '\n')
	if err := os.WriteFile(path, doc, 0o644); err != nil {
		return fmt.Errorf("spec: write %s: %w", path, err)
	}
	return nil
}
