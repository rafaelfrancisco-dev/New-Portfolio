package handlers

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-fuego/fuego"
	"go-port/internal/redact"
	"go-port/internal/views"
	"go-port/static"
)

// ValidateOrigin rejects ambiguous URLs and requires literal loopback binds.
// Development HTTP must be explicitly selected and limited to localhost.
func ValidateOrigin(raw, addr string, development bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("invalid configured origin")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return nil, fmt.Errorf("listener must bind an explicit loopback address")
	}
	local := u.Hostname() == "localhost" || (net.ParseIP(u.Hostname()) != nil && net.ParseIP(u.Hostname()).IsLoopback())
	if u.Scheme != "https" && !(development && local && u.Scheme == "http") {
		return nil, fmt.Errorf("HTTPS origin required outside explicit local development")
	}
	return u, nil
}

// Boundary wraps the mux itself so headers and access policy cover redirects,
// unmatched routes and method errors as well as registered handlers.
func Boundary(next http.Handler, origin *url.URL, publicOrigin string, admin bool) http.Handler {
	slots := make(chan struct{}, 32)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Admin responses and the health probe must never be cached; other
		// public pages keep no origin override so the CDN/browser can cache
		// them as before. Handlers may still set their own Cache-Control
		// (e.g. immutable uploaded media), which overrides this default.
		if admin || r.URL.Path == "/healthz" {
			w.Header().Set("Cache-Control", "no-store")
		}
		policy := "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"
		if admin {
			policy += " https://www.flickr.com"
		}
		w.Header().Set("Content-Security-Policy", policy)
		if origin.Scheme == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		}
		if !admin && (r.URL.Path == "/admin" || strings.HasPrefix(r.URL.Path, "/admin/")) {
			http.NotFound(w, r)
			return
		}
		if !strings.EqualFold(r.Host, origin.Host) {
			http.Error(w, "unexpected host", 421)
			return
		}
		if !admin {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			default:
				w.Header().Set("Retry-After", "1")
				slog.Warn("public request throttled")
				http.Error(w, "temporarily unavailable", 503)
				return
			}
		}
		// Forwarded proto is deliberately ignored; only configured origin controls cookies.
		ctx := context.WithValue(r.Context(), secureOriginKey{}, origin.Scheme == "https")
		ctx = views.WithPublicOrigin(ctx, publicOrigin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func SafeError(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	var withStatus fuego.ErrorWithStatus
	if errors.As(err, &withStatus) {
		status = withStatus.StatusCode()
	}
	if status < 400 || status > 599 {
		status = 500
	}
	if status >= 500 {
		id, _ := randomToken(12)
		w.Header().Set("X-Request-ID", id)
		// Error types identify the failing subsystem without logging credential URLs.
		slog.Error("request failed", "request_id", id, "error", redact.Error(err.Error()))
	}
	http.Error(w, http.StatusText(status), status)
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/static/")
	info, err := fs.Stat(static.FS, name)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.StripPrefix("/static/", http.FileServerFS(static.FS)).ServeHTTP(w, r)
}
