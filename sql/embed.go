// Package sqlassets embeds the goose schema migrations so binaries
// (cmd/database) can apply them without the sql/ directory on disk.
package sqlassets

import "embed"

//go:embed schema/*.sql
var SchemaFS embed.FS
