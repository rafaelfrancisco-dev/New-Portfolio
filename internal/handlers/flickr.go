package handlers

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"go-port/internal/views"
)

func (rs Resource) adminFlickr(w http.ResponseWriter, r *http.Request) {
	if rs.Flickr == nil {
		rs.render(w, r, views.AdminFlickrPage(false, "", time.Time{}, time.Time{}, "", adminSession(r).CSRFToken, "Flickr is not configured."))
		return
	}
	status, err := rs.Flickr.Store.Status(r.Context(), time.Now().UTC())
	if err != nil {
		http.Error(w, "failed to load Flickr status", 500)
		return
	}
	message := r.URL.Query().Get("message")
	if !rs.Flickr.Ready() {
		message = "Set FLICKR_API_KEY, FLICKR_API_SECRET, FLICKR_TOKEN_ENCRYPTION_KEY, and ADMIN_BASE_URL before connecting."
	}
	var attempt, success time.Time
	if status.LastAttempt.Valid {
		attempt = status.LastAttempt.Time
	}
	if status.LastSuccess.Valid {
		success = status.LastSuccess.Time
	}
	rs.render(w, r, views.AdminFlickrPage(status.Connected, status.Username, attempt, success, status.LastError, adminSession(r).CSRFToken, message))
}

func (rs Resource) adminFlickrConnect(w http.ResponseWriter, r *http.Request) {
	if rs.Flickr == nil || !rs.Flickr.Ready() {
		http.Error(w, "Flickr is not configured", http.StatusServiceUnavailable)
		return
	}
	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "Flickr connection unavailable", 500)
		return
	}
	callback := strings.TrimRight(rs.AdminBaseURL, "/") + "/admin/flickr/callback?state=" + state
	authorizeURL, err := rs.Flickr.BeginConnect(r.Context(), state, adminSession(r).TokenHash, callback)
	if err != nil {
		http.Error(w, "Flickr connection failed", 502)
		return
	}
	http.Redirect(w, r, authorizeURL, http.StatusSeeOther)
}

func (rs Resource) adminFlickrCallback(w http.ResponseWriter, r *http.Request) {
	if rs.Flickr == nil || !rs.Flickr.Ready() {
		http.Error(w, "Flickr is not configured", http.StatusServiceUnavailable)
		return
	}
	if r.URL.Query().Get("oauth_verifier") == "" {
		http.Error(w, "Flickr authorization was denied", http.StatusBadRequest)
		return
	}
	message := "connected and synchronized"
	if err := rs.Flickr.Connect(r.Context(), r.URL.Query().Get("state"), adminSession(r).TokenHash, r.URL.Query().Get("oauth_token"), r.URL.Query().Get("oauth_verifier")); err != nil {
		http.Error(w, "Flickr authorization or synchronization failed", 403)
		return
	}
	http.Redirect(w, r, "/admin/flickr?message="+urlQueryEscape(message), http.StatusSeeOther)
}

func urlQueryEscape(value string) string { return url.QueryEscape(value) }

func (rs Resource) adminFlickrSync(w http.ResponseWriter, r *http.Request) {
	if rs.Flickr == nil || !rs.Flickr.Ready() {
		http.Error(w, "Flickr is not configured", http.StatusServiceUnavailable)
		return
	}
	if err := rs.Flickr.Sync(r.Context()); err != nil {
		http.Redirect(w, r, "/admin/flickr?message="+urlQueryEscape("sync failed; try again later"), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/flickr?message=sync%20complete", http.StatusSeeOther)
}

func (rs Resource) adminFlickrDisconnect(w http.ResponseWriter, r *http.Request) {
	if rs.Flickr != nil {
		if err := rs.Flickr.Disconnect(r.Context()); err != nil && !errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "failed to disconnect Flickr", 500)
			return
		}
	}
	http.Redirect(w, r, "/admin/flickr?message=disconnected", http.StatusSeeOther)
}
