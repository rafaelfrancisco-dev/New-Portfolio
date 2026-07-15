package handlers_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-fuego/fuego"

	"go-port/internal/db"
	"go-port/internal/handlers"
)

// newTestServer spins up the real route table against a throwaway SQLite
// file. DBOS is left nil: these tests cover the plain CRUD routes; the
// workflow path is exercised in internal/workflows.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	sqlDB, err := db.Connect(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Migrate(sqlDB); err != nil {
		t.Fatal(err)
	}

	s := fuego.NewServer()
	handlers.Register(s, handlers.Resource{
		Queries: db.New(sqlDB),
		AppDB:   sqlDB,
		DBOSDB:  sqlDB,
	})
	ts := httptest.NewServer(s.Mux)
	t.Cleanup(ts.Close)
	return ts
}

func postForm(t *testing.T, ts *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()
	resp, err := http.PostForm(ts.URL+path, form)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestCreateTodoReturnsFragment(t *testing.T) {
	ts := newTestServer(t)

	resp := postForm(t, ts, "/todos", url.Values{"title": {"write tests"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "write tests") || !strings.Contains(string(body), `id="todo-`) {
		t.Errorf("expected an HTML row fragment, got: %s", body)
	}
}

func TestCreateTodoRequiresTitle(t *testing.T) {
	ts := newTestServer(t)

	resp := postForm(t, ts, "/todos", url.Values{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestListTodosContentNegotiation(t *testing.T) {
	ts := newTestServer(t)
	postForm(t, ts, "/todos", url.Values{"title": {"negotiate"}})

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/todos", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var todos []db.Todo
	if err := json.NewDecoder(resp.Body).Decode(&todos); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if len(todos) != 1 || todos[0].Title != "negotiate" {
		t.Errorf("todos = %+v, want one titled %q", todos, "negotiate")
	}
}

func TestToggleTodoNotFound(t *testing.T) {
	ts := newTestServer(t)

	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/todos/999/toggle", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
