// Package docs embeds the documentation tree so the gantry CLI can
// browse it offline (gantry docs).
package docs

import "embed"

// FS holds every docs page. Paths are relative to this directory,
// e.g. "getting-started/first-app.md".
//
//go:embed README.md */*.md manifest.json
var FS embed.FS
