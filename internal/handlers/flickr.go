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
		message = "Set FLICKR_API_KEY, FLICKR_API_SECRET, FLICKR_TOKEN_ENCRYPTION_KEY, and PUBLIC_BASE_URL before connecting."
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
	callback := strings.TrimRight(rs.Flickr.Client.Config.PublicBaseURL, "/") + "/admin/flickr/callback?state=" + state
	token, authorizeURL, err := rs.Flickr.Client.RequestToken(r.Context(), callback)
	if err != nil {
		http.Error(w, "Flickr connection failed", http.StatusBadGateway)
		return
	}
	if err := rs.Flickr.Store.SaveOAuthRequest(r.Context(), state, adminSession(r).TokenHash, token, time.Now().UTC().Add(10*time.Minute)); err != nil {
		http.Error(w, "Flickr connection failed", 500)
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
	request, err := rs.Flickr.Store.ConsumeOAuthRequest(r.Context(), r.URL.Query().Get("state"), adminSession(r).TokenHash, r.URL.Query().Get("oauth_token"), time.Now().UTC())
	if err != nil {
		http.Error(w, "invalid or expired Flickr OAuth state", http.StatusForbidden)
		return
	}
	token, userID, username, err := rs.Flickr.Client.AccessToken(r.Context(), request.Token, r.URL.Query().Get("oauth_verifier"))
	if err != nil {
		http.Error(w, "Flickr connection failed", http.StatusBadGateway)
		return
	}
	if err := rs.Flickr.Store.SaveConnection(r.Context(), userID, username, token); err != nil {
		http.Error(w, "Flickr connection failed", 500)
		return
	}
	message := "connected and synchronized"
	if err := rs.Flickr.Sync(r.Context()); err != nil {
		message = "connected; initial sync failed: " + err.Error()
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
		http.Redirect(w, r, "/admin/flickr?message="+urlQueryEscape("sync failed: "+err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/flickr?message=sync%20complete", http.StatusSeeOther)
}

func (rs Resource) adminFlickrDisconnect(w http.ResponseWriter, r *http.Request) {
	if rs.Flickr != nil {
		if err := rs.Flickr.Store.Disconnect(r.Context()); err != nil && !errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "failed to disconnect Flickr", 500)
			return
		}
	}
	http.Redirect(w, r, "/admin/flickr?message=disconnected", http.StatusSeeOther)
}
