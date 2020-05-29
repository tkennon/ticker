package ticker

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sleeper is a type that implements an After() function that can stub out a
// ticker's timeAfterFn.
type sleeper struct {
	next chan time.Duration
}

// After is a stub of time.After. It record the requested sleep duration and
// immediately returns the current time.
func (s *sleeper) After(next time.Duration) <-chan time.Time {
	if s.next != nil {
		s.next <- next
	}
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

// newSleeperForTicker takes a ticker and replaces its timeAfterFn with a fake
// sleeper that will return immediately and reports the requested sleep duration
// on the returned channel. It should be used by tests that don't want to do a
// real sleep, but want to inspect the duration of the sleep requested by the
// ticker.
func newSleeperForTicker(ticker *Ticker) (*Ticker, <-chan time.Duration) {
	fc := &sleeper{next: make(chan time.Duration)}
	ticker.timeAfterFn = fc.After
	return ticker, fc.next
}

// newIgnoredSleeperForTicker takes a ticker and replaces its timeAfterFn with a
// fake sleeper that returns immediately. It should be used by tests that don't
// want to do a real sleep, but also don't care about the duration of the sleep
// requested by the ticker.
func newIgnoredSleeperForTicker(ticker *Ticker) *Ticker {
	fc := &sleeper{}
	ticker.timeAfterFn = fc.After
	return ticker
}

// waitForStopped reads from the both the fake sleeper's next channel and the
// tickers channel until the ticker's channel is closed. It returns the total
// amount of time that the fake sleeper slept for.
func waitForStopped(t *testing.T, ticker *Ticker, nextSleep <-chan time.Duration) time.Duration {
	var total time.Duration
	for {
		select {
		case next := <-nextSleep:
			t.Log(next)
			total += next
		case now, ok := <-ticker.C:
			if !ok {
				assert.Empty(t, now)
				return total
			}
			assert.NotEmpty(t, now)
		}
	}
}

func TestConstant(t *testing.T) {
	ticker, nextSleep := newSleeperForTicker(NewConstant(time.Second))
	require.NoError(t, ticker.Start())
	defer ticker.Stop()

	for i := 0; i < 10; i++ {
		next := <-nextSleep
		t.Log(next)
		assert.Equal(t, time.Second, next, "iteration %d", i)
		assert.NotEmpty(t, <-ticker.C)
	}
}

func TestLinear(t *testing.T) {
	tests := []struct {
		name      string
		initial   time.Duration
		increment time.Duration
	}{
		{
			name:      "increase",
			initial:   time.Second,
			increment: time.Second,
		},
		{
			name:      "decrease",
			initial:   time.Minute,
			increment: -time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, nextSleep := newSleeperForTicker(NewLinear(tt.initial, tt.increment))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				next := <-nextSleep
				t.Log(next)
				assert.Equal(t, tt.initial+time.Duration(i)*tt.increment, next, "iteration %d", i)
				assert.NotEmpty(t, <-ticker.C)
			}
		})
	}
}

func TestExponential(t *testing.T) {
	tests := []struct {
		name       string
		initial    time.Duration
		multiplier float32
	}{
		{
			name:       "increase",
			initial:    time.Second,
			multiplier: 1.1,
		},
		{
			name:       "decrease",
			initial:    time.Minute,
			multiplier: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, nextSleep := newSleeperForTicker(NewExponential(tt.initial, tt.multiplier))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				next := <-nextSleep
				t.Log(next)
				expected := float64(tt.initial) * math.Pow(float64(tt.multiplier), float64(i))
				actual := float64(next)
				tolerance := float64(tt.initial) * 0.01
				assert.InDelta(t, expected, actual, tolerance, "iteration %d", i)
				assert.NotEmpty(t, <-ticker.C)
			}
		})
	}
}

func TestWithJitter(t *testing.T) {
	tests := []struct {
		name      string
		newTicker func() *Ticker
	}{
		{
			name:      "constant",
			newTicker: func() *Ticker { return NewConstant(time.Second) },
		},
		{
			name:      "linear",
			newTicker: func() *Ticker { return NewLinear(time.Second, time.Second) },
		},
		{
			name:      "exponential",
			newTicker: func() *Ticker { return NewExponential(time.Second, 2.0) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unjittered version.
			ticker, nextSleep := newSleeperForTicker(tt.newTicker())
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			// Jittered version.
			tickerJitter, nextJitterSleep := newSleeperForTicker(tt.newTicker().WithJitter(0.1))
			require.NoError(t, tickerJitter.Start())
			defer tickerJitter.Stop()

			for i := 0; i < 10; i++ {
				// Check that the jittered ticker is within +/-10% of unjittered
				// ticker.
				next := <-nextSleep
				nextJitter := <-nextJitterSleep
				t.Log(nextJitter)
				assert.NotEqual(t, next, nextJitter)
				assert.LessOrEqual(t, 0.9*float64(next), float64(nextJitter))
				assert.GreaterOrEqual(t, 1.1*float64(next), float64(nextJitter))
				assert.NotEmpty(t, <-ticker.C)
				assert.NotEmpty(t, <-tickerJitter.C)
			}
		})
	}
}

