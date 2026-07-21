// Package workflows registers DBOS durable workflows.
package workflows

import (
	"time"

	"github.com/dbos-inc/dbos-transact-golang/dbos"

	"go-port/internal/flickr"
)

// Register registers all durable workflows with the DBOS context. It must be
// called before dbos.Launch.
func Register(ctx dbos.DBOSContext, service *flickr.Service) {
	refresh := func(ctx dbos.DBOSContext, _ time.Time) (string, error) {
		if !service.Ready() {
			return "Flickr is not configured", nil
		}
		if err := service.Sync(ctx); err != nil {
			return "", err
		}
		return "Flickr snapshot refreshed", nil
	}
	dbos.RegisterWorkflow(ctx, refresh,
		dbos.WithWorkflowName("refreshFlickrCache"),
		dbos.WithSchedule("0 17 */12 * * *"),
	)
}
