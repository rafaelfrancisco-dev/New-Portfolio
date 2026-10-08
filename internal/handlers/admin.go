package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"

	"go-port/internal/auth"
	"go-port/internal/db"
	"go-port/internal/views"
)

const (
	adminSessionCookie = "portfolio_admin_session"
	loginCSRFCookie    = "portfolio_login_csrf"
	maxUploadSize      = 10 << 20
)

type adminSessionContextKey struct{}

type requestAdminSession struct {
	TokenHash string
	CSRFToken string
}

func (rs Resource) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		slog.Error("render response", "error", err)
	}
}

func randomToken(bytes int) (string, error) {
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		return "", errors.Wrap(err, "generate random token")
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil || r.Context().Value(secureOriginKey{}) == true
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(r),
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(r),
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (rs Resource) adminAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName(r))
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		tokenHash := hashToken(cookie.Value)
		session, err := rs.Queries.GetAdminSession(r.Context(), tokenHash)
		if errors.Is(err, sql.ErrNoRows) {
			clearSessionCookie(w, r)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			http.Error(w, "admin session unavailable", http.StatusInternalServerError)
			return
		}
		if !session.ExpiresAt.After(time.Now().UTC()) || !rs.validSession(r.Context(), tokenHash) {
			_ = rs.Queries.DeleteAdminSession(r.Context(), tokenHash)
			clearSessionCookie(w, r)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !sameOrigin(r) {
				http.Error(w, "cross-origin request rejected", http.StatusForbidden)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+(1<<20))
			if err := r.ParseMultipartForm(maxUploadSize + (1 << 20)); err != nil && !errors.Is(err, http.ErrNotMultipart) {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
			submitted := r.FormValue("csrf_token")
			if submitted == "" {
				submitted = r.Header.Get("X-CSRF-Token")
			}
			if !auth.SecureEqual(submitted, session.CsrfToken) {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}
		ctx := context.WithValue(r.Context(), adminSessionContextKey{}, requestAdminSession{
			TokenHash: tokenHash,
			CSRFToken: session.CsrfToken,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sameOrigin(r *http.Request) bool {
	source := r.Header.Get("Origin")
	if source == "" {
		source = r.Header.Get("Referer")
	}
	if source == "" {
		return false
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	scheme := "http"
	if requestIsSecure(r) {
		scheme = "https"
	}
	return strings.EqualFold(parsed.Scheme, scheme) && strings.EqualFold(parsed.Host, r.Host)
}

func adminSession(r *http.Request) requestAdminSession {
	session, _ := r.Context().Value(adminSessionContextKey{}).(requestAdminSession)
	return session
}

func (rs Resource) adminLoginPage(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName(r)); err == nil {
		if session, err := rs.Queries.GetAdminSession(r.Context(), hashToken(cookie.Value)); err == nil && session.ExpiresAt.After(time.Now().UTC()) && rs.validSession(r.Context(), hashToken(cookie.Value)) {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
	}
	token, err := randomToken(32)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     loginCSRFCookie,
		Value:    token,
		Path:     "/admin/login",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
	rs.render(w, r, views.AdminLoginPage(token, ""))
}

func (rs Resource) adminLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		http.Error(w, "unsupported content type", 415)
		return
	}
	if err := r.ParseForm(); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			http.Error(w, "request too large", 413)
		} else {
			http.Error(w, "invalid form", 400)
		}
		return
	}

	if !sameOrigin(r) {
		http.Error(w, "cross-origin request rejected", http.StatusForbidden)
		return
	}
	cookie, err := r.Cookie(loginCSRFCookie)
	if err != nil || !auth.SecureEqual(cookie.Value, r.FormValue("csrf_token")) {
		http.Error(w, "invalid CSRF token", http.StatusForbidden)
		return
	}
	if len(r.PostForm.Get("username")) > 128 || len(r.PostForm.Get("password")) > 1024 {
		http.Error(w, "invalid credentials", http.StatusBadRequest)
		return
	}
	reserved, err := db.ReserveLogin(r.Context(), rs.AppDB, clientIP(r))
	if err != nil {
		http.Error(w, "login unavailable", 503)
		return
	}
	if !reserved {
		loginOverloaded(w)
		return
	}
	release, available := acquireHashSlot()
	if !available {
		loginOverloaded(w)
		return
	}
	defer release()
	var generation int64
	if err := rs.AppDB.QueryRowContext(r.Context(), "SELECT generation FROM admin_security WHERE id=1").Scan(&generation); err != nil {
		http.Error(w, "login unavailable", 503)
		return
	}
	usernameOK := auth.SecureEqual(r.FormValue("username"), rs.AdminUsername)
	passwordOK := auth.VerifyPassword(r.FormValue("password"), rs.AdminPasswordHash)
	if !usernameOK || !passwordOK {
		w.WriteHeader(http.StatusUnauthorized)
		rs.render(w, r, views.AdminLoginPage(cookie.Value, "Unable to sign in. Check your credentials."))
		return
	}
	rawToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	csrfToken, err := randomToken(32)
	if err != nil {
		http.Error(w, "login unavailable", http.StatusInternalServerError)
		return
	}
	expires := time.Now().UTC().Add(12 * time.Hour)
	if ok, err := db.CreateBoundSession(r.Context(), rs.AppDB, hashToken(rawToken), csrfToken, rs.fingerprint(), generation, expires); err != nil || !ok {
		http.Error(w, "login unavailable", 503)
		return
	}
	setSessionCookie(w, r, rawToken, expires)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return "unknown"
	}
	if !peer.IsLoopback() {
		return peer.String()
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		ip := net.ParseIP(strings.TrimSpace(chain[i]))
		if ip == nil {
			return peer.String()
		}
		if !ip.IsLoopback() {
			return ip.String()
		}
	}
	return peer.String()
}

