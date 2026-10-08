package handlers_test

// Regressions converted from the bounded, local-only security audit.
import (
	"fmt"
	"github.com/go-fuego/fuego"
	"go-port/internal/db"
	"go-port/internal/handlers"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func auditLoginRequest(ip string) *http.Request {
	form := url.Values{"username": {"admin"}, "password": {"incorrect"}, "csrf_token": {"audit-token"}}
	r := httptest.NewRequest("POST", "http://example.com/admin/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://example.com")
	r.RemoteAddr = "192.0.2.10:12345"
	if ip != "" {
		r.Header.Set("X-Forwarded-For", ip)
	}
	r.AddCookie(&http.Cookie{Name: "portfolio_login_csrf", Value: "audit-token"})
	return r
}

func TestSecurityAuditWriteRouteGuards(t *testing.T) {
	s := newTestServer(t)
	c := loginAdmin(t, s)
	paths := []string{"/admin/logout", "/admin/posts", "/admin/posts/1", "/admin/posts/1/delete", "/admin/posts/preview", "/admin/media", "/admin/media/1/delete", "/admin/settings", "/admin/flickr/connect", "/admin/flickr/sync", "/admin/flickr/disconnect"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			if got := s.do(t, httptest.NewRequest("POST", "http://example.com"+path, nil)).Code; got != http.StatusSeeOther {
				t.Fatalf("anonymous request returned %d", got)
			}
			if got := c.request(t, "POST", path, nil, false).Code; got != http.StatusForbidden {
				t.Fatalf("missing CSRF returned %d", got)
			}
			form := url.Values{"csrf_token": {c.csrf}}
			r := httptest.NewRequest("POST", "http://example.com"+path, strings.NewReader(form.Encode()))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", "https://attacker.invalid")
			r.AddCookie(c.session)
			if got := s.do(t, r).Code; got != http.StatusForbidden {
				t.Fatalf("cross-origin request returned %d", got)
			}
		})
	}
	for _, path := range []string{"/admin", "/admin/posts", "/admin/posts/new", "/admin/posts/1/edit", "/admin/media", "/admin/settings", "/admin/flickr", "/admin/flickr/callback?state=x&oauth_token=x&oauth_verifier=x"} {
		if got := s.do(t, httptest.NewRequest("GET", "http://example.com"+path, nil)).Code; got != http.StatusSeeOther {
			t.Errorf("anonymous GET %s returned %d", path, got)
		}
	}
	var media, sessions int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM media").Scan(&media); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM admin_sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if media != 0 || sessions != 1 {
		t.Fatalf("unexpected writes: media=%d sessions=%d", media, sessions)
	}
	t.Log("All 11 admin POST routes reject anonymous, missing-CSRF and foreign-Origin requests; admin reads and callback require authentication.")
}

