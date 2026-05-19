package main

import (
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestWatcher_FiresOnceAfterGrace(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	fired := make(chan string, 4)

	w := NewWatcher(WatcherOpts{
		Grace:    10 * time.Second,
		Interval: 5 * time.Millisecond,
		Now:      clk.Now,
		Shutdown: func(reason string) { fired <- reason },
	})

	stop := make(chan struct{})
	defer close(stop)
	go w.Run(stop)

	time.Sleep(20 * time.Millisecond)
	select {
	case r := <-fired:
		t.Fatalf("fired prematurely: %s", r)
	default:
	}

	clk.Advance(11 * time.Second)

	select {
	case r := <-fired:
		if r != "idle" {
			t.Errorf("reason = %q, want %q", r, "idle")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("watcher did not fire after grace expired")
	}

	time.Sleep(20 * time.Millisecond)
	if got := len(fired); got != 0 {
		t.Errorf("extra fires: %d (want 0)", got)
	}
}

func TestWatcher_BeatPreventsIdle(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	fired := make(chan string, 4)

	w := NewWatcher(WatcherOpts{
		Grace:    10 * time.Second,
		Interval: 5 * time.Millisecond,
		Now:      clk.Now,
		Shutdown: func(reason string) { fired <- reason },
	})

	stop := make(chan struct{})
	defer close(stop)
	go w.Run(stop)

	for i := 0; i < 5; i++ {
		clk.Advance(8 * time.Second)
		w.Beat()
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case r := <-fired:
		t.Fatalf("fired despite beats: %s", r)
	default:
	}
}

func TestWatcher_ByeFiresImmediately(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	fired := make(chan string, 4)

	w := NewWatcher(WatcherOpts{
		Grace:    1 * time.Hour,
		Interval: 5 * time.Millisecond,
		Now:      clk.Now,
		Shutdown: func(reason string) { fired <- reason },
	})

	stop := make(chan struct{})
	defer close(stop)
	go w.Run(stop)

	w.Bye()

	select {
	case r := <-fired:
		if r != "browser closed" {
			t.Errorf("reason = %q, want %q", r, "browser closed")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("watcher did not fire on Bye")
	}
}

func TestWatcher_RunReturnsOnStop(t *testing.T) {
	clk := &fakeClock{t: time.Unix(0, 0)}
	w := NewWatcher(WatcherOpts{
		Grace:    1 * time.Hour,
		Interval: 5 * time.Millisecond,
		Now:      clk.Now,
		Shutdown: func(_ string) {},
	})

	stop := make(chan struct{})
	runDone := make(chan struct{})
	go func() {
		w.Run(stop)
		close(runDone)
	}()

	close(stop)
	select {
	case <-runDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Run did not return on stop")
	}
}
