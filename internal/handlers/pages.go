package handlers

import (
	"context"
	"database/sql"
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/go-fuego/fuego"

	"go-port/internal/content"
	"go-port/internal/db"
	"go-port/internal/flickr"
	portmarkdown "go-port/internal/markdown"
	"go-port/internal/views"
)

// Resource groups the dependencies of all HTTP handlers.
type Resource struct {
	PublicBaseURL     string
	AdminBaseURL      string
	Queries           *db.Queries
	DBOS              dbos.DBOSContext
	AppDB             *sql.DB
	DBOSDB            *sql.DB
	AdminUsername     string
	AdminPasswordHash string
	UploadDir         string
	Markdown          *portmarkdown.Renderer
	Flickr            *flickr.Service
}

func (rs Resource) siteSettings(ctx context.Context) (db.SiteSetting, error) {
	settings, err := rs.Queries.GetSiteSettings(ctx)
	if err != nil {
		return db.SiteSetting{}, errors.Wrap(err, "get site settings")
	}
	return settings, nil
}

func (rs Resource) homePage(c fuego.ContextNoBody) (fuego.Templ, error) {
	posts, err := rs.Queries.ListRecentPosts(c.Context(), 3)
	if err != nil {
		return nil, errors.Wrap(err, "list recent posts")
	}
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	var (
		photo    flickr.Photo
		hasPhoto bool
	)
	if rs.Flickr != nil {
		photo, hasPhoto, err = rs.Flickr.Store.RandomPhoto(c.Context(), time.Now().UTC())
		if err != nil {
			return nil, errors.Wrap(err, "load random Flickr photo")
		}
	}
	return views.HomePage(posts, settings, photo, hasPhoto), nil
}

func (rs Resource) softwarePage(c fuego.ContextNoBody) (fuego.Templ, error) {
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	return views.SoftwarePage(content.Projects, settings), nil
}

func (rs Resource) lightroomPage(c fuego.ContextNoBody) (fuego.Templ, error) {
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	return views.LightroomPage(content.Plugins, settings), nil
}

func (rs Resource) aboutPage(c fuego.ContextNoBody) (fuego.Templ, error) {
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	return views.AboutPage(settings), nil
}

func (rs Resource) photographyPage(c fuego.ContextNoBody) (fuego.Templ, error) {
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	if rs.Flickr == nil {
		return views.PhotographyPage(flickr.Gallery{}, settings), nil
	}
	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || page < 1 {
		page = 1
	}
	gallery, err := rs.Flickr.Store.Gallery(c.Context(), c.QueryParam("filter"), time.Now().UTC(), page)
	if err != nil {
		return nil, errors.Wrap(err, "load Flickr gallery")
	}
	return views.PhotographyPage(gallery, settings), nil
}

// photoDetail returns the modal fragment for one gallery photo; prev/next
// wrap around within the current filter.
func (rs Resource) photoDetail(c fuego.ContextNoBody) (fuego.Templ, error) {
	if rs.Flickr == nil {
		return nil, fuego.NotFoundError{Detail: "photo not found"}
	}
	photo, prev, next, filter, err := rs.Flickr.Store.Photo(c.Context(), c.PathParam("photoID"), c.QueryParam("filter"), time.Now().UTC())
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fuego.NotFoundError{Detail: "photo not found"}
	}
	if err != nil {
		return nil, errors.Wrap(err, "load Flickr photo")
	}
	return views.PhotoDetail(photo, prev, next, filter), nil
}

func (rs Resource) journalPage(c fuego.ContextNoBody) (fuego.Templ, error) {
	filter := c.QueryParam("filter")
	if filter != "software" && filter != "photography" {
		filter = "all"
	}
	count, err := rs.Queries.CountJournalPosts(c.Context(), filter)
	if err != nil {
		return nil, errors.Wrap(err, "count journal posts")
	}
	totalPages := max(1, int((count+19)/20))
	page, _ := strconv.Atoi(c.QueryParam("page"))
	page = min(max(1, page), totalPages)
	posts, err := rs.Queries.ListJournalPage(c.Context(), db.ListJournalPageParams{CategoryFilter: filter, PageOffset: int64((page - 1) * 20)})
	if err != nil {
		return nil, errors.Wrap(err, "list posts")
	}
	years, err := rs.Queries.CountPostsByYear(c.Context())
	if err != nil {
		return nil, errors.Wrap(err, "count posts by year")
	}
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	return views.JournalPage(posts, filter, years, settings, page, totalPages), nil
}

func (rs Resource) articlePage(c fuego.ContextNoBody) (fuego.Templ, error) {
	post, err := rs.Queries.GetPostBySlug(c.Context(), c.PathParam("slug"))
	if errors.Is(err, sql.ErrNoRows) {
		redirect, redirectErr := rs.Queries.GetSlugRedirect(c.Context(), c.PathParam("slug"))
		if redirectErr == nil {
			_, _ = c.Redirect(http.StatusMovedPermanently, "/journal/"+redirect.Slug)
			return nil, nil
		}
		if !errors.Is(redirectErr, sql.ErrNoRows) {
			return nil, errors.Wrap(redirectErr, "get slug redirect")
		}
		return nil, fuego.NotFoundError{Detail: "post not found"}
	}
	if err != nil {
		return nil, errors.Wrap(err, "get post")
	}

	body, err := rs.Markdown.Render(post.BodyMd)
	if err != nil {
		return nil, errors.Wrap(err, "render post body")
	}

	older, err := rs.adjacentPost(c, post, rs.Queries.GetOlderPost)
	if err != nil {
		return nil, err
	}
	newer, err := rs.adjacentPost(c, post, rs.Queries.GetNewerPost)
	if err != nil {
		return nil, err
	}

	readMinutes := max(1, len(strings.Fields(post.BodyMd))/200)
	settings, err := rs.siteSettings(c.Context())
	if err != nil {
		return nil, err
	}
	return views.ArticlePage(post, body, readMinutes, older, newer, settings), nil
}

func (rs Resource) adjacentPost(c fuego.ContextNoBody, post db.Post, query func(context.Context, sql.NullTime) (db.Post, error)) (*db.Post, error) {
	adj, err := query(c.Context(), post.PublishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "get adjacent post")
	}
	return &adj, nil
}

// journalFeed serves the journal as RSS 2.0, with links built from the
// request host so the feed works on any domain it is deployed under.
func (rs Resource) journalFeed(w http.ResponseWriter, r *http.Request) {
	posts, err := rs.Queries.ListRecentPosts(r.Context(), 50)
	if err != nil {
		http.Error(w, "failed to list posts", http.StatusInternalServerError)
		return
	}
	base := rs.PublicBaseURL

	settings, err := rs.siteSettings(r.Context())
	if err != nil {
		http.Error(w, "failed to load settings", http.StatusInternalServerError)
		return
	}
	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       settings.DisplayName + " — Journal",
			Link:        base + "/journal",
			Description: settings.JournalDescription,
		},
	}
	for _, p := range posts {
		feed.Channel.Items = append(feed.Channel.Items, rssItem{
			Title:       p.Title,
			Link:        base + "/journal/" + p.Slug,
			GUID:        base + "/journal/" + p.Slug,
			PubDate:     p.PublishedAt.Time.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"),
			Description: p.Teaser,
		})
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	if err := xml.NewEncoder(w).Encode(feed); err != nil {
		// Headers are already sent; nothing useful left to do.
		return
	}
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}
