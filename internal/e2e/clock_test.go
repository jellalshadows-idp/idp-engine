package e2e

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time        { return c.now }
func (c *fakeClock) Sleep(d time.Duration) { c.now = c.now.Add(d) }

func TestPoll(t *testing.T) {
	c := &fakeClock{}
	calls := 0
	err := poll(context.Background(), c, time.Second, time.Minute, "x", func() (bool, error) { calls++; return calls == 3, nil })
	if err != nil || calls != 3 {
		t.Errorf("poll = %v after %d calls", err, calls)
	}
	err = poll(context.Background(), c, 10*time.Second, 30*time.Second, "the thing", func() (bool, error) { return false, nil })
	if err == nil || !strings.Contains(err.Error(), "timed out after 30s waiting for the thing") {
		t.Errorf("timeout err = %v", err)
	}
	boom := errors.New("boom")
	if err := poll(context.Background(), c, time.Second, time.Minute, "x", func() (bool, error) { return false, boom }); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}
