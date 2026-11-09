package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These two handlers use the same receipt table as creditIdempotent, but they
// write the receipt and the credit in SEPARATE transactions. Each pool.Exec
// below commits on its own the moment it returns. Run them with -crash to see
// why that is not safe.

// creditReceiptFirst saves the receipt, then credits in a second step.
func creditReceiptFirst(ctx context.Context, pool *pgxpool.Pool, job Job) (bool, error) {
	// Step 1: the receipt. Saved for good once this returns.
	receipt, err := pool.Exec(ctx,
		`INSERT INTO processed_jobs (job_id) VALUES ($1) ON CONFLICT (job_id) DO NOTHING`,
		job.ID)
	if err != nil {
		return false, fmt.Errorf("write receipt: %w", err)
	}
	if receipt.RowsAffected() == 0 {
		return false, nil // receipt already there: skip
	}

	if crashPoint() {
		return false, errCrashed // the receipt is saved, the credit never happens
	}

	// Step 2: the credit.
	if _, err := pool.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
		job.Amount, job.UserID); err != nil {
		return false, fmt.Errorf("credit wallet: %w", err)
	}
	return true, nil
}

// creditCreditFirst credits, then saves the receipt in a second step.
// Because the receipt comes last, it has to look for it before crediting.
func creditCreditFirst(ctx context.Context, pool *pgxpool.Pool, job Job) (bool, error) {
	var done bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM processed_jobs WHERE job_id = $1)`,
		job.ID).Scan(&done); err != nil {
		return false, fmt.Errorf("check receipt: %w", err)
	}
	if done {
		return false, nil // receipt already there: skip
	}

	// Step 1: the credit. Saved for good once this returns.
	if _, err := pool.Exec(ctx,
		`UPDATE wallets SET balance = balance + $1 WHERE user_id = $2`,
		job.Amount, job.UserID); err != nil {
		return false, fmt.Errorf("credit wallet: %w", err)
	}

	if crashPoint() {
		return false, errCrashed // the money is credited, the receipt never gets written
	}

	// Step 2: the receipt.
	if _, err := pool.Exec(ctx,
		`INSERT INTO processed_jobs (job_id) VALUES ($1) ON CONFLICT (job_id) DO NOTHING`,
		job.ID); err != nil {
		return false, fmt.Errorf("write receipt: %w", err)
	}
	return true, nil
}
