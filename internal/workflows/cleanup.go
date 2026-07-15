// Package workflows holds DBOS durable workflows. Workflows must be
// deterministic: anything that touches the outside world (database, HTTP,
// time, randomness) belongs inside a dbos.RunAsStep call, so that on crash
// recovery completed steps are replayed from the DBOS system database
// instead of re-executed.
package workflows

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"

	"go-port/internal/db"
)

// queries is injected once at startup via Register. DBOS workflows are
// package-level functions (they must be registered by name for recovery),
// so dependencies are provided through this package variable.
var queries *db.Queries

// Register wires dependencies and registers all workflows on the DBOS
// context. Must be called before dbos.Launch.
func Register(dbosCtx dbos.DBOSContext, q *db.Queries) {
	queries = q
	dbos.RegisterWorkflow(dbosCtx, CleanupCompleted)
	// Cron format has second precision (6 fields): nightly at 03:00.
	dbos.RegisterWorkflow(dbosCtx, ScheduledCleanup, dbos.WithSchedule("0 0 3 * * *"))
}

// CleanupInput carries the cutoff computed by the caller — computing
// time.Now() inside the workflow body would be non-deterministic on replay.
type CleanupInput struct {
	Cutoff time.Time `json:"cutoff"`
}

// CleanupCompleted deletes completed todos created before the cutoff.
// It runs as two durable steps so a crash between them resumes at the
// delete instead of restarting from scratch.
func CleanupCompleted(ctx dbos.DBOSContext, input CleanupInput) (int64, error) {
	candidates, err := dbos.RunAsStep(ctx, func(stepCtx context.Context) (int64, error) {
		return queries.CountCompletedBefore(stepCtx, input.Cutoff)
	}, dbos.WithStepName("CountCompleted"))
	if err != nil {
		return 0, errors.Wrap(err, "count completed todos")
	}
	if candidates == 0 {
		return 0, nil
	}

	deleted, err := dbos.RunAsStep(ctx, func(stepCtx context.Context) (int64, error) {
		return queries.DeleteCompletedBefore(stepCtx, input.Cutoff)
	}, dbos.WithStepName("DeleteCompleted"), dbos.WithStepMaxRetries(3))
	if err != nil {
		return 0, errors.Wrap(err, "delete completed todos")
	}
	return deleted, nil
}

// ScheduledCleanup is the cron-triggered entry point. Scheduled workflows
// receive the scheduled firing time as input; it doubles as the cutoff.
// It invokes CleanupCompleted as a durable child workflow.
func ScheduledCleanup(ctx dbos.DBOSContext, scheduledAt time.Time) (int64, error) {
	handle, err := dbos.RunWorkflow(ctx, CleanupCompleted, CleanupInput{Cutoff: scheduledAt})
	if err != nil {
		return 0, errors.Wrap(err, "start cleanup child workflow")
	}
	return handle.GetResult()
}
