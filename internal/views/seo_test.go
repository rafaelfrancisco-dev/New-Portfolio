package views

import (
	"testing"

	"go-port/internal/flickr"
)

func TestFlickrSized(t *testing.T) {
	for in, want := range map[string]string{
		"https://live.staticflickr.com/65535/123_abc.jpg":   "https://live.staticflickr.com/65535/123_abc_b.jpg",
		"https://live.staticflickr.com/65535/123_abc_z.jpg": "https://live.staticflickr.com/65535/123_abc_b.jpg",
		"https://live.staticflickr.com/0.jpg":               "https://live.staticflickr.com/0.jpg",
	} {
		if got := flickrSized(in, "b"); got != want {
			t.Errorf("flickrSized(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPhotoCaptionAndAlt(t *testing.T) {
	p := flickr.Photo{Title: "Red kite", Location: "Lousã, Coimbra, Portugal", TakenAt: "2024-03-02 09:10:00"}
	if got := photoCaption(p); got != "Lousã, Coimbra, Portugal (2024)" {
		t.Errorf("caption = %q", got)
	}
	if got := photoAlt(p); got != "Red kite, Lousã, Coimbra, Portugal" {
		t.Errorf("alt = %q", got)
	}
	p.Location = "40.11, -8.24"
	if got := photoCaption(p); got != "2024" {
		t.Errorf("coordinates should not be used as a place, caption = %q", got)
	}
	p = flickr.Photo{Title: "Fog over Lousã", Location: "Lousã, Portugal"}
	if got := photoAlt(p); got != "Fog over Lousã" {
		t.Errorf("place already in the title should not repeat, alt = %q", got)
	}
}
