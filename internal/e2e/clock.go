package e2e

import (
	"context"
	"fmt"
	"time"
)

// Clock is the harness's notion of time, so tests never sleep.
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

// poll calls check every interval until it reports done or fails, or until
// timeout passes; what names the wait in the timeout error.
func poll(ctx context.Context, c Clock, interval, timeout time.Duration, what string, check func() (bool, error)) error {
	deadline := c.Now().Add(timeout)
	for {
		done, err := check()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !c.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s", timeout, what)
		}
		c.Sleep(interval)
	}
}
