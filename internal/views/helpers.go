package views

import (
	"strings"

	"go-port/internal/db"
)

// vtName builds a CSS custom-ident for view-transition-name. The prefix keeps
// the result valid when raw starts with a digit (Flickr photo IDs do), and
// anything outside [A-Za-z0-9-] folds to "-".
func vtName(prefix, raw string) string {
	var b strings.Builder
	b.Grow(len(prefix) + len(raw) + 1)
	b.WriteString(prefix)
	b.WriteByte('-')
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// vtStyle is the inline declaration that makes an element a shared transition
// participant, e.g. vtStyle("post", "a-slug") -> "view-transition-name: post-a-slug".
// The name must match on both sides of a navigation for the browser to morph
// one element into the other.
func vtStyle(prefix, raw string) string {
	return "view-transition-name: " + vtName(prefix, raw)
}

func formatPostDate(post db.Post) string {
	if !post.PublishedAt.Valid {
		return ""
	}
	return post.PublishedAt.Time.Format("2006-01-02")
}
