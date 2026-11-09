# idempotency-lab

A queue that delivers "at least once" will sometimes hand a worker the same job twice. This lab shows what that does to a naive handler, and is where we write one that is safe to run any number of times.

The job: credit alice's wallet with 100. However many times it is delivered, she should end up with 100.

## Run it

Start Postgres 16 on `localhost:55433` with the schema loaded:

```sh
make up
```

Deliver the job twice, one after another (a worker crashed before acking):

```sh
go run . -handler naive -deliveries 2
```

Deliver it 10 times at the same moment (several workers got the same job):

```sh
go run . -handler naive -deliveries 10 -concurrent
```

The naive handler ends at 200, then 1000. Both are wrong. Run the same two commands with `-handler idempotent` and both end at 100.

Now make the first delivery crash halfway, between saving the receipt and crediting the wallet:

```sh
go run . -handler receipt-first -deliveries 2 -crash
go run . -handler credit-first -deliveries 2 -crash
go run . -handler idempotent -deliveries 2 -crash
```

Saving the receipt and the credit as two separate commits is wrong whichever goes first: `receipt-first` loses the credit (0) and `credit-first` pays twice (200). Only `idempotent`, which does both in one transaction, ends at 100.

Stop and delete the database when you're done:

```sh
make down
```

## Read in this order

1. `schema.sql`: the two tables
2. `handlers.go`: the job, the naive handler, and the idempotent one
3. `broken.go`: two handlers that split the receipt and the credit into separate commits
4. `crash.go`: the `-crash` switch that kills the first delivery halfway
5. `queue.go`: the fake queue that delivers the same job again
6. `main.go`: wires it together and prints the result
7. `db.go`: connect, reset, read the balance

## Next

The same idea for an HTTP API that takes an `Idempotency-Key` header.
