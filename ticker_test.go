package ticker

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clock is a type that is stubbed out for timeAfter so that we can easily
// test the ticker package.
type clock struct {
	report bool
	next   chan time.Duration
}

// newClock returns a clock object which synchronously writes the value that the
// ticker requested to sleep for to the clock's next channel. Users must read
// this channel to prevent deadlock.
func newClock() *clock {
	return &clock{
		report: true,
		next:   make(chan time.Duration),
	}
}

// newIgnoredClock returns a clock object that immediately returns the current
// time. It does not write the sleep duration to the the next channel. It should
// be used by tests that don't want to do a real sleep, but also don't care
// about the value of the sleep request.
func newIgnoredClock() *clock {
	return &clock{}
}

// After is a stub of time.After. It record the requested sleep duration and
// immediately returns the current time.
func (c *clock) After(next time.Duration) <-chan time.Time {
	if c.report {
		c.next <- next
	}
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

func waitForStopped(t *testing.T, ticker *Ticker, fakeClock *clock) time.Duration {
	var total time.Duration
	for {
		select {
		case next := <-fakeClock.next:
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
	fakeClock := newClock()
	constant := NewConstant(time.Second)
	constant.timeAfterFn = fakeClock.After
	err := constant.Start()
	require.NoError(t, err)
	defer constant.Stop()

	for i := 0; i < 10; i++ {
		next := <-fakeClock.next
		t.Log(next)
		assert.Equal(t, time.Second, next, "iteration %d", i)
		assert.NotEmpty(t, <-constant.C)
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
			fakeClock := newClock()
			lin := NewLinear(tt.initial, tt.increment)
			lin.timeAfterFn = fakeClock.After
			err := lin.Start()
			require.NoError(t, err)
			defer lin.Stop()
			for i := 0; i < 10; i++ {
				next := <-fakeClock.next
				t.Log(next)
				assert.Equal(t, tt.initial+time.Duration(i)*tt.increment, next, "iteration %d", i)
				assert.NotEmpty(t, <-lin.C)
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
			fakeClock := newClock()
			exp := NewExponential(tt.initial, tt.multiplier)
			exp.timeAfterFn = fakeClock.After
			err := exp.Start()
			require.NoError(t, err)
			defer exp.Stop()

			for i := 0; i < 10; i++ {
				next := <-fakeClock.next
				t.Log(next)
				expected := float64(tt.initial) * math.Pow(float64(tt.multiplier), float64(i))
				actual := float64(next)
				tolerance := float64(tt.initial) * 0.01
				assert.InDelta(t, expected, actual, tolerance, "iteration %d", i)
				assert.NotEmpty(t, <-exp.C)
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
			fakeClock := newClock()
			ticker := tt.newTicker()
			ticker.timeAfterFn = fakeClock.After
			err := ticker.Start()
			require.NoError(t, err)
			defer ticker.Stop()

			// Jittered version.
			fakeClockJitter := newClock()
			tickerJitter := tt.newTicker().WithJitter(0.1)
			tickerJitter.timeAfterFn = fakeClockJitter.After
			err = tickerJitter.Start()
			require.NoError(t, err)
			defer tickerJitter.Stop()

			for i := 0; i < 10; i++ {
				// Check that the jittered ticker is within +/-10% of un
				// jittered ticker.
				next := <-fakeClock.next
				nextJitter := <-fakeClockJitter.next
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
			fakeClock := newClock()
			ticker := tt.ticker.WithMaxInterval(tt.max)
			ticker.timeAfterFn = fakeClock.After
			err := ticker.Start()
			require.NoError(t, err)
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				next := <-fakeClock.next
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
			fakeClock := newClock()
			ticker := tt.ticker.WithMinInterval(tt.min)
			ticker.timeAfterFn = fakeClock.After
			err := ticker.Start()
			require.NoError(t, err)
			defer ticker.Stop()

			for i := 0; i < 10; i++ {
				next := <-fakeClock.next
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
			fakeClock := newClock()
			ticker := tt.ticker.WithMaxDuration(tt.max)
			ticker.timeAfterFn = fakeClock.After
			err := ticker.Start()
			require.NoError(t, err)
			defer ticker.Stop()

			total := waitForStopped(t, ticker, fakeClock)
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
			fakeClock := newIgnoredClock()
			ctx, cancel := context.WithCancel(context.Background())
			ticker := tt.ticker.WithContext(ctx)
			ticker.timeAfterFn = fakeClock.After

			err := ticker.Start()
			require.NoError(t, err)
			defer ticker.Stop()
			assert.NotEmpty(t, <-ticker.C)

			cancel()
			// No guarantee about when the ticker will stop. Wait until the
			// channel is closed.
			waitForStopped(t, ticker, fakeClock)

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
			fakeClock := newIgnoredClock()
			ticker := tt.ticker.WithFunc(func() { close(done) })
			ticker.timeAfterFn = fakeClock.After
			err := ticker.Start()
			require.NoError(t, err)
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
			fakeClock := newIgnoredClock()
			tt.ticker.timeAfterFn = fakeClock.After

			err := tt.ticker.Start()
			require.NoError(t, err)
			tt.ticker.Stop()
			// There are no guarantees about when the ticker will stop, only
			// that it will stop eventually. As such, loop until the channel
			// closes.
			waitForStopped(t, tt.ticker, fakeClock)
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
