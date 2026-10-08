package flickr_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-port/internal/db"
	"go-port/internal/flickr"
)

func testStore(t *testing.T) flickr.Store {
	t.Helper()
	sqlDB, err := db.Connect(filepath.Join(t.TempDir(), "flickr.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	cipher, err := flickr.NewCipher("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	return flickr.Store{DB: sqlDB, Cipher: cipher}
}

func photo(id, title, takenAt string) flickr.Photo {
	return flickr.Photo{ID: id, Title: title, TakenAt: takenAt, PageURL: "https://flickr.test/" + id, SmallURL: "https://img.test/" + id + "_m.jpg", LargeURL: "https://img.test/" + id + "_b.jpg", Width: 1200, Height: 800}
}

func TestSnapshotDeduplicationOrderingFilteringAndExpiry(t *testing.T) {
	store := testStore(t)
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	shared := photo("opaque/42", "shared", "2026-07-15 10:00:00")
	groups := []flickr.AlbumPhotos{
		{Album: flickr.Album{ID: "album-z", Title: "First", Position: 0}, Photos: []flickr.Photo{shared, photo("p2", "second", "2026-07-10 10:00:00")}},
		{Album: flickr.Album{ID: "private-only", Title: "Hidden", Position: 1}},
		{Album: flickr.Album{ID: "album-a", Title: "Second", Position: 2}, Photos: []flickr.Photo{photo("p3", "third", "2026-07-18 10:00:00"), shared}},
	}
	if err := store.ReplaceSnapshot(t.Context(), groups, now); err != nil {
		t.Fatal(err)
	}
	all, err := store.Gallery(t.Context(), "all", now.Add(time.Hour), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !all.Available || len(all.Albums) != 2 || len(all.Photos) != 3 {
		t.Fatalf("gallery = %+v", all)
	}
	// Newest taken_at first, regardless of album/position.
	if all.Albums[0].ID != "album-z" || all.Photos[0].ID != "p3" || all.Photos[1].ID != "opaque/42" || all.Photos[2].ID != "p2" {
		t.Fatalf("ordering = albums %+v photos %+v", all.Albums, all.Photos)
	}
	album, err := store.Gallery(t.Context(), "album-a", now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(album.Photos) != 2 || album.Photos[0].ID != "p3" || album.Photos[1].ID != "opaque/42" {
		t.Fatalf("album ordering = %+v", album.Photos)
	}
	unknown, _ := store.Gallery(t.Context(), "does-not-exist", now, 1)
	if unknown.Filter != "all" {
		t.Fatalf("unknown filter = %q", unknown.Filter)
	}
	expired, _ := store.Gallery(t.Context(), "all", now.Add(24*time.Hour), 1)
	if expired.Available || len(expired.Photos) > 0 {
		t.Fatal("expired cache was served")
	}
}

func TestGalleryPagination(t *testing.T) {
	store := testStore(t)
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	const total = flickr.PhotosPerPage + 15
	photos := make([]flickr.Photo, total)
	for i := range photos {
		// Day 1 is oldest, so the last photo created is the newest.
		photos[i] = photo(fmt.Sprintf("p%02d", i), fmt.Sprintf("photo %02d", i), fmt.Sprintf("2026-01-%02d 10:00:00", i+1))
	}
	groups := []flickr.AlbumPhotos{{Album: flickr.Album{ID: "album", Title: "Album"}, Photos: photos}}
	if err := store.ReplaceSnapshot(t.Context(), groups, now); err != nil {
		t.Fatal(err)
	}

	page1, err := store.Gallery(t.Context(), "all", now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page1.TotalPhotos != total || page1.TotalPages != 2 || page1.PageSize != flickr.PhotosPerPage {
		t.Fatalf("page1 metadata = %+v", page1)
	}
	if len(page1.Photos) != flickr.PhotosPerPage || page1.Photos[0].ID != fmt.Sprintf("p%02d", total-1) {
		t.Fatalf("page1 photos = %+v", page1.Photos)
	}

	page2, err := store.Gallery(t.Context(), "all", now, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Photos) != total-flickr.PhotosPerPage {
		t.Fatalf("page2 photos = %+v", page2.Photos)
	}
	if page2.Photos[len(page2.Photos)-1].ID != "p00" {
		t.Fatalf("page2 should end at the oldest photo, got %+v", page2.Photos)
	}

	// Out-of-range pages clamp into [1, TotalPages] instead of erroring or
	// returning an empty set.
	clampedHigh, err := store.Gallery(t.Context(), "all", now, 99)
	if err != nil || clampedHigh.Page != 2 || len(clampedHigh.Photos) != total-flickr.PhotosPerPage {
		t.Fatalf("clamped high page = %+v, %v", clampedHigh, err)
	}
	clampedLow, err := store.Gallery(t.Context(), "all", now, 0)
	if err != nil || clampedLow.Page != 1 {
		t.Fatalf("clamped low page = %+v, %v", clampedLow, err)
	}

	// Prev/next and random selection must see the whole filtered set, not
	// just whichever page the gallery view happens to be showing: the
	// oldest photo (last in the dated order, on page 2) wraps forward to
	// the newest photo (first in the order, on page 1).
	oldest, prev, next, _, err := store.Photo(t.Context(), "p00", "all", now)
	if err != nil {
		t.Fatal(err)
	}
	if oldest.ID != "p00" || prev != "p01" || next != fmt.Sprintf("p%02d", total-1) {
		t.Fatalf("oldest photo navigation = %+v prev=%q next=%q", oldest, prev, next)
	}
	// The featured selection is rolled once per snapshot and cached, not
	// re-randomized per request, so it must be a real photo from the
	// current snapshot and stable across repeated calls.
	first, ok, err := store.RandomPhoto(t.Context(), now)
	if err != nil || !ok || !strings.HasPrefix(first.ID, "p") {
		t.Fatalf("random photo = %+v, ok=%v, %v", first, ok, err)
	}
	for i := 0; i < 5; i++ {
		again, ok, err := store.RandomPhoto(t.Context(), now)
		if err != nil || !ok || again.ID != first.ID {
			t.Fatalf("random photo was not cached: got %+v, want %q", again, first.ID)
		}
	}
}

func TestFailedReplacementKeepsActiveSnapshot(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	original := []flickr.AlbumPhotos{{Album: flickr.Album{ID: "a", Title: "A"}, Photos: []flickr.Photo{photo("one", "original", "")}}}
	if err := store.ReplaceSnapshot(t.Context(), original, now); err != nil {
		t.Fatal(err)
	}
	bad := []flickr.AlbumPhotos{{Album: flickr.Album{ID: "b", Title: "B"}, Photos: []flickr.Photo{photo("duplicate", "one", ""), photo("duplicate", "two", "")}}}
	if err := store.ReplaceSnapshot(t.Context(), bad, now.Add(time.Minute)); err == nil {
		t.Fatal("invalid partial snapshot succeeded")
	}
	gallery, err := store.Gallery(t.Context(), "all", now.Add(2*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(gallery.Photos) != 1 || gallery.Photos[0].Title != "original" {
		t.Fatalf("active snapshot changed: %+v", gallery.Photos)
	}
	status, err := store.Status(t.Context(), now.Add(2*time.Minute))
	if err != nil || status.LastError == "" {
		t.Fatalf("status = %+v, %v", status, err)
	}
}

func TestOAuthStateIsSessionBoundOneTimeAndTokensAreEncrypted(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	request := flickr.Token{Value: "request-token", Secret: "request-secret"}
	if err := store.SaveOAuthRequest(t.Context(), "state", "session-a", request, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ConsumeOAuthRequest(t.Context(), "state", "session-b", request.Value, now); err == nil {
		t.Fatal("state accepted for another admin session")
	}
	got, err := store.ConsumeOAuthRequest(t.Context(), "state", "session-a", request.Value, now)
	if err != nil || got.Token != request {
		t.Fatalf("request = %+v, %v", got, err)
	}
	if _, err := store.ConsumeOAuthRequest(t.Context(), "state", "session-a", request.Value, now); err == nil {
		t.Fatal("OAuth state was reusable")
	}
	access := flickr.Token{Value: "plain-token", Secret: "plain-secret"}
	if err := store.SaveConnection(t.Context(), "user", "name", access); err != nil {
		t.Fatal(err)
	}
	var rawToken, rawSecret []byte
	if err := store.DB.QueryRow(`SELECT token_ciphertext,secret_ciphertext FROM flickr_connections`).Scan(&rawToken, &rawSecret); err != nil {
		t.Fatal(err)
	}
	if string(rawToken) == access.Value || string(rawSecret) == access.Secret {
		t.Fatal("access credentials stored in plaintext")
	}
}
