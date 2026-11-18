package handlers_test

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-fuego/fuego"

	"go-port/internal/auth"
	"go-port/internal/db"
	"go-port/internal/flickr"
	"go-port/internal/handlers"
)

type testServer struct {
	public   http.Handler
	private  http.Handler
	resource handlers.Resource
	handler  http.Handler
	db       *sql.DB
	uploads  string
}

// newTestServer spins up the real route table against a throwaway SQLite
// file; migrations include the seed posts, so the journal has content. DBOS
// is left nil: no route uses it.
func newTestServer(t *testing.T) *testServer {
	t.Helper()
	sqlDB, err := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	uploadDir := filepath.Join(t.TempDir(), "uploads")
	cipher, err := flickr.NewCipher("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	flickrService := &flickr.Service{Store: flickr.Store{DB: sqlDB, Cipher: cipher}}
	if err := flickrService.Store.ReplaceSnapshot(t.Context(), []flickr.AlbumPhotos{
		{Album: flickr.Album{ID: "landscapes", Title: "landscapes", Position: 0}, Photos: []flickr.Photo{{ID: "0", Title: "schist village, dusk", PageURL: "https://www.flickr.com/photos/test/0", SmallURL: "https://live.staticflickr.com/0.jpg", LargeURL: "https://live.staticflickr.com/0_b.jpg", Width: 1200, Height: 800}}},
		{Album: flickr.Album{ID: "wildlife", Title: "wildlife", Position: 1}, Photos: []flickr.Photo{{ID: "1", Title: "griffon vulture, Douro", PageURL: "https://www.flickr.com/photos/test/1", SmallURL: "https://live.staticflickr.com/1.jpg", LargeURL: "https://live.staticflickr.com/1_b.jpg", Width: 1200, Height: 800}, {ID: "2", Title: "otter, Rio Ceira", PageURL: "https://www.flickr.com/photos/test/2", SmallURL: "https://live.staticflickr.com/2.jpg", LargeURL: "https://live.staticflickr.com/2_b.jpg", Width: 1200, Height: 800}, {ID: "3", Title: "red kite in flight", PageURL: "https://www.flickr.com/photos/test/3", SmallURL: "https://live.staticflickr.com/3.jpg", LargeURL: "https://live.staticflickr.com/3_b.jpg", Width: 1200, Height: 800}}},
	}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	s := fuego.NewServer(fuego.WithErrorSerializer(handlers.SafeError))
	resource := handlers.Resource{
		PublicBaseURL: "http://example.com", AdminBaseURL: "http://example.com",
		Queries:           db.New(sqlDB),
		AppDB:             sqlDB,
		DBOSDB:            sqlDB,
		AdminUsername:     "admin",
		AdminPasswordHash: mustPasswordHash(t, "correct horse battery staple"),
		UploadDir:         uploadDir,
		Flickr:            flickrService,
	}
	handlers.RegisterPublic(s, resource)
	private := fuego.NewServer(fuego.WithErrorSerializer(handlers.SafeError))
	handlers.RegisterAdmin(private, resource)
	origin, _ := url.Parse("http://example.com")
	publicHandler := handlers.Boundary(s.Mux, origin, origin.String(), false)
	privateHandler := handlers.Boundary(private.Mux, origin, origin.String(), true)
	dispatch := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/admin") {
			privateHandler.ServeHTTP(w, r)
		} else {
			publicHandler.ServeHTTP(w, r)
		}
	})
	return &testServer{handler: dispatch, public: publicHandler, private: privateHandler, resource: resource, db: sqlDB, uploads: uploadDir}
}

func mustPasswordHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func (ts *testServer) do(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	ts.handler.ServeHTTP(recorder, request)
	return recorder
}

func get(t *testing.T, ts *testServer, path string) (int, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "http://example.com"+path, nil)
	response := ts.do(t, request).Result()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}

func TestPagesRender(t *testing.T) {
	ts := newTestServer(t)

	tests := []struct {
		path string
		want string
	}{
		{"/", "I build iOS apps and photograph the wild bits of Portugal."},
		{"/software", "Bird Sound Identifier"},
		{"/photography", "red kite in flight"},
		{"/journal", "Notes from the field"},
		{"/about", "Olá, I'm Rafael."},
		{"/lightroom", "Vision Keywords"},
	}
	for _, tt := range tests {
		status, body := get(t, ts, tt.path)
		if status != http.StatusOK {
			t.Errorf("GET %s status = %d, want 200", tt.path, status)
		}
		if !strings.Contains(body, tt.want) {
			t.Errorf("GET %s: body does not contain %q", tt.path, tt.want)
		}
	}
}

