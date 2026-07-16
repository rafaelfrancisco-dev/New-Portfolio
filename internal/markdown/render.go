package markdown

import (
	"bytes"
	"regexp"

	"github.com/cockroachdb/errors"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
)

// Renderer is the single Markdown-to-HTML path used by both public articles
// and the admin preview. Goldmark supports authored figure HTML; bluemonday
// then removes scripts, event handlers, unsafe URLs, and unsupported markup.
type Renderer struct {
	markdown goldmark.Markdown
	policy   *bluemonday.Policy
}

func NewRenderer() *Renderer {
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("figure", "figcaption")
	policy.AllowAttrs("class").Matching(regexp.MustCompile(`^placeholder$`)).OnElements("div")
	policy.AllowAttrs("src", "alt", "title", "width", "height", "loading").OnElements("img")
	policy.AllowRelativeURLs(true)
	policy.AllowURLSchemes("http", "https", "mailto")
	policy.RequireNoFollowOnLinks(false)

	return &Renderer{
		markdown: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(goldmarkhtml.WithUnsafe()),
		),
		policy: policy,
	}
}

func (r *Renderer) Render(source string) (string, error) {
	var output bytes.Buffer
	if err := r.markdown.Convert([]byte(source), &output); err != nil {
		return "", errors.Wrap(err, "convert markdown")
	}
	return string(r.policy.SanitizeBytes(output.Bytes())), nil
}
