package handlers

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"

	"go-port/static"
)

// Register mounts all routes on the fuego server. Page routes show up in the
// generated OpenAPI spec at /swagger.
func Register(s *fuego.Server, rs Resource) {
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
		option.Query("filter", "Gallery filter: all, landscapes or wildlife"),
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

	// HTMX fragments
	fuego.Get(s, "/photography/photos/{index}", rs.photoDetail,
		option.Summary("Photo detail modal fragment"),
		option.Query("filter", "Gallery filter the modal navigates within"),
		option.Hide(),
	)

	// Feeds
	fuego.GetStd(s, "/journal/feed.xml", rs.journalFeed,
		option.Summary("Journal RSS feed"),
		option.Tags("feeds"),
	)

	// Ops
	fuego.Get(s, "/healthz", rs.healthz,
		option.Summary("Health check"),
		option.Tags("ops"),
	)

	// Embedded static assets (htmx, compiled tailwind css, fonts)
	fuego.GetStd(s, "/static/", http.StripPrefix("/static/", http.FileServerFS(static.FS)).ServeHTTP,
		option.Hide(),
	)
}
