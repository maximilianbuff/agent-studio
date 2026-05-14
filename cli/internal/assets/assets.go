// Package assets embeds the default files that studio install copies into
// ~/.agent-studio/ on first run.
package assets

import "embed"

//go:embed all:files
var FS embed.FS
