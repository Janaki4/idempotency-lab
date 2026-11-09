package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connect opens a connection pool and checks that the database is reachable.
func connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("can't reach postgres (did you run make up?): %w", err)
	}
	return pool, nil
}

// reset puts the lab back to the same starting point before every run:
// the user's balance is 0 and no job has been processed.
func reset(ctx context.Context, pool *pgxpool.Pool, userID string) error {
	if _, err := pool.Exec(ctx, `TRUNCATE processed_jobs`); err != nil {
		return fmt.Errorf("clear receipts: %w", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO wallets (user_id, balance) VALUES ($1, 0)
		 ON CONFLICT (user_id) DO UPDATE SET balance = 0`, userID); err != nil {
		return fmt.Errorf("reset wallet: %w", err)
	}
	return nil
}

// balance reads the user's current balance.
func balance(ctx context.Context, pool *pgxpool.Pool, userID string) (int64, error) {
	var b int64
	err := pool.QueryRow(ctx, `SELECT balance FROM wallets WHERE user_id = $1`, userID).Scan(&b)
	return b, err
}
