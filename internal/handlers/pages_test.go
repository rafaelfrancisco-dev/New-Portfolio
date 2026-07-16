package handlers_test

import (
	"database/sql"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"

	"go-port/internal/auth"
	"go-port/internal/db"
	"go-port/internal/handlers"
)

type testServer struct {
	handler http.Handler
	db      *sql.DB
	uploads string
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

	s := fuego.NewServer()
	handlers.Register(s, handlers.Resource{
		Queries:           db.New(sqlDB),
		AppDB:             sqlDB,
		DBOSDB:            sqlDB,
		AdminUsername:     "admin",
		AdminPasswordHash: mustPasswordHash(t, "correct horse battery staple"),
		UploadDir:         uploadDir,
	})
	return &testServer{handler: s.Mux, db: sqlDB, uploads: uploadDir}
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

	status, body := get(t, ts, "/photography?filter=wildlife")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, "otter, Rio Ceira") || strings.Contains(body, "schist village, dusk") {
		t.Error("wildlife filter should only list wildlife photos")
	}

	// Photo 1 (griffon vulture) is wildlife; the fragment should render its
	// modal with the placeholder EXIF panel.
	status, body = get(t, ts, "/photography/photos/1?filter=wildlife")
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