func (rs Resource) adminLogout(w http.ResponseWriter, r *http.Request) {
	if err := rs.Queries.DeleteAdminSession(r.Context(), adminSession(r).TokenHash); err != nil {
		http.Error(w, "logout unavailable", 503)
		return
	}
	clearSessionCookie(w, r)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

func (rs Resource) adminDashboard(w http.ResponseWriter, r *http.Request) {
	counts, err := rs.Queries.CountAdminPosts(r.Context())
	if err != nil {
		http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		return
	}
	mediaCount, err := rs.Queries.CountMedia(r.Context())
	if err != nil {
		http.Error(w, "failed to load dashboard", http.StatusInternalServerError)
		return
	}
	rs.render(w, r, views.AdminDashboardPage(counts, int(mediaCount), adminSession(r).CSRFToken))
}

func (rs Resource) adminPosts(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "draft" && status != "published" {
		status = ""
	}
	category := r.URL.Query().Get("category")
	if category != "software" && category != "photography" {
		category = ""
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	posts, err := rs.Queries.ListAdminPosts(r.Context(), db.ListAdminPostsParams{
		StatusFilter: status, CategoryFilter: category, SearchText: search,
	})
	if err != nil {
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	rs.render(w, r, views.AdminPostsPage(posts, status, category, search, adminSession(r).CSRFToken))
}

func (rs Resource) adminNewPost(w http.ResponseWriter, r *http.Request) {
	form := views.AdminPostForm{
		Category: "software", Status: "draft",
		PublishedDate: time.Now().Format("2006-01-02"),
	}
	rs.renderPostEditor(w, r, form)
}

func (rs Resource) adminEditPost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	post, err := rs.Queries.GetAdminPost(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to load post", http.StatusInternalServerError)
		return
	}
	rs.renderPostEditor(w, r, views.NewAdminPostForm(post))
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func (rs Resource) renderPostEditor(w http.ResponseWriter, r *http.Request, form views.AdminPostForm) {
	preview, err := rs.Markdown.Render(form.BodyMD)
	if err != nil {
		http.Error(w, "failed to render preview", http.StatusInternalServerError)
		return
	}
	media, err := rs.Queries.ListMedia(r.Context())
	if err != nil {
		http.Error(w, "failed to list media", http.StatusInternalServerError)
		return
	}
	rs.render(w, r, views.AdminPostEditorPage(form, preview, media, adminSession(r).CSRFToken))
}

var repeatedHyphen = regexp.MustCompile(`-+`)

func slugify(title string) string {
	var output strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) && r <= unicode.MaxASCII, unicode.IsDigit(r):
			output.WriteRune(r)
			hyphen = false
		case output.Len() > 0 && !hyphen:
			output.WriteByte('-')
			hyphen = true
		}
	}
	slug := strings.Trim(repeatedHyphen.ReplaceAllString(output.String(), "-"), "-")
	if slug == "" {
		return "post"
	}
	return slug
}

