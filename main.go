// Command idempotency-lab delivers the same job several times, the way an
// at-least-once queue does, and shows whether the user got paid once or more.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

func main() {
	handlerName := flag.String("handler", "naive", "which handler runs the job: naive, idempotent, receipt-first or credit-first")
	times := flag.Int("deliveries", 2, "how many times the queue delivers the same job")
	concurrent := flag.Bool("concurrent", false, "deliver every copy at the same moment instead of one after another")
	crash := flag.Bool("crash", false, "make the first delivery die halfway, between the receipt and the credit")
	dbURL := flag.String("db", "postgres://lab:lab@localhost:55433/lab?sslmode=disable&pool_max_conns=20", "Postgres connection URL")
	flag.Parse()

	handlers := map[string]Handler{
		"naive":         creditNaive,
		"idempotent":    creditIdempotent,
		"receipt-first": creditReceiptFirst,
		"credit-first":  creditCreditFirst,
	}
	handle, ok := handlers[*handlerName]
	if !ok {
		log.Fatalf("unknown handler %q: use naive, idempotent, receipt-first or credit-first", *handlerName)
	}
	if *times < 1 {
		log.Fatal("deliveries must be at least 1")
	}
	crashArmed.Store(*crash)

	ctx := context.Background()
	pool, err := connect(ctx, *dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	job := Job{ID: "job-42", UserID: "alice", Amount: 100}
	if err := reset(ctx, pool, job.UserID); err != nil {
		log.Fatal(err)
	}

	mode := "one after another"
	if *concurrent {
		mode = "all at the same moment"
	}
	fmt.Printf("Job %s: credit %s with %d\n", job.ID, job.UserID, job.Amount)
	fmt.Printf("Handler: %s · delivered %d times, %s\n\n", *handlerName, *times, mode)

	for _, r := range deliver(ctx, pool, handle, job, *times, *concurrent) {
		switch {
		case r.err != nil:
			fmt.Printf("  delivery %d: failed: %v\n", r.delivery, r.err)
		case r.applied:
			fmt.Printf("  delivery %d: credited +%d\n", r.delivery, job.Amount)
		default:
			fmt.Printf("  delivery %d: skipped (already processed)\n", r.delivery)
		}
	}

	got, err := balance(ctx, pool, job.UserID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nFinal balance: %d (should be %d)", got, job.Amount)
	switch {
	case got == job.Amount:
		fmt.Println("  → correct")
	case got == 0:
		fmt.Println("  → WRONG: the credit was lost")
	default:
		fmt.Printf("  → WRONG: paid %d times\n", got/job.Amount)
	}
}
