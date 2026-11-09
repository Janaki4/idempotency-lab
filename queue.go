package main

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// result is what happened on one delivery of the job.
type result struct {
	delivery int   // which copy this was: 1, 2, 3, ...
	applied  bool  // true: this copy changed the balance; false: it skipped
	err      error // non-nil: the handler failed
}

// deliver plays the part of an at-least-once queue: it hands the SAME job
// to the handler `times` times.
//
// concurrent = false: one copy after another. Picture: the worker did the
// work, crashed before acking, so the queue delivered the job again.
//
// concurrent = true: every copy at the same moment. Picture: the lease
// expired while worker 1 was still running, so worker 2 got the job too.
func deliver(ctx context.Context, pool *pgxpool.Pool, handle Handler, job Job, times int, concurrent bool) []result {
	results := make([]result, times)

	if !concurrent {
		for i := range times {
			applied, err := handle(ctx, pool, job)
			results[i] = result{delivery: i + 1, applied: applied, err: err}
		}
		return results
	}

	// Start every copy, hold them all at a gate, then open the gate at once
	// so they really hit the database together.
	gate := make(chan struct{})
	var wg sync.WaitGroup
	for i := range times {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-gate
			applied, err := handle(ctx, pool, job)
			// Each goroutine writes only its own slot, so no lock is needed.
			results[i] = result{delivery: i + 1, applied: applied, err: err}
		}()
	}
	close(gate)
	wg.Wait()
	return results
}
