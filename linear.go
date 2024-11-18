package ticker

import "time"

type linear struct {
	current   time.Duration
	increment time.Duration
}

func (l *linear) Next() time.Duration {
	current := l.current
	l.current += l.increment
	return current
}

// NewLinear returns a linear backoff ticker.
func NewLinear(initial, increment time.Duration) *Ticker {
	return NewTicker(&linear{
		current:   initial,
		increment: increment,
	})
}
