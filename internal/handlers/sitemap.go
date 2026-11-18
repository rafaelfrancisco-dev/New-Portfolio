package handlers

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// sitemap lists every public page, post, album and photo. Photo entries carry
// an image sitemap extension so the Flickr-hosted files are credited to the
// photo pages here.
func (rs Resource) sitemap(w http.ResponseWriter, r *http.Request) {
	base := strings.TrimRight(rs.PublicBaseURL, "/")
	set := urlSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", ImageNS: "http://www.google.com/schemas/sitemap-image/1.1"}
	for _, path := range []string{"/", "/software", "/photography", "/journal", "/lightroom", "/about"} {
		set.URLs = append(set.URLs, sitemapURL{Loc: base + path})
	}

	posts, err := rs.Queries.ListPosts(r.Context())
	if err != nil {
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	for _, p := range posts {
		set.URLs = append(set.URLs, sitemapURL{Loc: base + "/journal/" + url.PathEscape(p.Slug), LastMod: p.UpdatedAt.UTC().Format("2006-01-02")})
	}

	if rs.Flickr != nil {
		now := time.Now().UTC()
		albums, err := rs.Flickr.Store.Albums(r.Context(), now)
		if err != nil {
			http.Error(w, "failed to list albums", http.StatusInternalServerError)
			return
		}
		for _, a := range albums {
			set.URLs = append(set.URLs, sitemapURL{Loc: base + "/photography/" + url.PathEscape(a.Slug)})
		}
		photos, err := rs.Flickr.Store.AllPhotos(r.Context(), now)
		if err != nil {
			http.Error(w, "failed to list photos", http.StatusInternalServerError)
			return
		}
		for _, p := range photos {
			entry := sitemapURL{Loc: base + "/photography/photos/" + url.PathEscape(p.ID)}
			if p.LargeURL != "" {
				entry.Images = []sitemapImage{{Loc: p.LargeURL}}
			}
			set.URLs = append(set.URLs, entry)
		}
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(set)
}

func (rs Resource) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("User-agent: *\nDisallow: /admin\nDisallow: /swagger\n\nSitemap: " + strings.TrimRight(rs.PublicBaseURL, "/") + "/sitemap.xml\n"))
}

type urlSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	ImageNS string       `xml:"xmlns:image,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string         `xml:"loc"`
	LastMod string         `xml:"lastmod,omitempty"`
	Images  []sitemapImage `xml:"image:image"`
}

type sitemapImage struct {
	Loc string `xml:"image:loc"`
}
