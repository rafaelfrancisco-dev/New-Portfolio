// Package static embeds the web assets (htmx and page scripts, the compiled Tailwind
// stylesheet and self-hosted fonts) into the binary so the app deploys as a
// single file.
package static

import "embed"

//go:embed css/app.css js fonts favicon.svg
var FS embed.FS
