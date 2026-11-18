package views

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go-port/internal/content"
	"go-port/internal/db"
	"go-port/internal/flickr"
)

// Meta is what a page puts in <head> for search engines and link previews.
type Meta struct {
	Title       string
	Description string // falls back to the site description when empty
	Path        string // canonical path, e.g. "/software"; empty means no canonical
	Image       string // absolute image URL for og:image
	ImageAlt    string
	Type        string // og:type; "website" when empty
	Published   time.Time
	NoIndex     bool
	JSONLD      []any
}

func (m Meta) description(settings db.SiteSetting) string {
	if m.Description != "" {
		return m.Description
	}
	return settings.SiteDescription
}

func (m Meta) ogType() string {
	if m.Type != "" {
		return m.Type
	}
	return "website"
}

// absURL joins the configured public origin with a site path.
func absURL(ctx context.Context, path string) string {
	return strings.TrimRight(publicOrigin(ctx), "/") + path
}

type ld = map[string]any

func person(ctx context.Context, settings db.SiteSetting) ld {
	p := ld{
		"@type":    "Person",
		"name":     settings.DisplayName,
		"url":      absURL(ctx, "/about"),
		"jobTitle": "Mobile Tech Lead",
		"address": ld{
			"@type":           "PostalAddress",
			"addressLocality": "Lousã",
			"addressRegion":   "Coimbra",
			"addressCountry":  "PT",
		},
	}
	var same []string
	for _, u := range []string{settings.GithubUrl, settings.LinkedinUrl, settings.InstagramUrl} {
		if u != "" {
			same = append(same, u)
		}
	}
	if len(same) > 0 {
		p["sameAs"] = same
	}
	return p
}

func withContext(v ld) ld {
	v["@context"] = "https://schema.org"
	return v
}

func breadcrumbs(ctx context.Context, items ...[2]string) ld {
	list := make([]ld, 0, len(items))
	for i, item := range items {
		list = append(list, ld{"@type": "ListItem", "position": i + 1, "name": item[0], "item": absURL(ctx, item[1])})
	}
	return withContext(ld{"@type": "BreadcrumbList", "itemListElement": list})
}

func homeMeta(ctx context.Context, settings db.SiteSetting, photo flickr.Photo, hasPhoto bool) Meta {
	m := Meta{
		Title: settings.SiteTitle,
		Path:  "/",
		JSONLD: []any{
			withContext(ld{"@type": "WebSite", "name": settings.SiteTitle, "url": absURL(ctx, "/")}),
			withContext(person(ctx, settings)),
		},
	}
	if hasPhoto {
		m.Image, m.ImageAlt = photo.LargeURL, photoAlt(photo)
	}
	return m
}

func softwareMeta(ctx context.Context, settings db.SiteSetting, projects []content.Project) Meta {
	var apps []ld
	for _, p := range projects {
		if p.Retired || p.URL == "" {
			continue
		}
		apps = append(apps, ld{
			"@type":    "ListItem",
			"position": len(apps) + 1,
			"item": ld{
				"@type":               "SoftwareApplication",
				"name":                p.Name,
				"description":         p.Desc,
				"operatingSystem":     projectOS(p),
				"applicationCategory": "MobileApplication",
				"url":                 p.URL,
				"author":              ld{"@type": "Person", "name": settings.DisplayName},
			},
		})
	}
	return Meta{
		Title:       "iOS, watchOS and Android apps by " + settings.DisplayName,
		Description: "Apps I built on my own for iOS, watchOS and Android, including Bird Sound Identifier, Photo Location Recorder and Car Model Identifier.",
		Path:        "/software",
		JSONLD:      []any{withContext(ld{"@type": "ItemList", "itemListElement": apps})},
	}
}

// projectOS reads the platforms out of a project's tech line.
func projectOS(p content.Project) string {
	var out []string
	for _, os := range []string{"iOS", "watchOS", "Android"} {
		if strings.Contains(p.Tech, os) {
			out = append(out, os)
		}
	}
	if len(out) == 0 {
		return "iOS"
	}
	return strings.Join(out, ", ")
}

func lightroomMeta(ctx context.Context, settings db.SiteSetting, plugins []content.Plugin) Meta {
	names := make([]string, 0, len(plugins))
	var data []any
	for _, p := range plugins {
		names = append(names, p.Name)
		data = append(data, withContext(ld{
			"@type":               "SoftwareApplication",
			"name":                p.Name + " for Lightroom Classic",
			"description":         p.Tagline,
			"softwareVersion":     p.Version,
			"operatingSystem":     "macOS",
			"applicationCategory": "MultimediaApplication",
			"downloadUrl":         p.DownloadURL,
			"url":                 absURL(ctx, "/lightroom"),
			"license":             "https://opensource.org/licenses/" + p.License,
			"author":              ld{"@type": "Person", "name": settings.DisplayName},
			"offers":              ld{"@type": "Offer", "price": "0", "priceCurrency": "EUR"},
		}))
	}
	return Meta{
		Title:       "Lightroom Classic plug-ins: " + strings.Join(names, " and "),
		Description: "Two free and open source plug-ins for Lightroom Classic on macOS: publish to iCloud Photos and add keywords with Apple's on-device Vision classifier.",
		Path:        "/lightroom",
		JSONLD:      data,
	}
}

