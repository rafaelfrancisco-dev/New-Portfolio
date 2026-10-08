package views

import (
	"context"
	"strings"

	"go-port/internal/db"
	"go-port/internal/flickr"
)

// vtName builds the inline declaration that makes an element a shared view
// transition participant. The name has to match on both sides of a navigation
// for the browser to morph one element into the other, so the prefix and the
// identity field are chosen once here rather than at each call site. The prefix
// also keeps the custom-ident valid when raw starts with a digit (Flickr photo
// IDs do), and anything outside [A-Za-z0-9-] folds to "-".
func vtName(prefix, raw string) string {
	return "view-transition-name: " + prefix + "-" + strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '-'
		}
	}, raw)
}

// vtPost pairs a post title in a list with the same title on its article page.
func vtPost(post db.Post) string { return vtName("post", post.Slug) }

// vtPhoto pairs the home hero with the same photo's tile in the gallery.
func vtPhoto(photo flickr.Photo) string { return vtName("photo", photo.ID) }

func formatPostDate(post db.Post) string {
	if !post.PublishedAt.Valid {
		return ""
	}
	return post.PublishedAt.Time.Format("2006-01-02")
}

type publicOriginKey struct{}

func WithPublicOrigin(ctx context.Context, origin string) context.Context {
	return context.WithValue(ctx, publicOriginKey{}, origin)
}
func publicOrigin(ctx context.Context) string {
	value, _ := ctx.Value(publicOriginKey{}).(string)
	if value == "" {
		return "/"
	}
	return value
}
