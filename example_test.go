package ticker_test

import (
	"context"
	"fmt"
	"time"

	"github.com/tkennon/ticker"
)

func runFiveTimes(t *ticker.Ticker) bool {
	for i := 0; i < 5; i++ {
		then := time.Now()
		now, ok := <-t.C
		if !ok {
			return false
		}
		fmt.Println(now.Sub(then).Round(time.Millisecond))
	}

	return true
}

func ExampleNewConstant() {
	con := ticker.NewConstant(time.Millisecond)
	if err := con.Start(); err != nil {
		panic(err)
	}
	defer con.Stop()
	if ok := runFiveTimes(con); !ok {
		panic("ticker stopped unexpectedly")
	}

	// Output:
	// 1ms
	// 1ms
	// 1ms
	// 1ms
	// 1ms
}

func ExampleNewLinear() {
	lin := ticker.NewLinear(time.Millisecond, time.Millisecond)
	if err := lin.Start(); err != nil {
		panic(err)
	}
	defer lin.Stop()
	if ok := runFiveTimes(lin); !ok {
		panic("ticker stopped unexpectedly")
	}

	// Output:
	// 1ms
	// 2ms
	// 3ms
	// 4ms
	// 5ms
}

func ExampleNewExponential() {
	exp := ticker.NewExponential(time.Millisecond, 2.0)
	if err := exp.Start(); err != nil {
		panic(err)
	}
	defer exp.Stop()
	if ok := runFiveTimes(exp); !ok {
		panic("ticker stopped unexpectedly")
	}

	// Output:
	// 1ms
	// 2ms
	// 4ms
	// 8ms
	// 16ms
}

func ExampleTicker_WithMinInterval() {
	lin := ticker.NewLinear(5*time.Millisecond, -time.Millisecond).WithMinInterval(3 * time.Millisecond)
	if err := lin.Start(); err != nil {
		panic(err)
	}
	defer lin.Stop()
	if ok := runFiveTimes(lin); !ok {
		panic("ticker stopped unexpectedly")
	}

	// Output:
	// 5ms
	// 4ms
	// 3ms
	// 3ms
	// 3ms
}

func ExampleTicker_WithMaxInterval() {
	lin := ticker.NewLinear(time.Millisecond, time.Millisecond).WithMaxInterval(3 * time.Millisecond)
	if err := lin.Start(); err != nil {
		panic(err)
	}
	defer lin.Stop()
	if ok := runFiveTimes(lin); !ok {
		panic("ticker stopped unexpectedly")
	}

	// Output:
	// 1ms
	// 2ms
	// 3ms
	// 3ms
	// 3ms
}

func ExampleTicker_WithMaxDuration() {
	exp := ticker.NewExponential(time.Millisecond, 2.0).WithMaxDuration(10 * time.Millisecond)
	if err := exp.Start(); err != nil {
		panic(err)
	}
	ok := runFiveTimes(exp)
	fmt.Println("ticker channel closed:", !ok)

	// Output:
	// 1ms
	// 2ms
	// 4ms
	// ticker channel closed: true
}

func ExampleTicker_WithContext() {
	ctx, cancel := context.WithCancel(context.Background())
	con := ticker.NewConstant(time.Millisecond).WithContext(ctx)
	then := time.Now()
	if err := con.Start(); err != nil {
		panic(err)
	}
	now, ok := <-con.C
	fmt.Println("ticker ok:", ok)
	fmt.Println(now.Sub(then).Round(time.Millisecond))
	cancel()
	now, ok = <-con.C
	fmt.Println("ticker ok:", ok)
	fmt.Println("ticker time is zero:", now.IsZero())

	// Output:
	// ticker ok: true
	// 1ms
	// ticker ok: false
	// ticker time is zero: true
}

func ExampleTicker_WithFunc() {
	str := make(chan string)
	con := ticker.NewConstant(time.Millisecond).WithFunc(func() {
		str <- "hello"
	})
	if err := con.Start(); err != nil {
		panic(err)
	}
	if ok := runFiveTimes(con); !ok {
		panic("ticker stopped unexpectedly")
	}

	for i := 0; i < 5; i++ {
		fmt.Println(<-str)
	}

	// Output:
	// 1ms
	// 1ms
	// 1ms
	// 1ms
	// 1ms
	// hello
	// hello
	// hello
	// hello
	// hello
}

func ExampleTicker_Stop() {
	con := ticker.NewConstant(time.Millisecond)
	con.Stop()
	if err := con.Start(); err != nil {
		panic(err)
	}
	now, ok := <-con.C
	fmt.Println("ticker ok:", ok)
	fmt.Println("ticker time is zero:", now.IsZero())

	// Output:
	// ticker ok: false
	// ticker time is zero: true
}
