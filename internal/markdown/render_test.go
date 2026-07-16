package markdown

import (
	"strings"
	"testing"
)

func TestRendererPreservesSupportedContentAndSanitizesUnsafeHTML(t *testing.T) {
	renderer := NewRenderer()
	html, err := renderer.Render(`
# Heading

[safe](https://example.com) [bad](javascript:alert(1))

![upload](/media/random.jpg)

<figure onclick="alert(1)">
<div class="placeholder" onmouseover="alert(1)">placeholder</div>
<figcaption>caption</figcaption>
</figure>

<script>alert(1)</script>
<iframe src="https://example.com"></iframe>

~~~go
fmt.Println("hello")
~~~
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<h1>Heading</h1>",
		`href="https://example.com"`,
		`src="/media/random.jpg"`,
		"<figure>",
		`class="placeholder"`,
		"<figcaption>caption</figcaption>",
		"<pre><code",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML does not contain %q:\n%s", want, html)
		}
	}
	for _, unsafe := range []string{"javascript:", "<script", "<iframe", "onclick", "onmouseover"} {
		if strings.Contains(strings.ToLower(html), unsafe) {
			t.Errorf("rendered HTML contains unsafe content %q:\n%s", unsafe, html)
		}
	}
}
