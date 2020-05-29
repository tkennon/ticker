package ticker

import (
	"time"
)

type exponential struct {
	current    time.Duration
	multiplier float32
}

func (e *exponential) next() time.Duration {
	current := e.current
	c := float32(e.current) * e.multiplier
	if e.multiplier > 0 && c > 0 || e.multiplier < 0 && c < 0 {
		// Only update if there has been no numerical overflow.
		e.current = time.Duration(c)
	}
	return current
}

// NewExponential returns an exponential backoff ticker.
func NewExponential(initial time.Duration, multiplier float32) *Ticker {
	return newTicker(&exponential{
		current:    initial,
		multiplier: multiplier,
	})
}
