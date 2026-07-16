package handlers_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

var csrfInput = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

type adminClient struct {
	server  *testServer
	session *http.Cookie
	csrf    string
}

func (c *adminClient) request(t *testing.T, method string, path string, form url.Values, includeCSRF bool) *httptest.ResponseRecorder {
	t.Helper()
	if form == nil {
		form = url.Values{}
	}
	if includeCSRF {
		form.Set("csrf_token", c.csrf)
	}
	request := httptest.NewRequest(method, "http://example.com"+path, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if method != http.MethodGet {
		request.Header.Set("Origin", "http://example.com")
	}
	if c.session != nil {
		request.AddCookie(c.session)
	}
	return c.server.do(t, request)
}

func loginAdmin(t *testing.T, server *testServer) *adminClient {
	t.Helper()
	getLogin := server.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/admin/login", nil))
	match := csrfInput.FindStringSubmatch(getLogin.Body.String())
	if len(match) != 2 {
		t.Fatalf("login CSRF token not found: %s", getLogin.Body.String())
	}
	var loginCookie *http.Cookie
	for _, cookie := range getLogin.Result().Cookies() {
		if cookie.Name == "portfolio_login_csrf" {
			loginCookie = cookie
		}
	}
	if loginCookie == nil {
		t.Fatal("login CSRF cookie missing")
	}
	form := url.Values{
		"username":   {"admin"},
		"password":   {"correct horse battery staple"},
		"csrf_token": {match[1]},
	}
	request := httptest.NewRequest(http.MethodPost, "http://example.com/admin/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(loginCookie)
	response := server.do(t, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	client := &adminClient{server: server}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "portfolio_admin_session" {
			client.session = cookie
		}
	}
	if client.session == nil {
		t.Fatal("admin session cookie missing")
	}
	if err := server.db.QueryRow("SELECT csrf_token FROM admin_sessions LIMIT 1").Scan(&client.csrf); err != nil {
		t.Fatal(err)
	}
	return client
}

func TestAdminAuthorizationLoginLogoutAndCookieSecurity(t *testing.T) {
	server := newTestServer(t)
	unauthorized := server.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/admin", nil))
	if unauthorized.Code != http.StatusSeeOther || unauthorized.Header().Get("Location") != "/admin/login" {
		t.Fatalf("unauthorized response = %d %q", unauthorized.Code, unauthorized.Header().Get("Location"))
	}

	client := loginAdmin(t, server)
	if !client.session.HttpOnly || client.session.SameSite != http.SameSiteStrictMode || client.session.Secure {
		t.Fatalf("unexpected HTTP cookie flags: %+v", client.session)
	}
	dashboard := client.request(t, http.MethodGet, "/admin", nil, false)
	if dashboard.Code != http.StatusOK || !strings.Contains(dashboard.Body.String(), "Dashboard") {
		t.Fatalf("dashboard status = %d", dashboard.Code)
	}

	noCSRF := client.request(t, http.MethodPost, "/admin/logout", nil, false)
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF = %d, want 403", noCSRF.Code)
	}
	logout := client.request(t, http.MethodPost, "/admin/logout", nil, true)
	if logout.Code != http.StatusSeeOther {
		t.Fatalf("logout = %d, want 303", logout.Code)
	}
	afterLogout := client.request(t, http.MethodGet, "/admin", nil, false)
	if afterLogout.Code != http.StatusSeeOther {
		t.Fatalf("request after logout = %d, want redirect", afterLogout.Code)
	}
}

func TestAdminSecureCookieAndExpiredSession(t *testing.T) {
	server := newTestServer(t)
	getLogin := server.do(t, httptest.NewRequest(http.MethodGet, "https://example.com/admin/login", nil))
	match := csrfInput.FindStringSubmatch(getLogin.Body.String())
	loginCookie := getLogin.Result().Cookies()[0]
	form := url.Values{"username": {"admin"}, "password": {"correct horse battery staple"}, "csrf_token": {match[1]}}
	request := httptest.NewRequest(http.MethodPost, "https://example.com/admin/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://example.com")
	request.AddCookie(loginCookie)
	response := server.do(t, request)
	var session *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "portfolio_admin_session" {
			session = cookie
		}
	}
	if session == nil || !session.Secure {
		t.Fatalf("HTTPS session cookie should be Secure: %+v", session)
	}
	if _, err := server.db.Exec("UPDATE admin_sessions SET expires_at = ?", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	expired := httptest.NewRequest(http.MethodGet, "https://example.com/admin", nil)
	expired.AddCookie(session)
	if got := server.do(t, expired).Code; got != http.StatusSeeOther {
		t.Fatalf("expired session status = %d, want redirect", got)
	}
}

func TestAdminLoginErrorsAndThrottle(t *testing.T) {
	server := newTestServer(t)
	for attempt := 0; attempt < 6; attempt++ {
		page := server.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/admin/login", nil))
		match := csrfInput.FindStringSubmatch(page.Body.String())
		cookie := page.Result().Cookies()[0]
		form := url.Values{"username": {"admin"}, "password": {"wrong password"}, "csrf_token": {match[1]}}
		request := httptest.NewRequest(http.MethodPost, "http://example.com/admin/login", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Origin", "http://example.com")
		request.AddCookie(cookie)
		response := server.do(t, request)
		want := http.StatusUnauthorized
		if attempt == 5 {
			want = http.StatusTooManyRequests
		}
		if response.Code != want || strings.Contains(response.Body.String(), "wrong password") {
			t.Fatalf("attempt %d status = %d, body = %s", attempt+1, response.Code, response.Body.String())
		}
	}
}

func TestAdminDraftPublishSlugRedirectAndConcurrency(t *testing.T) {
	server := newTestServer(t)
	client := loginAdmin(t, server)
	create := url.Values{
		"title": {"Hidden Draft"}, "slug": {""}, "category": {"software"},
		"teaser": {"A private teaser"}, "lede": {"A private lede"},
		"body_md": {"Draft body"}, "published_date": {"2026-07-16"}, "action": {"draft"},
	}
	response := client.request(t, http.MethodPost, "/admin/posts", create, true)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("create draft status = %d, body = %s", response.Code, response.Body.String())
	}
	var id int64
	var status string
	var updated time.Time
	if err := server.db.QueryRow("SELECT id, status, updated_at FROM posts WHERE slug = 'hidden-draft'").Scan(&id, &status, &updated); err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("status = %q, want draft", status)
	}
	for _, path := range []string{"/", "/journal", "/journal/feed.xml", "/journal/hidden-draft"} {
		code, body := get(t, server, path)
		if strings.Contains(body, "Hidden Draft") || (path == "/journal/hidden-draft" && code != http.StatusNotFound) {
			t.Fatalf("draft leaked at %s (status %d)", path, code)
		}
	}

	update := url.Values{
		"title": {"Published Story"}, "slug": {"published-story"}, "category": {"software"},
		"teaser": {"Public teaser"}, "lede": {"Public lede"}, "body_md": {"Public body"},
		"published_date": {"2026-07-15"}, "action": {"publish"}, "updated_at": {updated.Format(time.RFC3339Nano)},
	}
	published := client.request(t, http.MethodPost, fmt.Sprintf("/admin/posts/%d", id), update, true)
	if published.Code != http.StatusSeeOther {
		t.Fatalf("publish status = %d, body = %s", published.Code, published.Body.String())
	}
	redirect := server.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/journal/hidden-draft", nil))
	if redirect.Code != http.StatusMovedPermanently || redirect.Header().Get("Location") != "/journal/published-story" {
		t.Fatalf("old slug response = %d %q", redirect.Code, redirect.Header().Get("Location"))
	}

	stale := client.request(t, http.MethodPost, fmt.Sprintf("/admin/posts/%d", id), update, true)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "another browser tab") {
		t.Fatalf("stale update = %d, body = %s", stale.Code, stale.Body.String())
	}

	var currentUpdated time.Time
	if err := server.db.QueryRow("SELECT updated_at FROM posts WHERE id = ?", id).Scan(&currentUpdated); err != nil {
		t.Fatal(err)
	}
	update.Set("updated_at", currentUpdated.Format(time.RFC3339Nano))
	update.Set("action", "unpublish")
	unpublished := client.request(t, http.MethodPost, fmt.Sprintf("/admin/posts/%d", id), update, true)
	if unpublished.Code != http.StatusSeeOther {
		t.Fatalf("unpublish = %d", unpublished.Code)
	}
	if code, _ := get(t, server, "/journal/published-story"); code != http.StatusNotFound {
		t.Fatalf("unpublished article status = %d, want 404", code)
	}
	if err := server.db.QueryRow("SELECT updated_at FROM posts WHERE id = ?", id).Scan(&currentUpdated); err != nil {
		t.Fatal(err)
	}
	deleted := client.request(t, http.MethodPost, fmt.Sprintf("/admin/posts/%d/delete", id), url.Values{
		"updated_at": {currentUpdated.Format(time.RFC3339Nano)},
	}, true)
	if deleted.Code != http.StatusSeeOther {
		t.Fatalf("delete = %d, body = %s", deleted.Code, deleted.Body.String())
	}
	var remaining int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM posts WHERE id = ?", id).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("deleted post count = %d, err = %v", remaining, err)
	}
}

