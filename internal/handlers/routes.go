package handlers

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"

	"go-port/internal/markdown"
	"go-port/static"
)

// Register mounts all routes on the fuego server. Page routes show up in the
// generated OpenAPI spec at /swagger.
func Register(s *fuego.Server, rs Resource) {
	if rs.AdminUsername == "" {
		rs.AdminUsername = "admin"
	}
	if rs.UploadDir == "" {
		rs.UploadDir = "uploads"
	}
	if rs.Markdown == nil {
		rs.Markdown = markdown.NewRenderer()
	}
	// SSR pages
	fuego.Get(s, "/", rs.homePage,
		option.Summary("Home page"),
		option.Tags("pages"),
	)
	fuego.Get(s, "/software", rs.softwarePage,
		option.Summary("Software projects"),
		option.Tags("pages"),
	)
	fuego.Get(s, "/photography", rs.photographyPage,
		option.Summary("Photography gallery"),
		option.Query("filter", "Gallery filter: all or an opaque Flickr album ID"),
		option.Tags("pages"),
	)
	fuego.Get(s, "/journal", rs.journalPage,
		option.Summary("Journal index"),
		option.Query("filter", "Category filter: all, software or photography"),
		option.Tags("pages"),
	)
	fuego.Get(s, "/journal/{slug}", rs.articlePage,
		option.Summary("Journal article"),
		option.Tags("pages"),
	)
	fuego.Get(s, "/about", rs.aboutPage,
		option.Summary("About & contact"),
		option.Tags("pages"),
	)
	fuego.Get(s, "/lightroom", rs.lightroomPage,
		option.Summary("Lightroom plug-ins"),
		option.Tags("pages"),
	)

	// HTMX fragments
	fuego.Get(s, "/photography/photos/{photoID}", rs.photoDetail,
		option.Summary("Photo detail modal fragment"),
		option.Query("filter", "Gallery filter the modal navigates within"),
		option.Hide(),
	)

	// Feeds
	fuego.GetStd(s, "/journal/feed.xml", rs.journalFeed,
		option.Summary("Journal RSS feed"),
		option.Tags("feeds"),
	)

	// Uploaded media is public, but only database-recorded randomized names
	// can be served from the configured persistent directory.
	fuego.GetStd(s, "/media/{filename}", rs.serveMedia, option.Hide())

	// Ops
	fuego.Get(s, "/healthz", rs.healthz,
		option.Summary("Health check"),
		option.Tags("ops"),
	)

	// Embedded static assets (htmx, compiled tailwind css, fonts)
	fuego.GetStd(s, "/static/", http.StripPrefix("/static/", http.FileServerFS(static.FS)).ServeHTTP,
		option.Hide(),
	)

	registerAdminRoutes(s, rs)
}

func registerAdminRoutes(s *fuego.Server, rs Resource) {
	admin := fuego.Group(s, "/admin")
	fuego.GetStd(admin, "/login", rs.adminLoginPage, option.Hide())
	fuego.PostStd(admin, "/login", rs.adminLogin, option.Hide())

	fuego.Use(admin, rs.adminAuthMiddleware)
	fuego.GetStd(admin, "", rs.adminDashboard, option.Hide())
	fuego.PostStd(admin, "/logout", rs.adminLogout, option.Hide())
	fuego.GetStd(admin, "/posts", rs.adminPosts, option.Hide())
	fuego.GetStd(admin, "/posts/new", rs.adminNewPost, option.Hide())
	fuego.PostStd(admin, "/posts", rs.adminCreatePost, option.Hide())
	fuego.GetStd(admin, "/posts/{id}/edit", rs.adminEditPost, option.Hide())
	fuego.PostStd(admin, "/posts/{id}", rs.adminUpdatePost, option.Hide())
	fuego.PostStd(admin, "/posts/{id}/delete", rs.adminDeletePost, option.Hide())
	fuego.PostStd(admin, "/posts/preview", rs.adminPreview, option.Hide())
	fuego.GetStd(admin, "/media", rs.adminMedia, option.Hide())
	fuego.PostStd(admin, "/media", rs.adminUploadMedia, option.Hide())
	fuego.PostStd(admin, "/media/{id}/delete", rs.adminDeleteMedia, option.Hide())
	fuego.GetStd(admin, "/settings", rs.adminSettings, option.Hide())
	fuego.PostStd(admin, "/settings", rs.adminUpdateSettings, option.Hide())
	fuego.GetStd(admin, "/flickr", rs.adminFlickr, option.Hide())
	fuego.PostStd(admin, "/flickr/connect", rs.adminFlickrConnect, option.Hide())
	fuego.GetStd(admin, "/flickr/callback", rs.adminFlickrCallback, option.Hide())
	fuego.PostStd(admin, "/flickr/sync", rs.adminFlickrSync, option.Hide())
	fuego.PostStd(admin, "/flickr/disconnect", rs.adminFlickrDisconnect, option.Hide())
}
