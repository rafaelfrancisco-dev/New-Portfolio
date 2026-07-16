package db

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func testQueries(t *testing.T) (*sql.DB, *Queries) {
	t.Helper()
	sqlDB, err := Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}
	return sqlDB, New(sqlDB)
}

func TestMigrationDefaultsAndPublicationQueries(t *testing.T) {
	_, queries := testQueries(t)
	posts, err := queries.ListPosts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 4 {
		t.Fatalf("published posts = %d, want 4", len(posts))
	}
	for _, post := range posts {
		if post.Status != "published" || !post.PublishedAt.Valid || post.CreatedAt.IsZero() || post.UpdatedAt.IsZero() {
			t.Fatalf("bad migrated defaults: %+v", post)
		}
	}

	draft, err := queries.CreatePost(t.Context(), CreatePostParams{
		Slug: "database-draft", Title: "Database Draft", Category: "software",
		Teaser: "teaser", Lede: "lede", BodyMd: "body", Status: "draft",
	})
	if err != nil {
		t.Fatal(err)
	}
	if draft.PublishedAt.Valid {
		t.Fatal("draft publication date should be null")
	}
	posts, err = queries.ListPosts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 4 {
		t.Fatalf("draft leaked into public list: %d posts", len(posts))
	}
	if _, err := queries.GetPostBySlug(t.Context(), draft.Slug); err != sql.ErrNoRows {
		t.Fatalf("public draft lookup error = %v, want sql.ErrNoRows", err)
	}
}

func TestPostCRUDSlugValidationAndRedirects(t *testing.T) {
	sqlDB, queries := testQueries(t)
	post, err := queries.CreatePost(t.Context(), CreatePostParams{
		Slug: "first-slug", Title: "Post", Category: "photography", Teaser: "teaser",
		Lede: "lede", BodyMd: "body", Status: "published",
		PublishedAt: sql.NullTime{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	inUse, err := queries.SlugInUse(t.Context(), SlugInUseParams{Slug: post.Slug, ID: 0, OldSlug: post.Slug})
	if err != nil || !inUse {
		t.Fatalf("slug in use = %v, err = %v", inUse, err)
	}

	tx, err := sqlDB.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	qtx := queries.WithTx(tx)
	updated, err := qtx.UpdatePost(t.Context(), UpdatePostParams{
		Slug: "second-slug", Title: post.Title, Category: post.Category, Teaser: post.Teaser,
		Lede: post.Lede, BodyMd: post.BodyMd, StoreUrl: post.StoreUrl, Status: post.Status,
		PublishedAt: post.PublishedAt, ID: post.ID,
		UpdatedAt: post.UpdatedAt.Format("2006-01-02 15:04:05.999999999"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := qtx.CreateSlugRedirect(t.Context(), CreateSlugRedirectParams{OldSlug: post.Slug, PostID: post.ID}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	redirected, err := queries.GetSlugRedirect(t.Context(), post.Slug)
	if err != nil || redirected.Slug != updated.Slug {
		t.Fatalf("redirect = %+v, err = %v", redirected, err)
	}
	if _, err := queries.UpdatePost(t.Context(), UpdatePostParams{
		Slug: updated.Slug, Title: "stale", Category: updated.Category, Teaser: updated.Teaser,
		Lede: updated.Lede, BodyMd: updated.BodyMd, Status: updated.Status,
		PublishedAt: updated.PublishedAt, ID: updated.ID, UpdatedAt: "2000-01-01 00:00:00",
	}); err != sql.ErrNoRows {
		t.Fatalf("stale update error = %v, want sql.ErrNoRows", err)
	}
	deleted, err := queries.DeletePost(t.Context(), DeletePostParams{
		ID: updated.ID, UpdatedAt: updated.UpdatedAt.Format("2006-01-02 15:04:05.999999999"),
	})
	if err != nil || deleted != 1 {
		t.Fatalf("delete rows = %d, err = %v", deleted, err)
	}
}

func TestSettingsSessionsAndMediaRecords(t *testing.T) {
	_, queries := testQueries(t)
	settings, err := queries.UpdateSiteSettings(t.Context(), UpdateSiteSettingsParams{
		DisplayName: "Test Person", SiteHandle: "test.person", SiteTitle: "Test Portfolio",
		SiteDescription: "description", JournalDescription: "journal",
		ContactEmail: "test@example.com", GithubUrl: "https://github.com/test",
	})
	if err != nil || settings.DisplayName != "Test Person" {
		t.Fatalf("settings = %+v, err = %v", settings, err)
	}

	expires := time.Now().UTC().Add(time.Hour)
	if err := queries.CreateAdminSession(t.Context(), CreateAdminSessionParams{
		TokenHash: "hash", CsrfToken: "csrf", ExpiresAt: expires,
	}); err != nil {
		t.Fatal(err)
	}
	session, err := queries.GetAdminSession(t.Context(), "hash")
	if err != nil || session.CsrfToken != "csrf" {
		t.Fatalf("session = %+v, err = %v", session, err)
	}
	if err := queries.DeleteAdminSession(t.Context(), "hash"); err != nil {
		t.Fatal(err)
	}

	media, err := queries.CreateMedia(t.Context(), CreateMediaParams{
		Filename: "random.png", OriginalFilename: "photo.png", MimeType: "image/png", SizeBytes: 123,
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := queries.ListMedia(t.Context())
	if err != nil || len(items) != 1 || items[0].InUse {
		t.Fatalf("media list = %+v, err = %v", items, err)
	}
	if err := queries.DeleteMedia(t.Context(), media.ID); err != nil {
		t.Fatal(err)
	}
}
