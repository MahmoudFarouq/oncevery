package once

import (
	"context"
	"sync"
	"time"
)

const never = 1 * 365 * 24 * time.Hour

type Options struct {
	every        time.Duration
	retryOnError bool
}

type Option func(*Options)

func OptionSetEvery(duration time.Duration) Option {
	return func(options *Options) {
		options.every = duration
	}
}

func OptionRetryOnError(retry bool) Option {
	return func(options *Options) {
		options.retryOnError = retry
	}
}

func New[T any](fn func(context.Context) (T, error), ops ...Option) *Once[T] {
	options := Options{
		every:        never,
		retryOnError: false,
	}

	for _, op := range ops {
		op(&options)
	}

	return &Once[T]{fn: fn, options: options}
}

type Once[T any] struct {
	options Options
	fn      func(context.Context) (T, error)

	mu         sync.RWMutex
	lastCalled time.Time
	result     T
	err        error
}

func (o *Once[T]) Pull(ctx context.Context) (T, error) {
	o.mu.RLock()

	checks := []check{
		checkIsOldEnough(o.lastCalled, o.options.every),
		checkShouldRetryOnError(o.err, o.options.retryOnError),
	}
	if all(checks, false) {
		o.mu.RUnlock()

		return o.result, o.err
	}

	o.mu.RUnlock()

	// At this point, there is a possibility that we have multiple callers
	// waiting here to take a WriteLock. Since we need to make sure we never
	// call fn more than needed, If we use `o.mu.Lock()`, all waiting
	// callers will call the function which isn't the desired behaviour. We
	// instead let the first caller take the WriteLock, then all others go
	// back and wait for a ReadLock instead of taking the WriteLock next and
	// doing the same processing we just did.
	if ok := o.mu.TryLock(); !ok {
		return o.Pull(ctx)
	}

	defer o.mu.Unlock()

	o.result, o.err = o.fn(ctx)
	o.lastCalled = time.Now()

	return o.result, o.err
}

type check func() bool

func checkIsOldEnough(lastCalled time.Time, every time.Duration) check {
	return func() bool {
		return time.Now().After(lastCalled.Add(every))
	}
}

// Returns false if we shouldn't retry on error
func checkShouldRetryOnError(lastErr error, retry bool) check {
	return func() bool {
		return retry && lastErr != nil
	}
}

func all(checks []check, are bool) bool {
	for _, c := range checks {
		if c() != are {
			return false
		}
	}

	return true
}
