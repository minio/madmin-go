//
// Copyright (c) 2015-2022 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
//

package madmin

import (
	"context"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// MaxRetry is the maximum number of retries before stopping.
var MaxRetry = 10

// MaxJitter will randomize over the full exponential backoff time
const MaxJitter = 1.0

// NoJitter disables the use of jitter for randomizing the exponential backoff time
const NoJitter = 0.0

// DefaultRetryUnit - default unit multiplicative per retry.
// defaults to 1 second.
const DefaultRetryUnit = time.Second

// DefaultRetryCap - Each retry attempt never waits no longer than
// this maximum time duration.
const DefaultRetryCap = time.Second * 30

// lockedRandSource provides protected rand source, implements rand.Source interface.
type lockedRandSource struct {
	lk  sync.Mutex
	src rand.Source
}

// Int63 returns a non-negative pseudo-random 63-bit integer as an int64.
func (r *lockedRandSource) Int63() (n int64) {
	r.lk.Lock()
	n = r.src.Int63()
	r.lk.Unlock()
	return
}

// Seed uses the provided seed value to initialize the generator to a
// deterministic state.
func (r *lockedRandSource) Seed(seed int64) {
	r.lk.Lock()
	r.src.Seed(seed)
	r.lk.Unlock()
}

// newRetryTimer creates a timer with exponentially increasing
// delays until the maximum retry attempts are reached.
func (adm AdminClient) newRetryTimer(ctx context.Context, maxRetry int, unit time.Duration, cap time.Duration, jitter float64) <-chan int {
	attemptCh := make(chan int)

	// computes the exponential backoff duration according to
	// https://www.awsarchitectureblog.com/2015/03/backoff.html
	exponentialBackoffWait := func(attempt int) time.Duration {
		// normalize jitter to the range [0, 1.0]
		if jitter < NoJitter {
			jitter = NoJitter
		}
		if jitter > MaxJitter {
			jitter = MaxJitter
		}

		// sleep = random_between(0, min(cap, base * 2 ** attempt))
		sleep := unit * 1 << uint(attempt)
		if sleep > cap {
			sleep = cap
		}
		if jitter > NoJitter {
			sleep -= time.Duration(adm.random.Float64() * float64(sleep) * jitter)
		}
		return sleep
	}

	go func() {
		defer close(attemptCh)
		for i := 0; i < maxRetry; i++ {
			// Attempts start from 1.
			select {
			case attemptCh <- i + 1:
			case <-ctx.Done():
				// Stop the routine.
				return
			}

			select {
			case <-time.After(exponentialBackoffWait(i)):
			case <-ctx.Done():
				// Stop the routine.
				return
			}
		}
	}()
	return attemptCh
}

// List of admin error codes which are retryable.
var retryableAdminErrCodes = map[string]struct{}{
	"RequestError":         {},
	"RequestTimeout":       {},
	"Throttling":           {},
	"ThrottlingException":  {},
	"RequestLimitExceeded": {},
	"RequestThrottled":     {},
	"SlowDown":             {},
	// Add more admin error codes here.
}

// isAdminErrCodeRetryable - is admin error code retryable.
func isAdminErrCodeRetryable(code string) (ok bool) {
	_, ok = retryableAdminErrCodes[code]
	return ok
}

// List of HTTP status codes which are retryable.
var retryableHTTPStatusCodes = map[int]struct{}{
	http.StatusRequestTimeout:     {},
	http.StatusTooManyRequests:    {},
	http.StatusBadGateway:         {},
	http.StatusServiceUnavailable: {},
	// Add more HTTP status codes here.
}

// isHTTPStatusRetryable - is HTTP error code retryable.
func isHTTPStatusRetryable(httpStatusCode int) (ok bool) {
	_, ok = retryableHTTPStatusCodes[httpStatusCode]
	return ok
}