func TestWithMaxInterval(t *testing.T) {
	tests := []struct {
		name   string
		max    time.Duration
		ticker *Ticker
	}{
		{
			name:   "constant",
			max:    time.Second,
			ticker: NewConstant(time.Minute),
		},
		{
			name:   "linear",
			max:    5 * time.Second,
			ticker: NewLinear(time.Second, time.Second),
		},
		{
			name:   "exponential",
			max:    time.Minute,
			ticker: NewExponential(time.Second, 3),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, nextSleep := newSleeperForTicker(tt.ticker.WithMaxInterval(tt.max))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				next := <-nextSleep
				t.Log(next)
				assert.LessOrEqual(t, next.Nanoseconds(), tt.max.Nanoseconds())
				assert.NotEmpty(t, <-ticker.C)
			}
		})
	}
}

func TestWithMinInterval(t *testing.T) {
	tests := []struct {
		name   string
		min    time.Duration
		ticker *Ticker
	}{
		{
			name:   "constant",
			min:    time.Second,
			ticker: NewConstant(time.Millisecond),
		},
		{
			name:   "linear",
			min:    5 * time.Second,
			ticker: NewLinear(10*time.Second, -time.Second),
		},
		{
			name:   "exponential",
			min:    time.Second,
			ticker: NewExponential(time.Minute, 0.5),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, nextSleep := newSleeperForTicker(tt.ticker.WithMinInterval(tt.min))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				next := <-nextSleep
				t.Log(next)
				assert.GreaterOrEqual(t, next.Nanoseconds(), tt.min.Nanoseconds())
				assert.NotEmpty(t, <-ticker.C)
			}
		})
	}
}

func TestWithMaxDuration(t *testing.T) {
	tests := []struct {
		name   string
		max    time.Duration
		ticker *Ticker
	}{
		{
			name:   "constant",
			max:    time.Minute,
			ticker: NewConstant(10 * time.Second),
		},
		{
			name:   "linear",
			max:    time.Minute,
			ticker: NewLinear(time.Second, time.Second)},
		{
			name:   "exponential",
			max:    time.Minute,
			ticker: NewExponential(time.Second, 2.0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, nextSleep := newSleeperForTicker(tt.ticker.WithMaxDuration(tt.max))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			total := waitForStopped(t, ticker, nextSleep)
			require.Empty(t, <-ticker.C)
			require.LessOrEqual(t, total.Nanoseconds(), tt.max.Nanoseconds())
		})
	}
}

func TestWithContext(t *testing.T) {
	tests := []struct {
		name   string
		ticker *Ticker
	}{
		{
			name:   "constant",
			ticker: NewConstant(time.Second),
		},
		{
			name:   "linear",
			ticker: NewLinear(time.Second, time.Second),
		},
		{
			name:   "exponential",
			ticker: NewExponential(time.Second, 2.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			ticker, nextSleep := newSleeperForTicker(tt.ticker.WithContext(ctx))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()

			cancel()
			// No guarantee about when the ticker will stop. Wait until the
			// channel is closed.
			waitForStopped(t, ticker, nextSleep)

			now, ok := <-ticker.C
			assert.False(t, ok)
			assert.Empty(t, now)
		})
	}
}

func TestWithFunc(t *testing.T) {
	tests := []struct {
		name   string
		ticker *Ticker
	}{
		{
			name:   "constant",
			ticker: NewConstant(time.Second),
		},
		{
			name:   "linear",
			ticker: NewLinear(time.Second, time.Second),
		},
		{
			name:   "exponential",
			ticker: NewExponential(time.Second, 2.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan struct{})
			ticker := newIgnoredSleeperForTicker(tt.ticker.WithFunc(func() { close(done) }))
			require.NoError(t, ticker.Start())
			defer ticker.Stop()
			assert.NotEmpty(t, <-ticker.C)
			<-done
		})
	}
}

func TestStop(t *testing.T) {
	tests := []struct {
		name   string
		ticker *Ticker
	}{
		{
			name:   "constant",
			ticker: NewConstant(time.Second),
		},
		{
			name:   "linear",
			ticker: NewLinear(time.Second, time.Second),
		},
		{
			name:   "exponential",
			ticker: NewExponential(time.Second, 2.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ticker, nextSleep := newSleeperForTicker(tt.ticker)
			require.NoError(t, ticker.Start())
			tt.ticker.Stop()

			// There are no guarantees about when the ticker will stop, only
			// that it will stop eventually. As such, loop until the channel
			// closes.
			waitForStopped(t, ticker, nextSleep)
			now, ok := <-tt.ticker.C
			assert.False(t, ok)
			assert.Empty(t, now)
		})
	}
}

func TestInvalidSettings(t *testing.T) {
	con := NewConstant(time.Minute).
		WithMaxInterval(time.Second).
		WithMinInterval(time.Hour)

	err := con.Start()
	require.Error(t, err)

	con = NewConstant(time.Minute).
		WithMaxDuration(time.Second).
		WithMinInterval(time.Hour)

	err = con.Start()
	require.Error(t, err)
}
