package ticker

import "time"

type constant time.Duration

func (c constant) next() time.Duration {
	return time.Duration(c)
}

// NewConstant returns a constant ticker, functionally equivalent to the standard
// library time.Ticker.
func NewConstant(interval time.Duration) *Ticker {
	return newTicker(constant(interval))
}
