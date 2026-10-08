package flickr

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const CacheLifetime = 24 * time.Hour

// PhotosPerPage caps how many photos the gallery page renders at once.
const PhotosPerPage = 60

type Store struct {
	DB     *sql.DB
	Cipher *Cipher
}
type Connection struct {
	UserID, Username string
	Token            Token
	ConnectedAt      time.Time
}
type OAuthRequest struct {
	Token       Token
	SessionHash string
}
type Status struct {
	Connected                bool
	Username                 string
	LastAttempt, LastSuccess sql.NullTime
	LastError                string
	Fresh                    bool
}
type Gallery struct {
	Albums      []Album
	Photos      []Photo
	Filter      string
	Available   bool
	SyncedAt    time.Time
	Page        int
	PageSize    int
	TotalPages  int
	TotalPhotos int
}

func (s Store) RecordFailure(ctx context.Context, now time.Time, cause error) {
	id, err := snapshotID()
	if err != nil {
		return
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM flickr_snapshots WHERE status='failed'`); err != nil {
		return
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO flickr_snapshots(id,started_at,completed_at,status,error) VALUES(?,?,?,'failed',?)`, id, now, now, "Flickr synchronization failed"); err != nil {
		return
	}
	if _, err = tx.ExecContext(ctx, `UPDATE flickr_sync_status SET last_attempt_at=?,last_error=? WHERE id=1`, now, "Flickr synchronization failed"); err != nil {
		return
	}
	_ = tx.Commit()
}

func HashState(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s Store) SaveOAuthRequest(ctx context.Context, state, sessionHash string, token Token, expires time.Time) error {
	secret, err := s.Cipher.Encrypt(token.Secret)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO flickr_oauth_requests(state_hash, admin_session_hash, request_token, secret_ciphertext, expires_at) VALUES(?,?,?,?,?)`, HashState(state), sessionHash, token.Value, secret, expires)
	return err
}

func (s Store) ConsumeOAuthRequest(ctx context.Context, state, sessionHash, requestToken string, now time.Time) (OAuthRequest, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return OAuthRequest{}, err
	}
	defer tx.Rollback()
	var result OAuthRequest
	var encrypted []byte
	var expires time.Time
	err = tx.QueryRowContext(ctx, `SELECT admin_session_hash, request_token, secret_ciphertext, expires_at FROM flickr_oauth_requests WHERE state_hash=?`, HashState(state)).Scan(&result.SessionHash, &result.Token.Value, &encrypted, &expires)
	if err != nil {
		return OAuthRequest{}, err
	}
	if result.SessionHash != sessionHash || result.Token.Value != requestToken || !expires.After(now) {
		return OAuthRequest{}, fmt.Errorf("invalid or expired Flickr OAuth state")
	}
	result.Token.Secret, err = s.Cipher.Decrypt(encrypted)
	if err != nil {
		return OAuthRequest{}, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM flickr_oauth_requests WHERE state_hash=?`, HashState(state)); err != nil {
		return OAuthRequest{}, err
	}
	return result, tx.Commit()
}

func (s Store) SaveConnection(ctx context.Context, userID, username string, token Token) error {
	tokenCipher, err := s.Cipher.Encrypt(token.Value)
	if err != nil {
		return err
	}
	secretCipher, err := s.Cipher.Encrypt(token.Secret)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO flickr_connections(id,user_id,username,token_ciphertext,secret_ciphertext) VALUES(1,?,?,?,?) ON CONFLICT(id) DO UPDATE SET user_id=excluded.user_id,username=excluded.username,token_ciphertext=excluded.token_ciphertext,secret_ciphertext=excluded.secret_ciphertext,connected_at=CURRENT_TIMESTAMP`, userID, username, tokenCipher, secretCipher)
	return err
}

func (s Store) Connection(ctx context.Context) (Connection, error) {
	var result Connection
	var tokenCipher, secretCipher []byte
	err := s.DB.QueryRowContext(ctx, `SELECT user_id,username,token_ciphertext,secret_ciphertext,connected_at FROM flickr_connections WHERE id=1`).Scan(&result.UserID, &result.Username, &tokenCipher, &secretCipher, &result.ConnectedAt)
	if err != nil {
		return result, err
	}
	result.Token.Value, err = s.Cipher.Decrypt(tokenCipher)
	if err != nil {
		return result, err
	}
	result.Token.Secret, err = s.Cipher.Decrypt(secretCipher)
	return result, err
}

func (s Store) Disconnect(ctx context.Context) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{`DELETE FROM flickr_connections`, `DELETE FROM flickr_oauth_requests`, `UPDATE flickr_sync_status SET active_snapshot_id=NULL,last_attempt_at=NULL,last_success_at=NULL,last_error='' WHERE id=1`, `DELETE FROM flickr_snapshots`} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s Store) Status(ctx context.Context, now time.Time) (Status, error) {
	var status Status
	var connected int
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flickr_connections), COALESCE((SELECT username FROM flickr_connections WHERE id=1),''), last_attempt_at,last_success_at,last_error FROM flickr_sync_status WHERE id=1`).Scan(&connected, &status.Username, &status.LastAttempt, &status.LastSuccess, &status.LastError)
	status.Connected = connected == 1
	status.Fresh = status.LastSuccess.Valid && now.Sub(status.LastSuccess.Time) < CacheLifetime
	return status, err
}