func parsePostForm(r *http.Request) (views.AdminPostForm, string, error) {
	form := views.AdminPostForm{
		Title:         strings.TrimSpace(r.FormValue("title")),
		Slug:          strings.TrimSpace(strings.ToLower(r.FormValue("slug"))),
		Category:      r.FormValue("category"),
		Teaser:        strings.TrimSpace(r.FormValue("teaser")),
		Lede:          strings.TrimSpace(r.FormValue("lede")),
		BodyMD:        strings.TrimSpace(r.FormValue("body_md")),
		StoreURL:      strings.TrimSpace(r.FormValue("store_url")),
		PublishedDate: r.FormValue("published_date"),
		UpdatedAt:     r.FormValue("updated_at"),
	}
	if len(r.FormValue("body_md")) > 1<<20 {
		return form, "", errors.New("Markdown exceeds 1 MiB")
	}
	if form.Slug == "" {
		form.Slug = slugify(form.Title)
	}
	if form.Title == "" || form.Teaser == "" || form.Lede == "" || form.BodyMD == "" {
		return form, "", errors.New("title, teaser, lede, and body are required")
	}
	if form.Category != "software" && form.Category != "photography" {
		return form, "", errors.New("choose a valid category")
	}
	if form.Slug != slugify(form.Slug) {
		return form, "", errors.New("slug must contain only lowercase letters, numbers, and hyphens")
	}
	if form.StoreURL != "" {
		parsed, err := url.ParseRequestURI(form.StoreURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return form, "", errors.New("store URL must be an http or https URL")
		}
	}
	action := r.FormValue("action")
	return form, action, nil
}

func publicationFor(action string, date string, existing db.Post) (string, sql.NullTime, error) {
	switch action {
	case "publish":
		if date == "" {
			return "", sql.NullTime{}, errors.New("choose a publication date")
		}
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			return "", sql.NullTime{}, errors.New("choose a valid publication date")
		}
		return "published", sql.NullTime{Time: parsed.UTC(), Valid: true}, nil
	case "unpublish":
		return "draft", sql.NullTime{}, nil
	case "draft":
		if existing.ID != 0 && existing.Status == "published" {
			return "published", existing.PublishedAt, nil
		}
		return "draft", sql.NullTime{}, nil
	default:
		return "", sql.NullTime{}, errors.New("unknown save action")
	}
}

