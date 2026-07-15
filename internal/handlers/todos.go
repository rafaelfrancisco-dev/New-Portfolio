package handlers

import (
	"database/sql"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/go-fuego/fuego"

	"go-port/internal/db"
	"go-port/internal/views"
	"go-port/internal/workflows"
)

// Resource groups the dependencies of all HTTP handlers.
type Resource struct {
	Queries *db.Queries
	DBOS    dbos.DBOSContext
	AppDB   *sql.DB
	DBOSDB  *sql.DB
}

// CreateTodoForm is decoded from HTMX's application/x-www-form-urlencoded
// body (schema tag) and documented in the OpenAPI spec (json tag).
type CreateTodoForm struct {
	Title string `json:"title" schema:"title" validate:"required,max=200"`
}

func (rs Resource) indexPage(c fuego.ContextNoBody) (fuego.Templ, error) {
	todos, err := rs.Queries.ListTodos(c.Context())
	if err != nil {
		return nil, errors.Wrap(err, "list todos")
	}
	return views.TodosPage(todos), nil
}

func (rs Resource) createTodo(c fuego.ContextWithBody[CreateTodoForm]) (fuego.Templ, error) {
	form, err := c.Body()
	if err != nil {
		return nil, err
	}
	todo, err := rs.Queries.CreateTodo(c.Context(), form.Title)
	if err != nil {
		return nil, errors.Wrap(err, "create todo")
	}
	return views.TodoRow(todo), nil
}

func (rs Resource) toggleTodo(c fuego.ContextNoBody) (fuego.Templ, error) {
	todo, err := rs.Queries.ToggleTodo(c.Context(), int64(c.PathParamInt("id")))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fuego.NotFoundError{Detail: "todo not found"}
	}
	if err != nil {
		return nil, errors.Wrap(err, "toggle todo")
	}
	return views.TodoRow(todo), nil
}

func (rs Resource) deleteTodo(c fuego.ContextNoBody) (string, error) {
	if _, err := rs.Queries.DeleteTodo(c.Context(), int64(c.PathParamInt("id"))); err != nil {
		return "", errors.Wrap(err, "delete todo")
	}
	// Empty 200 body + hx-swap="outerHTML" removes the row client-side.
	return "", nil
}

// listTodos serves both `Accept: application/json` (JSON, in the OpenAPI
// spec) and browsers (HTML fragment) from one handler.
func (rs Resource) listTodos(c fuego.ContextNoBody) (*fuego.DataOrTemplate[[]db.Todo], error) {
	todos, err := rs.Queries.ListTodos(c.Context())
	if err != nil {
		return nil, errors.Wrap(err, "list todos")
	}
	return fuego.DataOrHTML(todos, views.TodoList(todos)), nil
}

// cleanupCompleted starts the durable workflow and waits for its result.
// The handle could instead be returned immediately (fire-and-forget) and
// polled via its workflow ID — see the DBOS docs.
func (rs Resource) cleanupCompleted(c fuego.ContextNoBody) (fuego.Templ, error) {
	handle, err := dbos.RunWorkflow(rs.DBOS, workflows.CleanupCompleted, workflows.CleanupInput{
		Cutoff: time.Now(),
	})
	if err != nil {
		return nil, errors.Wrap(err, "start cleanup workflow")
	}
	deleted, err := handle.GetResult()
	if err != nil {
		return nil, errors.Wrap(err, "cleanup workflow failed")
	}
	return views.CleanupResult(deleted), nil
}
