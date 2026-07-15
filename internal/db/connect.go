package db

import (
	"database/sql"
	"embed"

	"github.com/cockroachdb/errors"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect opens a SQLite database with sane defaults for a web app:
// WAL for concurrent readers, a busy timeout instead of immediate
// SQLITE_BUSY errors, and foreign keys enforced.
func Connect(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errors.Wrapf(err, "open sqlite database %q", path)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, errors.Wrapf(err, "ping sqlite database %q", path)
	}
	return sqlDB, nil
}

// Migrate applies all embedded goose migrations.
func Migrate(sqlDB *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return errors.Wrap(err, "set goose dialect")
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return errors.Wrap(err, "run migrations")
	}
	return nil
}