func aboutMeta(ctx context.Context, settings db.SiteSetting) Meta {
	return Meta{
		Title:       "About " + settings.DisplayName + ", iOS developer in Lousã, Portugal",
		Description: "Software developer from Lousã, near Coimbra, Portugal, building native mobile apps since 2016. Prints of my photos are available in A3 and A2.",
		Path:        "/about",
		Type:        "profile",
		JSONLD:      []any{withContext(ld{"@type": "ProfilePage", "mainEntity": person(ctx, settings)})},
	}
}

func journalMeta(settings db.SiteSetting, filter string, page int) Meta {
	m := Meta{Title: "Journal - " + settings.DisplayName, Description: settings.JournalDescription, Path: "/journal"}
	if filter != "all" {
		// Category views repeat posts already on /journal.
		m.NoIndex = true
	} else if page > 1 {
		m.Path = "/journal?page=" + strconv.Itoa(page)
		m.Title = fmt.Sprintf("Journal, page %d - %s", page, settings.DisplayName)
	}
	return m
}

func articleMeta(ctx context.Context, settings db.SiteSetting, post db.Post) Meta {
	path := "/journal/" + post.Slug
	posting := ld{
		"@type":            "BlogPosting",
		"headline":         post.Title,
		"description":      post.Teaser,
		"url":              absURL(ctx, path),
		"mainEntityOfPage": absURL(ctx, path),
		"author":           person(ctx, settings),
		"dateModified":     post.UpdatedAt.UTC().Format(time.RFC3339),
		"articleSection":   post.Category,
	}
	m := Meta{Title: post.Title, Description: post.Teaser, Path: path, Type: "article"}
	if post.PublishedAt.Valid {
		m.Published = post.PublishedAt.Time
		posting["datePublished"] = post.PublishedAt.Time.UTC().Format(time.RFC3339)
	}
	m.JSONLD = []any{withContext(posting), breadcrumbs(ctx, [2]string{"Journal", "/journal"}, [2]string{post.Title, path})}
	return m
}

// currentAlbum is the album a gallery is filtered to, if any.
func currentAlbum(g flickr.Gallery) (flickr.Album, bool) {
	for _, a := range g.Albums {
		if a.ID == g.Filter {
			return a, true
		}
	}
	return flickr.Album{}, false
}

// galleryPath is the canonical path of a gallery view.
func galleryPath(g flickr.Gallery) string {
	if a, ok := currentAlbum(g); ok {
		return "/photography/" + url.PathEscape(a.Slug)
	}
	return "/photography"
}

func galleryPagePath(g flickr.Gallery, page int) string {
	if page > 1 {
		return galleryPath(g) + "?page=" + strconv.Itoa(page)
	}
	return galleryPath(g)
}

func photographyMeta(ctx context.Context, settings db.SiteSetting, g flickr.Gallery) Meta {
	m := Meta{
		Title:       "Wildlife and landscape photography from Portugal - " + settings.DisplayName,
		Description: "Wildlife and landscape photos, mostly from Portugal, with camera, lens and exposure details for each one.",
	}
	if a, ok := currentAlbum(g); ok {
		m.Title = a.Title + " photos - " + settings.DisplayName
		m.Description = "Photos from my " + a.Title + " album, with camera, lens and exposure details for each one."
		if d := strings.TrimSpace(a.Description); d != "" {
			m.Description = truncate(d, 160)
		}
	}
	if g.Available {
		m.Path = galleryPagePath(g, g.Page)
		crumbs := [][2]string{{"Photography", "/photography"}}
		if a, ok := currentAlbum(g); ok {
			crumbs = append(crumbs, [2]string{a.Title, galleryPath(g)})
		}
		m.JSONLD = []any{
			withContext(ld{"@type": "CollectionPage", "name": m.Title, "description": m.Description, "url": absURL(ctx, m.Path), "author": ld{"@type": "Person", "name": settings.DisplayName}}),
			breadcrumbs(ctx, crumbs...),
		}
		if g.Page > 1 {
			m.Title = fmt.Sprintf("%s, page %d", m.Title, g.Page)
		}
	}
	if len(g.Photos) > 0 {
		m.Image, m.ImageAlt = g.Photos[0].LargeURL, photoAlt(g.Photos[0])
	}
	return m
}

func photoPath(id string) string { return "/photography/photos/" + url.PathEscape(id) }

