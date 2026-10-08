// Package redact removes credential-bearing URLs and OAuth fields from diagnostics.
package redact

import "regexp"

var urls = regexp.MustCompile(`(?i)https?://[^\s"<>]+`)
var secrets = regexp.MustCompile(`(?i)(oauth_(?:token(?:_secret)?|verifier|signature)|api[_-]?secret|password|authorization)[=:][^\s&,;]+`)

func Error(message string) string {
	return secrets.ReplaceAllString(urls.ReplaceAllString(message, "[redacted URL]"), "$1=[redacted]")
}
