package handlers

import (
	"net/http"

	"github.com/go-fuego/fuego"
	"github.com/go-fuego/fuego/option"

	"go-port/static"
)

// Register mounts all routes on the fuego server. Page/fragment routes and
// the JSON API alike show up in the generated OpenAPI spec at /swagger.
func Register(s *fuego.Server, rs Resource) {
	// SSR pages & HTMX fragments
	fuego.Get(s, "/", rs.indexPage,
		option.Summary("Todos page"),
		option.Tags("pages"),
	)
	fuego.Post(s, "/todos", rs.createTodo,
		option.Summary("Create a todo"),
		option.Description("Returns the new row as an HTML fragment for HTMX."),
		option.Tags("todos"),
	)
	fuego.Patch(s, "/todos/{id}/toggle", rs.toggleTodo,
		option.Summary("Toggle a todo"),
		option.Tags("todos"),
	)
	fuego.Delete(s, "/todos/{id}", rs.deleteTodo,
		option.Summary("Delete a todo"),
		option.Tags("todos"),
	)
	fuego.Post(s, "/admin/cleanup", rs.cleanupCompleted,
		option.Summary("Delete completed todos"),
		option.Description("Runs a DBOS durable workflow."),
		option.Tags("admin"),
	)

	// JSON API (content-negotiated: HTML for browsers, JSON for clients)
	fuego.Get(s, "/api/todos", rs.listTodos,
		option.Summary("List todos"),
		option.Tags("api"),
	)

	// Ops
	fuego.Get(s, "/healthz", rs.healthz,
		option.Summary("Health check"),
		option.Tags("ops"),
	)

	// Embedded static assets (htmx, compiled tailwind css)
	fuego.GetStd(s, "/static/", http.StripPrefix("/static/", http.FileServerFS(static.FS)).ServeHTTP,
		option.Hide(),
	)
}