func (rs Resource) adminCreatePost(w http.ResponseWriter, r *http.Request) {
	form, action, err := parsePostForm(r)
	if err != nil {
		form.Error = err.Error()
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderPostEditor(w, r, form)
		return
	}
	status, publishedAt, err := publicationFor(action, form.PublishedDate, db.Post{})
	if err != nil {
		form.Error = err.Error()
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderPostEditor(w, r, form)
		return
	}
	inUse, err := rs.Queries.SlugInUse(r.Context(), db.SlugInUseParams{
		Slug: form.Slug, ID: 0, OldSlug: form.Slug, PostID: 0,
	})
	if err != nil || inUse {
		form.Error = "slug is already in use"
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderPostEditor(w, r, form)
		return
	}
	post, err := rs.Queries.CreatePost(r.Context(), db.CreatePostParams{
		Slug: form.Slug, Title: form.Title, Category: form.Category, Teaser: form.Teaser,
		Lede: form.Lede, BodyMd: form.BodyMD, StoreUrl: form.StoreURL,
		Status: status, PublishedAt: publishedAt,
	})
	if err != nil {
		http.Error(w, "failed to create post", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/posts/%d/edit", post.ID), http.StatusSeeOther)
}

func (rs Resource) adminUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	existing, err := rs.Queries.GetAdminPost(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to load post", http.StatusInternalServerError)
		return
	}
	form, action, err := parsePostForm(r)
	form.ID = id
	form.Status = existing.Status
	if err != nil {
		form.Error = err.Error()
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderPostEditor(w, r, form)
		return
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, form.UpdatedAt)
	if err != nil {
		form.Error = "This editor is stale. Reload the post and try again."
		w.WriteHeader(http.StatusConflict)
		rs.renderPostEditor(w, r, form)
		return
	}
	status, publishedAt, err := publicationFor(action, form.PublishedDate, existing)
	if err != nil {
		form.Error = err.Error()
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderPostEditor(w, r, form)
		return
	}
	inUse, err := rs.Queries.SlugInUse(r.Context(), db.SlugInUseParams{
		Slug: form.Slug, ID: id, OldSlug: form.Slug, PostID: id,
	})
	if err != nil || inUse {
		form.Error = "slug is already in use"
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderPostEditor(w, r, form)
		return
	}
	tx, err := rs.AppDB.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, "failed to update post", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	qtx := rs.Queries.WithTx(tx)
	post, err := qtx.UpdatePost(r.Context(), db.UpdatePostParams{
		Slug: form.Slug, Title: form.Title, Category: form.Category, Teaser: form.Teaser,
		Lede: form.Lede, BodyMd: form.BodyMD, StoreUrl: form.StoreURL, Status: status,
		PublishedAt: publishedAt, ID: id, UpdatedAt: updatedAt.Format("2006-01-02 15:04:05.999999999"),
	})
	if errors.Is(err, sql.ErrNoRows) {
		form.Error = "This post changed in another browser tab. Reload before saving."
		w.WriteHeader(http.StatusConflict)
		rs.renderPostEditor(w, r, form)
		return
	}
	if err != nil {
		http.Error(w, "failed to update post", http.StatusInternalServerError)
		return
	}
	if existing.Slug != post.Slug {
		if err := qtx.CreateSlugRedirect(r.Context(), db.CreateSlugRedirectParams{OldSlug: existing.Slug, PostID: post.ID}); err != nil {
			http.Error(w, "failed to record slug redirect", http.StatusInternalServerError)
			return
		}
		if err := qtx.DeleteSlugRedirectForCurrentSlug(r.Context(), post.Slug); err != nil {
			http.Error(w, "failed to update slug redirects", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "failed to update post", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/posts/%d/edit", post.ID), http.StatusSeeOther)
}

func (rs Resource) adminDeletePost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, r.FormValue("updated_at"))
	if err != nil {
		http.Error(w, "stale editor", http.StatusConflict)
		return
	}
	deleted, err := rs.Queries.DeletePost(r.Context(), db.DeletePostParams{
		ID: id, UpdatedAt: updatedAt.Format("2006-01-02 15:04:05.999999999"),
	})
	if err != nil {
		http.Error(w, "failed to delete post", http.StatusInternalServerError)
		return
	}
	if deleted == 0 {
		http.Error(w, "post changed; reload before deleting", http.StatusConflict)
		return
	}
	http.Redirect(w, r, "/admin/posts", http.StatusSeeOther)
}

func (rs Resource) adminPreview(w http.ResponseWriter, r *http.Request) {
	if len(r.FormValue("body_md")) > 1<<20 {
		http.Error(w, "Markdown exceeds 1 MiB", 413)
		return
	}
	html, err := rs.Markdown.Render(r.FormValue("body_md"))
	if err != nil {
		http.Error(w, "failed to render preview", http.StatusInternalServerError)
		return
	}
	rs.render(w, r, views.AdminPreview(html))
}

func safeUploadName(header *multipart.FileHeader) bool {
	name := header.Filename
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

var uploadTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

func (rs Resource) adminUploadMedia(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		rs.renderMedia(w, r, "Choose a file to upload.")
		return
	}
	defer file.Close()
	if !safeUploadName(header) || header.Size <= 0 || header.Size > maxUploadSize {
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderMedia(w, r, "File names must be safe and files must be no larger than 10 MiB.")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize+1))
	if err != nil || len(data) > maxUploadSize {
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderMedia(w, r, "File is too large.")
		return
	}
	detected := http.DetectContentType(data)
	extension, allowed := uploadTypes[detected]
	declared := strings.ToLower(strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]))
	if !allowed || declared != detected {
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.renderMedia(w, r, "File type is unsupported or does not match its contents.")
		return
	}
	cfg, _, decodeErr := image.DecodeConfig(bytes.NewReader(data))
	if decodeErr != nil || cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		http.Error(w, "invalid image dimensions", 422)
		return
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		http.Error(w, "invalid image", 422)
		return
	}
	random, err := randomToken(18)
	if err != nil {
		http.Error(w, "failed to store upload", http.StatusInternalServerError)
		return
	}
	filename := random + extension
	if err := os.MkdirAll(rs.UploadDir, 0o750); err != nil {
		http.Error(w, "failed to store upload", http.StatusInternalServerError)
		return
	}
	path := filepath.Join(rs.UploadDir, filename)
	if err := os.WriteFile(path, data, 0o640); err != nil {
		http.Error(w, "failed to store upload", http.StatusInternalServerError)
		return
	}
	if _, err := rs.Queries.CreateMedia(r.Context(), db.CreateMediaParams{
		Filename: filename, OriginalFilename: header.Filename, MimeType: detected, SizeBytes: int64(len(data)),
	}); err != nil {
		_ = os.Remove(path)
		http.Error(w, "failed to record upload", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func (rs Resource) adminMedia(w http.ResponseWriter, r *http.Request) {
	rs.renderMedia(w, r, "")
}

func (rs Resource) renderMedia(w http.ResponseWriter, r *http.Request, message string) {
	items, err := rs.Queries.ListMedia(r.Context())
	if err != nil {
		http.Error(w, "failed to list media", http.StatusInternalServerError)
		return
	}
	rs.render(w, r, views.AdminMediaPage(items, adminSession(r).CSRFToken, message))
}

func (rs Resource) adminDeleteMedia(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	item, err := rs.Queries.GetMedia(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to load media", http.StatusInternalServerError)
		return
	}
	referenced, err := rs.Queries.MediaIsReferenced(r.Context(), sql.NullString{String: item.Filename, Valid: true})
	if err != nil {
		http.Error(w, "failed to check media usage", http.StatusInternalServerError)
		return
	}
	if referenced {
		w.WriteHeader(http.StatusConflict)
		rs.renderMedia(w, r, "This file is referenced by a post and cannot be deleted.")
		return
	}
	if err := rs.Queries.DeleteMedia(r.Context(), id); err != nil {
		http.Error(w, "failed to delete media", http.StatusInternalServerError)
		return
	}
	if err := os.Remove(filepath.Join(rs.UploadDir, item.Filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, "media record deleted but file cleanup failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func (rs Resource) serveMedia(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" || filename != filepath.Base(filename) {
		http.NotFound(w, r)
		return
	}
	item, err := rs.Queries.GetMediaByFilename(r.Context(), filename)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to load media", http.StatusInternalServerError)
		return
	}
	file, err := os.Open(filepath.Join(rs.UploadDir, item.Filename))
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "failed to load media", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "failed to load media", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", item.MimeType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, item.Filename, info.ModTime(), file)
}

func (rs Resource) adminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := rs.Queries.GetSiteSettings(r.Context())
	if err != nil {
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	rs.render(w, r, views.AdminSettingsPage(settings, adminSession(r).CSRFToken, ""))
}

func validateOptionalHTTPURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (rs Resource) adminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	params := db.UpdateSiteSettingsParams{
		DisplayName:        strings.TrimSpace(r.FormValue("display_name")),
		SiteHandle:         strings.TrimSpace(r.FormValue("site_handle")),
		SiteTitle:          strings.TrimSpace(r.FormValue("site_title")),
		SiteDescription:    strings.TrimSpace(r.FormValue("site_description")),
		JournalDescription: strings.TrimSpace(r.FormValue("journal_description")),
		ContactEmail:       strings.TrimSpace(r.FormValue("contact_email")),
		GithubUrl:          strings.TrimSpace(r.FormValue("github_url")),
		LinkedinUrl:        strings.TrimSpace(r.FormValue("linkedin_url")),
		InstagramUrl:       strings.TrimSpace(r.FormValue("instagram_url")),
	}
	_, emailErr := mail.ParseAddress(params.ContactEmail)
	if params.DisplayName == "" || params.SiteHandle == "" || params.SiteTitle == "" ||
		params.SiteDescription == "" || params.JournalDescription == "" || emailErr != nil ||
		!validateOptionalHTTPURL(params.GithubUrl) || !validateOptionalHTTPURL(params.LinkedinUrl) ||
		!validateOptionalHTTPURL(params.InstagramUrl) {
		settings := db.SiteSetting{
			DisplayName: params.DisplayName, SiteHandle: params.SiteHandle, SiteTitle: params.SiteTitle,
			SiteDescription: params.SiteDescription, JournalDescription: params.JournalDescription,
			ContactEmail: params.ContactEmail, GithubUrl: params.GithubUrl,
			LinkedinUrl: params.LinkedinUrl, InstagramUrl: params.InstagramUrl,
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		rs.render(w, r, views.AdminSettingsPage(settings, adminSession(r).CSRFToken, "Complete required fields with a valid email and http/https social URLs."))
		return
	}
	if _, err := rs.Queries.UpdateSiteSettings(r.Context(), params); err != nil {
		http.Error(w, "failed to save settings", http.StatusInternalServerError)
		return
	}
	settings, _ := rs.Queries.GetSiteSettings(r.Context())
	rs.render(w, r, views.AdminSettingsPage(settings, adminSession(r).CSRFToken, "Settings saved."))
}

var hashSlots = make(chan struct{}, 2)

type secureOriginKey struct{}

func sessionCookieName(r *http.Request) string {
	if requestIsSecure(r) {
		return "__Host-portfolio_admin_session"
	}
	return adminSessionCookie
}
func loginOverloaded(w http.ResponseWriter) {
	slog.Warn("login throttled")
	w.Header().Set("Retry-After", "900")
	http.Error(w, "Unable to sign in. Try again later.", 429)
}
func (rs Resource) fingerprint() string {
	return hashToken(rs.AdminUsername + "\x00" + rs.AdminPasswordHash)
}
func (rs Resource) validSession(ctx context.Context, hash string) bool {
	var valid bool
	err := rs.AppDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM admin_sessions s JOIN admin_security a ON a.id=1 AND a.generation=s.generation WHERE s.token_hash=? AND s.credential_fingerprint=?)`, hash, rs.fingerprint()).Scan(&valid)
	return err == nil && valid
}
func (rs Resource) adminRevokeAll(w http.ResponseWriter, r *http.Request) {
	if err := db.RevokeSessions(r.Context(), rs.AppDB); err != nil {
		http.Error(w, "revocation unavailable", 503)
		return
	}
	clearSessionCookie(w, r)
	http.Redirect(w, r, "/admin/login", 303)
}

func acquireHashSlot() (func(), bool) {
	select {
	case hashSlots <- struct{}{}:
		return func() { <-hashSlots }, true
	default:
		return nil, false
	}
}