func snapshotID() (string, error) {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s Store) ReplaceSnapshot(ctx context.Context, albums []AlbumPhotos, now time.Time) error {
	id, err := snapshotID()
	if err != nil {
		return err
	}
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM flickr_snapshots WHERE status='failed'`)
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO flickr_snapshots(id,started_at,status) VALUES(?,?,'building')`, id, now); err != nil {
		return err
	}
	fail := func(cause error) error {
		_, _ = s.DB.ExecContext(ctx, `UPDATE flickr_snapshots SET status='failed',completed_at=?,error=? WHERE id=?`, time.Now().UTC(), "Flickr synchronization failed", id)
		_, _ = s.DB.ExecContext(ctx, `UPDATE flickr_sync_status SET last_attempt_at=?,last_error=? WHERE id=1`, now, "Flickr synchronization failed")
		return cause
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback()
	failTx := func(cause error) error { _ = tx.Rollback(); return fail(cause) }
	seen := map[string]bool{}
	for _, group := range albums {
		if len(group.Photos) == 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO flickr_albums(snapshot_id,album_id,title,description,position) VALUES(?,?,?,?,?)`, id, group.Album.ID, group.Album.Title, group.Album.Description, group.Album.Position); err != nil {
			return failTx(err)
		}
		for position, photo := range group.Photos {
			if !seen[photo.ID] {
				_, err := tx.ExecContext(ctx, `INSERT INTO flickr_photos(snapshot_id,photo_id,title,description,page_url,image_small_url,image_large_url,width,height,taken_at,location,camera,lens,exposure,aperture,iso,focal_length) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, photo.ID, photo.Title, photo.Description, photo.PageURL, photo.SmallURL, photo.LargeURL, photo.Width, photo.Height, photo.TakenAt, photo.Location, photo.Camera, photo.Lens, photo.Exposure, photo.Aperture, photo.ISO, photo.FocalLength)
				if err != nil {
					return failTx(err)
				}
				seen[photo.ID] = true
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO flickr_album_photos(snapshot_id,album_id,photo_id,position) VALUES(?,?,?,?)`, id, group.Album.ID, photo.ID, position); err != nil {
				return failTx(err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flickr_snapshots SET status='active',completed_at=?,featured_photo_id=(SELECT photo_id FROM flickr_photos WHERE snapshot_id=? ORDER BY random() LIMIT 1) WHERE id=?`, now, id, id); err != nil {
		return failTx(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flickr_snapshots SET status='building' WHERE status='active' AND id<>?`, id); err != nil {
		return failTx(err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flickr_sync_status SET active_snapshot_id=?,last_attempt_at=?,last_success_at=?,last_error='' WHERE id=1`, id, now, now); err != nil {
		return failTx(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM flickr_snapshots WHERE status='building' AND id<>?`, id); err != nil {
		return failTx(err)
	}
	if err := tx.Commit(); err != nil {
		return fail(err)
	}
	return nil
}

func scanPhoto(rows interface{ Scan(...any) error }) (Photo, error) {
	var p Photo
	err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.PageURL, &p.SmallURL, &p.LargeURL, &p.Width, &p.Height, &p.TakenAt, &p.Location, &p.Camera, &p.Lens, &p.Exposure, &p.Aperture, &p.ISO, &p.FocalLength)
	if err == nil {
		if !safeFlickrURL(p.PageURL, false) {
			p.PageURL = ""
		}
		if !safeFlickrURL(p.SmallURL, true) {
			p.SmallURL = ""
		}
		if !safeFlickrURL(p.LargeURL, true) {
			p.LargeURL = ""
		}
	}
	return p, err
}

const photoColumns = `p.photo_id,p.title,p.description,p.page_url,p.image_small_url,p.image_large_url,p.width,p.height,p.taken_at,p.location,p.camera,p.lens,p.exposure,p.aperture,p.iso,p.focal_length`

func (s Store) snapshot(ctx context.Context, now time.Time) (string, time.Time, error) {
	var id string
	var completed time.Time
	err := s.DB.QueryRowContext(ctx, `SELECT s.id,s.completed_at FROM flickr_sync_status st JOIN flickr_snapshots s ON s.id=st.active_snapshot_id WHERE st.id=1`).Scan(&id, &completed)
	if err == nil && now.Sub(completed) >= CacheLifetime {
		err = sql.ErrNoRows
	}
	return id, completed, err
}
func (s Store) filter(ctx context.Context, snapshot, filter string) (string, error) {
	if filter == "" || filter == "all" {
		return "all", nil
	}
	var exists bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM flickr_albums WHERE snapshot_id=? AND album_id=?)`, snapshot, filter).Scan(&exists)
	if !exists {
		filter = "all"
	}
	return filter, err
}
func photoWhere(snapshot, filter string) (string, []any) {
	query := ` FROM flickr_photos p WHERE p.snapshot_id=?`
	args := []any{snapshot}
	if filter != "all" {
		query += ` AND EXISTS(SELECT 1 FROM flickr_album_photos ap WHERE ap.snapshot_id=p.snapshot_id AND ap.photo_id=p.photo_id AND ap.album_id=?)`
		args = append(args, filter)
	}
	return query, args
}

func (s Store) Gallery(ctx context.Context, filter string, now time.Time, page int) (Gallery, error) {
	result := Gallery{Page: 1, PageSize: PhotosPerPage, TotalPages: 1}
	snapshot, completed, err := s.snapshot(ctx, now)
	if err == sql.ErrNoRows {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	filter, err = s.filter(ctx, snapshot, filter)
	if err != nil {
		return result, err
	}
	result.Filter = filter
	result.Available = true
	result.SyncedAt = completed
	rows, err := s.DB.QueryContext(ctx, `SELECT album_id,title,description,position FROM flickr_albums WHERE snapshot_id=? ORDER BY position`, snapshot)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var a Album
		if err = rows.Scan(&a.ID, &a.Title, &a.Description, &a.Position); err != nil {
			rows.Close()
			return result, err
		}
		result.Albums = append(result.Albums, a)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	where, args := photoWhere(snapshot, filter)
	if err = s.DB.QueryRowContext(ctx, `SELECT count(*)`+where, args...).Scan(&result.TotalPhotos); err != nil {
		return result, err
	}
	result.TotalPages = max(1, (result.TotalPhotos+PhotosPerPage-1)/PhotosPerPage)
	result.Page = min(max(1, page), result.TotalPages)
	args = append(args, PhotosPerPage, (result.Page-1)*PhotosPerPage)
	rows, err = s.DB.QueryContext(ctx, `SELECT `+photoColumns+where+` ORDER BY p.taken_at DESC,p.photo_id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return result, err
		}
		result.Photos = append(result.Photos, p)
	}
	return result, rows.Err()
}

func (s Store) Photo(ctx context.Context, id, filter string, now time.Time) (Photo, string, string, string, error) {
	snapshot, _, err := s.snapshot(ctx, now)
	if err != nil {
		return Photo{}, "", "", "", err
	}
	filter, err = s.filter(ctx, snapshot, filter)
	if err != nil {
		return Photo{}, "", "", "", err
	}
	where, args := photoWhere(snapshot, filter)
	p, err := scanPhoto(s.DB.QueryRowContext(ctx, `SELECT `+photoColumns+where+` AND p.photo_id=?`, append(args, id)...))
	if err != nil {
		return Photo{}, "", "", "", err
	}
	adjacent := func(newer bool) (string, error) {
		cmp, order := "<", "DESC"
		if newer {
			cmp, order = ">", "ASC"
		}
		var id string
		err := s.DB.QueryRowContext(ctx, `SELECT p.photo_id`+where+` AND (p.taken_at,p.photo_id) `+cmp+` (?,?) ORDER BY p.taken_at `+order+`,p.photo_id `+order+` LIMIT 1`, append(append([]any{}, args...), p.TakenAt, p.ID)...).Scan(&id)
		if err == sql.ErrNoRows {
			if newer {
				order = "ASC"
			} else {
				order = "DESC"
			}
			err = s.DB.QueryRowContext(ctx, `SELECT p.photo_id`+where+` ORDER BY p.taken_at `+order+`,p.photo_id `+order+` LIMIT 1`, args...).Scan(&id)
		}
		return id, err
	}
	prev, err := adjacent(true)
	if err != nil {
		return Photo{}, "", "", "", err
	}
	next, err := adjacent(false)
	return p, prev, next, filter, err
}

// The featured selection is cached in the snapshot at publication time.
func (s Store) RandomPhoto(ctx context.Context, now time.Time) (Photo, bool, error) {
	snapshot, _, err := s.snapshot(ctx, now)
	if err == sql.ErrNoRows {
		return Photo{}, false, nil
	}
	if err != nil {
		return Photo{}, false, err
	}
	p, err := scanPhoto(s.DB.QueryRowContext(ctx, `SELECT `+photoColumns+` FROM flickr_photos p WHERE p.snapshot_id=? AND p.photo_id=(SELECT featured_photo_id FROM flickr_snapshots WHERE id=?)`, snapshot, snapshot))
	if err == sql.ErrNoRows {
		return Photo{}, false, nil
	}
	return p, err == nil, err
}

func safeFlickrURL(raw string, image bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if image {
		return host == "staticflickr.com" || strings.HasSuffix(host, ".staticflickr.com")
	}
	return host == "www.flickr.com" || host == "flickr.com"
}
