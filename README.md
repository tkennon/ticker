# ticker

[![GoDoc](https://godoc.org/github.com/tkennon/ticker?status.svg)](https://godoc.org/github.com/tkennon/ticker)

Ticker utilities for Golang - extensions of the standard library's time.Ticker.
Create constant, linear, or exponential tickers. Optionally configure random
jitter, minimum or maximum intervals, and maximum cumulative duration. See the
`examples_test.go` for comprehensive usage.

# Install

`go get github.com/tkennon/ticker`

# Usage

Create tickers that fire at linearly or exponentially increasing intervals. A
constant ticker is also included that is functionally equivalent to a
`time.Ticker`. Once started, tickers _must_ be stopped in order to clean up
resources.

```
t := ticker.NewExponential(time.Second, 2.0). // Create a ticker starting at one second whose intervals double each time it fires.
    WithMaxInterval(time.Hour)                // Optionally cap the maximum interval to an hour.
    WithJitter(0.1)                           // Optionally add a +/-10% jitter to each interval.

if err := t.Start(); err != nil {
    return err
}
defer t.Stop()

for range t.C {
    // Do something each time the ticker fires. Note that because we have not
    // added a context to the ticker, and we have not specified a maximum total
    // duration through WithMaxDuration(), the ticker will tick forever.
}
```

# API Stability

`v1.0.0` is not yet tagged, but the package is considered stable.