// place is the photo's location name, or "" when Flickr only gave raw
// coordinates.
func place(p flickr.Photo) string {
	loc := strings.TrimSpace(p.Location)
	if loc == "" || strings.Trim(loc, "0123456789.,- ") == "" {
		return ""
	}
	return loc
}

func photoYear(p flickr.Photo) string {
	if len(p.TakenAt) >= 4 {
		if _, err := strconv.Atoi(p.TakenAt[:4]); err == nil {
			return p.TakenAt[:4]
		}
	}
	return ""
}

// photoCaption is the Ken Rockwell style line under a photo: "Place (Year)".
func photoCaption(p flickr.Photo) string {
	pl, year := place(p), photoYear(p)
	switch {
	case pl != "" && year != "":
		return pl + " (" + year + ")"
	case pl != "":
		return pl
	default:
		return year
	}
}

// photoAlt describes a photo with its title and, when it isn't already in
// the title, where it was taken.
func photoAlt(p flickr.Photo) string {
	pl := place(p)
	if pl == "" || strings.Contains(strings.ToLower(p.Title), strings.ToLower(strings.SplitN(pl, ",", 2)[0])) {
		return p.Title
	}
	return p.Title + ", " + pl
}

func photoTitle(p flickr.Photo) string {
	title := photoAlt(p)
	if y := photoYear(p); y != "" {
		title += " (" + y + ")"
	}
	return title
}

func photoDescription(p flickr.Photo) string {
	if d := strings.TrimSpace(p.Description); d != "" {
		return truncate(d, 160)
	}
	desc := p.Title
	if pl := place(p); pl != "" {
		desc += ", photographed in " + pl
	}
	if y := photoYear(p); y != "" {
		desc += " in " + y
	}
	if p.Camera != "" {
		desc += " with a " + p.Camera
		if p.Lens != "" {
			desc += " and " + p.Lens
		}
	}
	return desc + "."
}

func photoMeta(ctx context.Context, settings db.SiteSetting, p flickr.Photo) Meta {
	path := photoPath(p.ID)
	img := ld{
		"@type":              "ImageObject",
		"name":               p.Title,
		"description":        photoDescription(p),
		"contentUrl":         p.LargeURL,
		"thumbnailUrl":       p.SmallURL,
		"url":                absURL(ctx, path),
		"width":              p.Width,
		"height":             p.Height,
		"creator":            ld{"@type": "Person", "name": settings.DisplayName},
		"creditText":         settings.DisplayName,
		"copyrightNotice":    "© " + settings.DisplayName,
		"license":            absURL(ctx, "/about"),
		"acquireLicensePage": absURL(ctx, "/about"),
	}
	if len(p.TakenAt) >= 10 {
		img["dateCreated"] = p.TakenAt[:10]
	}
	if pl := place(p); pl != "" {
		img["contentLocation"] = ld{"@type": "Place", "name": pl}
	}
	var exif []ld
	for _, kv := range [][2]string{{"Camera", p.Camera}, {"Lens", p.Lens}, {"Exposure time", p.Exposure}, {"Aperture", p.Aperture}, {"ISO", p.ISO}, {"Focal length", p.FocalLength}} {
		if kv[1] != "" {
			exif = append(exif, ld{"@type": "PropertyValue", "name": kv[0], "value": kv[1]})
		}
	}
	if len(exif) > 0 {
		img["exifData"] = exif
	}
	return Meta{
		Title:       photoTitle(p),
		Description: photoDescription(p),
		Path:        path,
		Image:       p.LargeURL,
		ImageAlt:    photoAlt(p),
		JSONLD:      []any{withContext(img), breadcrumbs(ctx, [2]string{"Photography", "/photography"}, [2]string{p.Title, path})},
	}
}

// flickrSized swaps the size suffix of a Flickr static URL. Medium (500px)
// URLs have no suffix; "_z" is 640, "_c" 800 and "_b" 1024 on the long edge,
// all sharing the same secret. Unknown shapes come back unchanged.
func flickrSized(raw, size string) string {
	if !strings.HasSuffix(raw, ".jpg") {
		return raw
	}
	base := strings.TrimSuffix(raw, ".jpg")
	slash := strings.LastIndex(base, "/")
	name := base[slash+1:]
	if parts := strings.Split(name, "_"); len(parts) == 3 {
		base = base[:slash+1] + parts[0] + "_" + parts[1]
	} else if len(parts) != 2 {
		return raw
	}
	return base + "_" + size + ".jpg"
}

// gridSrcset offers sizes up to 1024px instead of jumping to the original.
func gridSrcset(p flickr.Photo) string {
	small := "500w"
	if strings.HasSuffix(p.SmallURL, "_z.jpg") {
		small = "640w"
	}
	return p.SmallURL + " " + small + ", " + flickrSized(p.SmallURL, "c") + " 800w, " + flickrSized(p.SmallURL, "b") + " 1024w"
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndex(string(r), " "); i > n/2 {
		return string(r)[:i] + "..."
	}
	return string(r) + "..."
}
