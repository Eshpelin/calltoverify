// Package migrations embeds the ordered SQL schema files so the Coordinator can
// apply them at startup without depending on files being present on disk.
package migrations

import "embed"

// FS holds the numbered *.sql migration files, applied in lexical order.
//
//go:embed *.sql
var FS embed.FS