func TestArticleRendersMarkdownBody(t *testing.T) {
	ts := newTestServer(t)

	status, body := get(t, ts, "/journal/photo-location-recorder")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	for _, want := range []string{
		"matching photos to places, on-device", // title
		"<pre",                                 // fenced code block rendered
		"TimeInterval(tolerance",               // code content
		"min read",                             // read-time header
		"← older post",                         // adjacent-post nav (newest post has an older one)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("article body does not contain %q", want)
		}
	}
	if strings.Contains(body, "newer post →") {
		t.Error("newest article should not link to a newer post")
	}
}

func TestArticleNotFound(t *testing.T) {
	ts := newTestServer(t)

	status, _ := get(t, ts, "/journal/no-such-post")
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestJournalCategoryFilter(t *testing.T) {
	ts := newTestServer(t)

	status, body := get(t, ts, "/journal?filter=photography")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, "griffon vultures") {
		t.Error("filtered journal should list photography posts")
	}
	if strings.Contains(body, "Five years of Bird Sound Identifier") {
		t.Error("filtered journal should not list software posts")
	}
}

func TestPhotographyFilterAndModal(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/photography?filter=wildlife&page=2", nil)).Result()
	if resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != "/photography/wildlife?page=2" {
		t.Fatalf("legacy filter = %d %q, want 301 to /photography/wildlife?page=2", resp.StatusCode, resp.Header.Get("Location"))
	}

	status, body := get(t, ts, "/photography/wildlife")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, "otter, Rio Ceira") || strings.Contains(body, "schist village, dusk") {
		t.Error("wildlife filter should only list wildlife photos")
	}

	// Photo 1 (griffon vulture) is wildlife; the fragment should render its
	// modal with the Flickr metadata panel.
	modal := httptest.NewRequest(http.MethodGet, "http://example.com/photography/photos/1?filter=wildlife", nil)
	modal.Header.Set("HX-Request", "true")
	rec := ts.do(t, modal)
	status, body = rec.Code, rec.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("HTMX request should get the bare modal fragment")
	}
	if status != http.StatusOK {
		t.Fatalf("modal status = %d, want 200", status)
	}
	if !strings.Contains(body, "griffon vulture, Douro") || !strings.Contains(body, "EXIF") {
		t.Errorf("modal fragment missing expected content: %s", body)
	}

	// Photo 0 is a landscape, so it is not addressable under the wildlife filter.
	status, _ = get(t, ts, "/photography/photos/0?filter=wildlife")
	if status != http.StatusNotFound {
		t.Fatalf("out-of-filter modal status = %d, want 404", status)
	}
}

