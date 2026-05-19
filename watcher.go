package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// Watcher implements the "shut down when the browser is gone" lifecycle:
// the frontend pings Beat() on an interval; Run() shuts the process down
// if no beat arrives within the grace window, and Bye() lets the browser
// trigger an immediate shutdown on pagehide.
type Watcher struct {
	grace    time.Duration
	interval time.Duration
	now      func() time.Time
	shutdown func(reason string)

	mu       sync.Mutex
	lastBeat time.Time
	fired    atomic.Bool
}

type WatcherOpts struct {
	Grace    time.Duration
	Interval time.Duration
	Now      func() time.Time
	Shutdown func(reason string)
}

func NewWatcher(opts WatcherOpts) *Watcher {
	if opts.Grace <= 0 {
		opts.Grace = 30 * time.Second
	}
	if opts.Interval <= 0 {
		opts.Interval = 2 * time.Second
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now() }
	}
	w := &Watcher{
		grace:    opts.Grace,
		interval: opts.Interval,
		now:      opts.Now,
		shutdown: opts.Shutdown,
	}
	w.lastBeat = w.now()
	return w
}

func (w *Watcher) Beat() {
	w.mu.Lock()
	w.lastBeat = w.now()
	w.mu.Unlock()
}

func (w *Watcher) Bye() {
	w.fire("browser closed")
}

func (w *Watcher) Run(stop <-chan struct{}) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			w.mu.Lock()
			stale := w.now().Sub(w.lastBeat) > w.grace
			w.mu.Unlock()
			if stale {
				w.fire("idle")
				return
			}
		}
	}
}

func (w *Watcher) fire(reason string) {
	if !w.fired.CompareAndSwap(false, true) {
		return
	}
	if w.shutdown != nil {
		go w.shutdown(reason)
	}
}