func TestAdminValidationPreservesContent(t *testing.T) {
	server := newTestServer(t)
	client := loginAdmin(t, server)
	response := client.request(t, http.MethodPost, "/admin/posts", url.Values{
		"title": {"Preserve Me"}, "category": {"invalid"}, "teaser": {"teaser"},
		"lede": {"lede"}, "body_md": {"unique submitted body"}, "action": {"draft"},
	}, true)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "unique submitted body") {
		t.Fatalf("validation response = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAdminMediaUploadServeAndReferenceProtection(t *testing.T) {
	server := newTestServer(t)
	client := loginAdmin(t, server)
	gif, _ := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("csrf_token", client.csrf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="pixel.gif"`)
	header.Set("Content-Type", "image/gif")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(gif); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	request := httptest.NewRequest(http.MethodPost, "http://example.com/admin/media", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(client.session)
	uploaded := server.do(t, request)
	if uploaded.Code != http.StatusSeeOther {
		t.Fatalf("upload = %d, body = %s", uploaded.Code, uploaded.Body.String())
	}
	var id int64
	var filename string
	if err := server.db.QueryRow("SELECT id, filename FROM media").Scan(&id, &filename); err != nil {
		t.Fatal(err)
	}
	if filename == "pixel.gif" || !strings.HasSuffix(filename, ".gif") {
		t.Fatalf("filename was not randomized: %q", filename)
	}
	served := server.do(t, httptest.NewRequest(http.MethodGet, "http://example.com/media/"+filename, nil))
	if served.Code != http.StatusOK || served.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(served.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("served media response = %d, headers = %v", served.Code, served.Header())
	}
	if _, err := os.Stat(server.uploads + string(os.PathSeparator) + filename); err != nil {
		t.Fatal(err)
	}

	if _, err := server.db.Exec("UPDATE posts SET body_md = ? WHERE id = 1", "![](/media/"+filename+")"); err != nil {
		t.Fatal(err)
	}
	blocked := client.request(t, http.MethodPost, fmt.Sprintf("/admin/media/%d/delete", id), nil, true)
	if blocked.Code != http.StatusConflict {
		t.Fatalf("referenced delete = %d, want 409", blocked.Code)
	}
}

func TestAdminRejectsMismatchedUploadAndPropagatesSettings(t *testing.T) {
	server := newTestServer(t)
	client := loginAdmin(t, server)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("csrf_token", client.csrf)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="fake.png"`)
	header.Set("Content-Type", "image/png")
	part, _ := writer.CreatePart(header)
	_, _ = part.Write([]byte("not an image"))
	writer.Close()
	request := httptest.NewRequest(http.MethodPost, "http://example.com/admin/media", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Origin", "http://example.com")
	request.AddCookie(client.session)
	if got := server.do(t, request).Code; got != http.StatusUnprocessableEntity {
		t.Fatalf("mismatched upload = %d, want 422", got)
	}

	settings := url.Values{
		"display_name": {"Example Person"}, "site_handle": {"example.handle"},
		"site_title": {"Example Portfolio"}, "site_description": {"Example description"},
		"journal_description": {"Example journal"}, "contact_email": {"hello@example.com"},
		"github_url": {"https://github.com/example"}, "linkedin_url": {""}, "instagram_url": {""},
	}
	saved := client.request(t, http.MethodPost, "/admin/settings", settings, true)
	if saved.Code != http.StatusOK {
		t.Fatalf("save settings = %d, body = %s", saved.Code, saved.Body.String())
	}
	for _, path := range []string{"/", "/about", "/journal/feed.xml"} {
		_, responseBody := get(t, server, path)
		want := "Example Person"
		if path == "/" {
			want = "Example Portfolio"
		}
		if path == "/journal/feed.xml" {
			want = "Example journal"
		}
		if !strings.Contains(responseBody, want) {
			t.Fatalf("%s does not contain propagated setting %q", path, want)
		}
	}
}

func TestAdminCrossOriginRejected(t *testing.T) {
	server := newTestServer(t)
	client := loginAdmin(t, server)
	form := url.Values{"csrf_token": {client.csrf}}
	request := httptest.NewRequest(http.MethodPost, "http://example.com/admin/logout", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://attacker.example")
	request.AddCookie(client.session)
	if got := server.do(t, request).Code; got != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d, want 403", got)
	}
}
