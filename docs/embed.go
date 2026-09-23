// Package docs embeds the Markdown documentation the server and MCP tools serve, so a packaged
// binary works without a docs/ directory beside it. The on-disk copy still wins when present, so
// the docs stay editable in a source checkout.
package docs

import "embed"

//go:embed *.md agent/*.md
var FS embed.FS