func TestForwardedIPCannotBypassThrottle(t *testing.T) {
	s := newTestServer(t)
	for i := 0; i < 5; i++ {
		if got := s.do(t, auditLoginRequest("")).Code; got != 401 {
			t.Fatalf("attempt %d: %d", i, got)
		}
	}
	for _, ip := range []string{"", "198.51.100.25", "127.0.0.1, 198.51.100.26"} {
		if got := s.do(t, auditLoginRequest(ip)).Code; got != 429 {
			t.Fatalf("spoof bypass: %d", got)
		}
	}
}
func TestConcurrentReservations(t *testing.T) {
	s := newTestServer(t)
	for _, global := range []bool{false, true} {
		if _, err := s.db.Exec("DELETE FROM login_attempts"); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		results := make(chan bool, 60)
		errs := make(chan error, 60)
		start := make(chan struct{})
		for i := 0; i < 60; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				ip := "same"
				if global {
					ip = fmt.Sprint(i)
				}
				ok, err := db.ReserveLogin(t.Context(), s.db, ip)
				results <- ok
				errs <- err
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		count := 0
		for ok := range results {
			if ok {
				count++
			}
		}
		want := 5
		if global {
			want = 30
		}
		if count != want {
			t.Fatalf("reserved %d, want %d", count, want)
		}
	}
}
func TestLoginBodyLimits(t *testing.T) {
	s := newTestServer(t)
	cases := []struct {
		name, content, body string
		want                int
	}{
		{"oversized", "application/x-www-form-urlencoded", "csrf_token=a&password=" + strings.Repeat("x", 17<<10), 413},
		{"multipart", "multipart/form-data; boundary=x", "--x", 415},
		{"malformed", "application/x-www-form-urlencoded", "x=%xx", 400},
		{"username", "application/x-www-form-urlencoded", url.Values{"csrf_token": {"audit-token"}, "username": {strings.Repeat("x", 129)}}.Encode(), 400},
		{"password", "application/x-www-form-urlencoded", url.Values{"csrf_token": {"audit-token"}, "password": {strings.Repeat("x", 1025)}}.Encode(), 400},
	}
	for _, c := range cases {
		for _, chunked := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/chunked=%v", c.name, chunked), func(t *testing.T) {
				r := auditLoginRequest("")
				r.Body = io.NopCloser(strings.NewReader(c.body))
				r.ContentLength = int64(len(c.body))
				if chunked {
					r.ContentLength = -1
					r.TransferEncoding = []string{"chunked"}
				}
				r.Header.Set("Content-Type", c.content)
				if got := s.do(t, r).Code; got != c.want {
					t.Fatalf("got %d want %d", got, c.want)
				}
			})
		}
	}
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM login_attempts").Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid bodies reserved work: %d %v", count, err)
	}
}
func TestPublicAdminDenied(t *testing.T) {
	s := newTestServer(t)
	c := loginAdmin(t, s)
	paths := []string{"/admin", "/admin/", "/admin/login", "/admin/posts", "/admin/flickr/callback", "/%61dmin", "/admin%2flogin"}
	for _, path := range paths {
		for _, method := range []string{"GET", "HEAD", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"} {
			r := httptest.NewRequest(method, "http://example.com"+path, nil)
			r.AddCookie(c.session)
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("X-Forwarded-Host", "private.invalid")
			w := httptest.NewRecorder()
			s.public.ServeHTTP(w, r)
			if w.Code != 404 {
				t.Errorf("%s %s: %d", method, path, w.Code)
			}
		}
	}
	for _, path := range []string{"/nonexistent", "/swagger", "/swagger/openapi.json", "/static/", "/static/js/"} {
		w := httptest.NewRecorder()
		s.public.ServeHTTP(w, httptest.NewRequest("GET", "http://example.com"+path, nil))
		if w.Code != 404 {
			t.Errorf("%s = %d", path, w.Code)
		}
	}
}
func TestRevocationRotationAndLogoutFailure(t *testing.T) {
	s := newTestServer(t)
	c := loginAdmin(t, s)
	rotated := s.resource
	rotated.AdminPasswordHash = "changed"
	server := fuego.NewServer()
	handlers.RegisterAdmin(server, rotated)
	r := httptest.NewRequest("GET", "http://example.com/admin", nil)
	r.AddCookie(c.session)
	w := httptest.NewRecorder()
	server.Mux.ServeHTTP(w, r)
	if w.Code != 303 {
		t.Fatalf("rotated session: %d", w.Code)
	}
	// The rotated-resource middleware lazily deletes the (shared) session row
	// it just found invalid, so the original cookie is now gone from the DB.
	// Get a fresh session before exercising revoke-all.
	c = loginAdmin(t, s)
	if _, err := s.db.Exec(`INSERT INTO flickr_oauth_requests(state_hash,admin_session_hash,request_token,secret_ciphertext,expires_at) VALUES('state','session','token',x'00',datetime('now','+1 hour'))`); err != nil {
		t.Fatal(err)
	}
	if got := c.request(t, "POST", "/admin/revoke-all", nil, true).Code; got != 303 {
		t.Fatalf("revoke: %d", got)
	}
	for _, table := range []string{"admin_sessions", "flickr_oauth_requests"} {
		var n int
		if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: %d %v", table, n, err)
		}
	}
	if got := c.request(t, "GET", "/admin", nil, false).Code; got != 303 {
		t.Fatalf("revoked: %d", got)
	}
	c = loginAdmin(t, s)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_logout BEFORE DELETE ON admin_sessions BEGIN SELECT RAISE(FAIL,'test'); END`); err != nil {
		t.Fatal(err)
	}
	if got := c.request(t, "POST", "/admin/logout", nil, true).Code; got != 503 {
		t.Fatalf("failed logout: %d", got)
	}
}
func TestHostErrorsHeadersAndLimiterFailure(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest("GET", "http://attacker.invalid/journal/feed.xml", nil)
	if got := s.do(t, r).Code; got != 421 {
		t.Fatalf("host spoof: %d", got)
	}
	r = httptest.NewRequest("GET", "http://example.com/journal/feed.xml", nil)
	r.Header.Set("X-Forwarded-Host", "attacker.invalid")
	r.Header.Set("X-Forwarded-Proto", "https")
	if body := s.do(t, r).Body.String(); strings.Contains(body, "attacker.invalid") || !strings.Contains(body, "http://example.com/journal") {
		t.Fatal("RSS origin changed")
	}
	for _, path := range []string{"/admin/login", "/admin", "/admin/missing"} {
		w := s.do(t, httptest.NewRequest("GET", "http://example.com"+path, nil))
		for k, v := range map[string]string{"Cache-Control": "no-store", "X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
			if w.Header().Get(k) != v {
				t.Errorf("%s missing %s", path, k)
			}
		}
	}
	if _, err := s.db.Exec("DROP TABLE login_attempts"); err != nil {
		t.Fatal(err)
	}
	if got := s.do(t, auditLoginRequest("")).Code; got != 503 {
		t.Fatalf("limiter failed open: %d", got)
	}
	if _, err := s.db.Exec("DROP TABLE posts"); err != nil {
		t.Fatal(err)
	}
	for _, accept := range []string{"text/html", "application/json", "text/plain"} {
		r := httptest.NewRequest("GET", "http://example.com/", nil)
		r.Header.Set("Accept", accept)
		w := s.do(t, r)
		if w.Code != 500 || strings.Contains(w.Body.String(), "SQL") || strings.Contains(w.Body.String(), "posts") {
			t.Fatalf("unsafe error: %d %s", w.Code, w.Body.String())
		}
	}
}