func TestPhotographyExpiredCacheIsUnavailable(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.db.Exec(`UPDATE flickr_snapshots SET completed_at=? WHERE status='active'`, time.Now().UTC().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	status, body := get(t, ts, "/photography")
	if status != http.StatusOK || !strings.Contains(body, "temporarily unavailable") {
		t.Fatalf("expired gallery = %d %s", status, body)
	}
	if strings.Contains(body, "red kite in flight") {
		t.Fatal("expired photo leaked")
	}
}

func TestJournalFeed(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/journal/feed.xml", nil)).Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/rss+xml") {
		t.Errorf("Content-Type = %q, want application/rss+xml", ct)
	}

	var feed struct {
		Channel struct {
			Title string `xml:"title"`
			Items []struct {
				Title string `xml:"title"`
				Link  string `xml:"link"`
			} `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		t.Fatalf("decode feed: %v", err)
	}
	if len(feed.Channel.Items) != 4 {
		t.Fatalf("feed items = %d, want 4", len(feed.Channel.Items))
	}
	if !strings.HasSuffix(feed.Channel.Items[0].Link, "/journal/photo-location-recorder") {
		t.Errorf("first item link = %q, want the newest post", feed.Channel.Items[0].Link)
	}
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/healthz", nil)).Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestUnknownAlbumNotFound(t *testing.T) {
	ts := newTestServer(t)
	if status, _ := get(t, ts, "/photography/no-such-album"); status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", status)
	}
}

func TestPhotoPage(t *testing.T) {
	ts := newTestServer(t)
	status, body := get(t, ts, "/photography/photos/1?filter=wildlife")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	for _, want := range []string{"<html", "<h1", "griffon vulture, Douro", "EXIF",
		`<link rel="canonical" href="http://example.com/photography/photos/1">`,
		`"@type":"ImageObject"`, `og:image`} {
		if !strings.Contains(body, want) {
			t.Errorf("photo page missing %q", want)
		}
	}
}

// TestPageMetadata checks every public page has its own title, description
// and canonical URL, and that each JSON-LD block is valid JSON.
func TestPageMetadata(t *testing.T) {
	ts := newTestServer(t)
	titleRe := regexp.MustCompile(`<title>([^<]*)</title>`)
	descRe := regexp.MustCompile(`<meta name="description" content="([^"]*)">`)
	canonRe := regexp.MustCompile(`<link rel="canonical" href="([^"]*)">`)
	ldRe := regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	titles, descs := map[string]string{}, map[string]string{}
	for _, path := range []string{"/", "/software", "/photography", "/photography/wildlife", "/journal", "/journal/photo-location-recorder", "/lightroom", "/about", "/photography/photos/2"} {
		status, body := get(t, ts, path)
		if status != http.StatusOK {
			t.Fatalf("GET %s = %d", path, status)
		}
		title, desc, canon := titleRe.FindStringSubmatch(body), descRe.FindStringSubmatch(body), canonRe.FindStringSubmatch(body)
		if title == nil || desc == nil || canon == nil {
			t.Fatalf("GET %s missing title, description or canonical", path)
		}
		if prev, dup := titles[title[1]]; dup {
			t.Errorf("%s and %s share the title %q", prev, path, title[1])
		}
		if prev, dup := descs[desc[1]]; dup {
			t.Errorf("%s and %s share the description %q", prev, path, desc[1])
		}
		titles[title[1]], descs[desc[1]] = path, path
		if want := "http://example.com" + path; canon[1] != want {
			t.Errorf("GET %s canonical = %q, want %q", path, canon[1], want)
		}
		blocks := ldRe.FindAllStringSubmatch(body, -1)
		if len(blocks) == 0 && path != "/journal" {
			t.Errorf("GET %s has no JSON-LD", path)
		}
		for _, b := range blocks {
			var v map[string]any
			if err := json.Unmarshal([]byte(b[1]), &v); err != nil || v["@context"] != "https://schema.org" {
				t.Errorf("GET %s invalid JSON-LD %q: %v", path, b[1], err)
			}
		}
	}

	_, body := get(t, ts, "/journal?filter=software")
	if !strings.Contains(body, `<meta name="robots" content="noindex,follow">`) {
		t.Error("filtered journal should be noindex")
	}
}

func TestSitemapAndRobots(t *testing.T) {
	ts := newTestServer(t)
	status, body := get(t, ts, "/sitemap.xml")
	if status != http.StatusOK {
		t.Fatalf("sitemap status = %d", status)
	}
	var set struct {
		URLs []struct {
			Loc    string `xml:"loc"`
			Images []struct {
				Loc string `xml:"loc"`
			} `xml:"image"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal([]byte(body), &set); err != nil {
		t.Fatalf("sitemap is not valid XML: %v", err)
	}
	locs := map[string]int{}
	for _, u := range set.URLs {
		locs[u.Loc] = len(u.Images)
	}
	for _, want := range []string{"http://example.com/", "http://example.com/journal/photo-location-recorder", "http://example.com/photography/wildlife", "http://example.com/photography/photos/3"} {
		if _, ok := locs[want]; !ok {
			t.Errorf("sitemap missing %s", want)
		}
	}
	if locs["http://example.com/photography/photos/3"] != 1 {
		t.Error("photo entries should carry an image:image")
	}

	status, body = get(t, ts, "/robots.txt")
	if status != http.StatusOK || !strings.Contains(body, "Disallow: /admin") || !strings.Contains(body, "Sitemap: http://example.com/sitemap.xml") {
		t.Fatalf("robots.txt = %d %q", status, body)
	}
}
