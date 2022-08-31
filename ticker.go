package ticker

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// Functions stubbed out during testing.
var (
	// magnitude returns a uniformly random number in the range [-1.0, 1.0).
	magnitude = func() float64 { return 1.0 - 2.0*rand.Float64() }
)

var (
	// errInvalidSettings is returned from (*Ticker).Start() when invalid ticker
	// settings are detected; such as if the minimum duration had been set to
	// greater than the maximum duration.
	errInvalidSettings = errors.New("invalid ticker settings")
)

type interval interface {
	next() time.Duration
}

// Ticker holds a channel that delivers `ticks` of a clock at intervals (just
// like the standard library `time.Ticker`).
type Ticker struct {
	ctx            context.Context
	interval       interval
	total          time.Duration
	jitter         float64
	minIntervalSet bool
	minInterval    time.Duration
	maxIntervalSet bool
	maxInterval    time.Duration
	maxDurationSet bool
	maxDuration    time.Duration
	stop           chan struct{}
	f              func()
	C              chan time.Time

	// Stubbed out during testing.
	timeAfterFn func(time.Duration) <-chan time.Time
}

func newTicker(interval interval) *Ticker {
	return &Ticker{
		ctx:         context.Background(),
		timeAfterFn: time.After,
		interval:    interval,
		stop:        make(chan struct{}),
		C:           make(chan time.Time),
	}
}

// WithJitter adds a uniformly random jitter to the time the ticker next fires.
// The jitter will be within `fraction` of the next interval. For example,
// WithJitter(0.1) applies a 10% jitter, so a linear ticker that would otherwise
// fire after 1, 2, 3, 4, ... second would now fire between 0.9-1.1 seconds on
// the first tick, and 1.8-2.2 seconds on the second tick, and so forth. The
// jitter fraction may be greater than one, allowing the possiblity for jittered
// tickers to fire immediately if the calculated interval with the jitter is
// less than zero.
func (t *Ticker) WithJitter(fraction float64) *Ticker {
	t.jitter = fraction
	return t
}

// WithMinInterval sets the minimum interval between times the ticker fires.
// This is applied after jitter is applied.
func (t *Ticker) WithMinInterval(d time.Duration) *Ticker {
	t.minInterval = d
	t.minIntervalSet = true
	return t
}

// WithMaxInterval sets the maximum interval between times the ticker fires.
// This is applied after jitter is applied.
func (t *Ticker) WithMaxInterval(d time.Duration) *Ticker {
	t.maxInterval = d
	t.maxIntervalSet = true
	return t
}

// WithMaxDuration sets the maxiumum total duration over all ticks that the
// ticker will run for. Once the maximum duration is reached, the ticker's
// channel will be closed.
func (t *Ticker) WithMaxDuration(d time.Duration) *Ticker {
	t.maxDuration = d
	t.maxDurationSet = true
	return t
}

// WithContext adds a context.Context to the ticker. If the context expires then
// the ticker's channel will be closed.
func (t *Ticker) WithContext(ctx context.Context) *Ticker {
	t.ctx = ctx
	return t
}

// WithFunc will execute f in its own goroutine each time the ticker fires. The
// ticker must be stopped to prevent running f.
func (t *Ticker) WithFunc(f func()) *Ticker {
	t.f = f
	return t
}

// Start starts the ticker. The ticker's channel will send the current time
// after an interval has elapsed. Start returns an error if the ticker could not
// be started due to restrictions imposed in the ticker config (e.g. minimum
// allowed interval is greater than the maximum allowed interval). The ticker's
// channel will fire at different intervals determined by the type of ticker
// (e.g. constant, linear, or exponential) and by the ticker modifiers
// (WithJitter, WithMaximum etc). The ticker must be stopped (either explicitly
// via Stop, or through a context expiring or a maximum duration being reached)
// to release all associated resources. The ticker's channel will be closed when
// the ticker is stopped . As such, callers must always check the ticker's
// channel is still open when receiving "ticks". The ticker will not fire again
// until its channel has been read.
//
// Note that once started, tickers must not be modified (through the WithXxx
// modifiers).
func (t *Ticker) Start() error {
	// Sanity check the min/max intervals.
	if t.minIntervalSet {
		if t.maxIntervalSet && t.minInterval > t.maxInterval {
			return errInvalidSettings
		}
		if t.maxDurationSet && t.minInterval > t.maxDuration {
			return errInvalidSettings
		}
	}

	// Asynchronously wait for the ticker to expire.
	go func() {
		// On exit, close and drain the ticker channel.
		defer func() {
			close(t.C)
			for range t.C {
			}
		}()

		for {
			// Get the next interval.
			next := t.interval.next()

			// Add jitter.
			next += time.Duration(t.jitter * magnitude() * float64(next))

			// Floor a single interval.
			if t.minIntervalSet && next < t.minInterval {
				next = t.minInterval
			}

			// Cap a single interval.
			if t.maxIntervalSet && next > t.maxInterval {
				next = t.maxInterval
			}

			// Cap the sum of all intervals.
			if t.maxDurationSet && t.total > t.maxDuration-next {
				return
			}
			t.total += next

			// Either exit or fire the ticker.
			select {
			case <-t.ctx.Done():
				return
			case <-t.stop:
				return
			case now := <-t.timeAfterFn(next):
				t.C <- now
				if t.f != nil {
					go t.f()
				}
			}
		}
	}()

	return nil
}

// Stop turns off a ticker. Unlike the standard library, Stop does close the
// ticker's channel. As such, callers must always check the ticker's channel is
// still open when receiving "ticks". The ticker may continue to return valid
// ticks for a brief period after Stop is called.
//
// Once stopped, a ticker will panic if (re)started. Stop will panic if called
// more than once. It is safe to stop a ticker via different methods (e.g.
// calling Stop explicitly, canceling a context, or capping the total maximum
// duration the ticker has ticked for).
func (t *Ticker) Stop() {
	close(t.stop)
}
