package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Job is one message from the queue.
// The queue may hand us the same Job more than once, and ID is identical
// every time. That stable ID is what we will use as the idempotency key.
type Job struct {
	ID     string
	UserID string
	Amount int64
}

// Handler does the work for one delivery of a job.
// applied reports whether this delivery changed anything.
type Handler func(ctx context.Context, pool *pgxpool.Pool, job Job) (applied bool, err error)

// creditNaive adds the amount every single time it runs.
// It is correct only if the job is delivered exactly once.
func creditNaive(ctx context.Context, pool *pgxpool.Pool, job Job) (bool, error) {
	_, err := pool.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
		job.Amount, job.UserID)
	if err != nil {
		return false, err
	}
	return true, nil
}

// creditIdempotent gives the same result whether a job runs once or ten times,
// even when the copies run at the same moment.
//
// The rules it follows:
//   - the first delivery of a job credits the wallet (applied = true)
//   - every later delivery of the same job.ID changes nothing (applied = false)
//
// How: writing the receipt IS the check. Only one copy can insert job.ID into
// processed_jobs, and the receipt and the credit commit together or not at all.
func creditIdempotent(ctx context.Context, pool *pgxpool.Pool, job Job) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin: %w", err)
	}
	// Undo everything if we return before Commit. After Commit it does nothing.
	defer tx.Rollback(ctx)

	// Try to take the key: write the receipt. If another copy has already
	// written it, or is writing it right now, this waits for that copy to
	// finish and then inserts 0 rows instead of failing.
	receipt, err := tx.Exec(ctx,
		`INSERT INTO processed_jobs (job_id) VALUES ($1) ON CONFLICT (job_id) DO NOTHING`,
		job.ID)
	if err != nil {
		return false, fmt.Errorf("write receipt: %w", err)
	}
	if receipt.RowsAffected() == 0 {
		return false, nil // another copy already did this job: skip
	}

	if crashPoint() {
		return false, errCrashed // returning before Commit rolls the receipt back too
	}

	// We got the key. Do the real work in the SAME transaction as the receipt.
	credit, err := tx.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
		job.Amount, job.UserID)
	if err != nil {
		return false, fmt.Errorf("credit wallet: %w", err)
	}
	if credit.RowsAffected() == 0 {
		// No such wallet. Returning here rolls back the receipt too, so we
		// never record a job as done when nothing was credited.
		return false, fmt.Errorf("wallet for %q not found", job.UserID)
	}

	// The receipt and the credit become visible together, or not at all.
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}
