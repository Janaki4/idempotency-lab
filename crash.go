package main

import (
	"errors"
	"sync/atomic"
)

// crashArmed is set by the -crash flag. The first handler that reaches a
// crashPoint() while it is armed "dies" right there, like a worker whose
// machine loses power mid-job. It fires only once, so the retry runs normally.
var crashArmed atomic.Bool

var errCrashed = errors.New("worker crashed at this point (simulated)")

// crashPoint reports whether the worker should die here.
func crashPoint() bool {
	return crashArmed.CompareAndSwap(true, false)
}
