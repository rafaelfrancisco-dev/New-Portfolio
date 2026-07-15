package workflows_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbos-inc/dbos-transact-golang/dbos"

	"go-port/internal/db"
	"go-port/internal/workflows"
)

// TestCleanupCompleted runs the workflow through a real DBOS runtime backed
// by a throwaway SQLite system database.
func TestCleanupCompleted(t *testing.T) {
	dir := t.TempDir()

	appDB, err := db.Connect(filepath.Join(dir, "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { appDB.Close() })
	if err := db.Migrate(appDB); err != nil {
		t.Fatal(err)
	}
	queries := db.New(appDB)

	dbosDB, err := db.Connect(filepath.Join(dir, "dbos.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbosDB.Close() })
	dbosCtx, err := dbos.NewDBOSContext(context.Background(), dbos.Config{
		AppName:        "go-port-test",
		SqliteSystemDB: dbosDB,
	})
	if err != nil {
		t.Fatal(err)
	}
	workflows.Register(dbosCtx, queries)
	if err := dbos.Launch(dbosCtx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dbos.Shutdown(dbosCtx, 5*time.Second) })

	ctx := context.Background()
	done, err := queries.CreateTodo(ctx, "done")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queries.ToggleTodo(ctx, done.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := queries.CreateTodo(ctx, "pending"); err != nil {
		t.Fatal(err)
	}

	handle, err := dbos.RunWorkflow(dbosCtx, workflows.CleanupCompleted, workflows.CleanupInput{
		Cutoff: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	remaining, err := queries.ListTodos(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Title != "pending" {
		t.Errorf("remaining = %+v, want only %q", remaining, "pending")
	}
}
