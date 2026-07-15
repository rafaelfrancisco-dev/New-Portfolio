// Package static embeds the web assets (htmx and the compiled Tailwind
// stylesheet) into the binary so the app deploys as a single file.
package static

import "embed"

//go:embed css/app.css js/htmx.min.js
var FS embed.FS
