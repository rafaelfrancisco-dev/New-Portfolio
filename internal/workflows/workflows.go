// Package workflows registers DBOS durable workflows. The portfolio has no
// workflows yet, but DBOS stays wired so scheduled jobs (e.g. a periodic
// Flickr cache refresh) can be added without touching main.
package workflows

import (
	"github.com/dbos-inc/dbos-transact-golang/dbos"

	"go-port/internal/db"
)

// Register registers all durable workflows with the DBOS context. It must be
// called before dbos.Launch.
func Register(ctx dbos.DBOSContext, queries *db.Queries) {}
