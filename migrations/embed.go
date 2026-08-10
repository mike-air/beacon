// Package migrations embeds the raw .sql files into the binary so the service
// can migrate itself on boot — no separate files to ship next to the binary.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS
