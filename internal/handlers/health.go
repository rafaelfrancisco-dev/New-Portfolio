package handlers

import (
	"github.com/cockroachdb/errors"
	"github.com/go-fuego/fuego"
)

// healthz reports whether both SQLite handles (app data and DBOS system
// database) are reachable. Suitable as a container/orchestrator probe.
func (rs Resource) healthz(c fuego.ContextNoBody) (string, error) {
	if err := rs.AppDB.PingContext(c.Context()); err != nil {
		return "", errors.Wrap(err, "ping app database")
	}
	if err := rs.DBOSDB.PingContext(c.Context()); err != nil {
		return "", errors.Wrap(err, "ping dbos database")
	}
	return "ok", nil
}
