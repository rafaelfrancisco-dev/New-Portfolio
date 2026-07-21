package flickr_test

import (
	"path/filepath"
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

func photo(id, title string) flickr.Photo {
	return flickr.Photo{ID: id, Title: title, PageURL: "https://flickr.test/" + id, SmallURL: "https://img.test/" + id + "_m.jpg", LargeURL: "https://img.test/" + id + "_b.jpg", Width: 1200, Height: 800}
}

func TestSnapshotDeduplicationOrderingFilteringAndExpiry(t *testing.T) {
	store := testStore(t)
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	shared := photo("opaque/42", "shared")
	groups := []flickr.AlbumPhotos{
		{Album: flickr.Album{ID: "album-z", Title: "First", Position: 0}, Photos: []flickr.Photo{shared, photo("p2", "second")}},
		{Album: flickr.Album{ID: "private-only", Title: "Hidden", Position: 1}},
		{Album: flickr.Album{ID: "album-a", Title: "Second", Position: 2}, Photos: []flickr.Photo{photo("p3", "third"), shared}},
	}
	if err := store.ReplaceSnapshot(t.Context(), groups, now); err != nil {
		t.Fatal(err)
	}
	all, err := store.Gallery(t.Context(), "all", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !all.Available || len(all.Albums) != 2 || len(all.Photos) != 3 {
		t.Fatalf("gallery = %+v", all)
	}
	if all.Albums[0].ID != "album-z" || all.Photos[0].ID != "opaque/42" || all.Photos[2].ID != "p3" {
		t.Fatalf("ordering = albums %+v photos %+v", all.Albums, all.Photos)
	}
	album, err := store.Gallery(t.Context(), "album-a", now)
	if err != nil {
		t.Fatal(err)
	}
	if len(album.Photos) != 2 || album.Photos[0].ID != "p3" || album.Photos[1].ID != "opaque/42" {
		t.Fatalf("album ordering = %+v", album.Photos)
	}
	unknown, _ := store.Gallery(t.Context(), "does-not-exist", now)
	if unknown.Filter != "all" {
		t.Fatalf("unknown filter = %q", unknown.Filter)
	}
	expired, _ := store.Gallery(t.Context(), "all", now.Add(24*time.Hour))
	if expired.Available || len(expired.Photos) > 0 {
		t.Fatal("expired cache was served")
	}
}

func TestFailedReplacementKeepsActiveSnapshot(t *testing.T) {
	store := testStore(t)
	now := time.Now().UTC()
	original := []flickr.AlbumPhotos{{Album: flickr.Album{ID: "a", Title: "A"}, Photos: []flickr.Photo{photo("one", "original")}}}
	if err := store.ReplaceSnapshot(t.Context(), original, now); err != nil {
		t.Fatal(err)
	}
	bad := []flickr.AlbumPhotos{{Album: flickr.Album{ID: "b", Title: "B"}, Photos: []flickr.Photo{photo("duplicate", "one"), photo("duplicate", "two")}}}
	if err := store.ReplaceSnapshot(t.Context(), bad, now.Add(time.Minute)); err == nil {
		t.Fatal("invalid partial snapshot succeeded")
	}
	gallery, err := store.Gallery(t.Context(), "all", now.Add(2*time.Minute))
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
