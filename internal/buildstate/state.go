// Package buildstate holds the GUI's "is a build currently running" state,
// shared between the button that starts it, the goroutine running it, and
// the Cancel button/quit path that can interrupt it.
//
// Per this project's own CLAUDE.md ("never fire a callback while holding a
// mutex"), BuildState never calls back into the UI itself — every method
// here only computes/mutates under its lock and returns. The caller (in
// package main, which can import fyne) does its own fyne.Do afterward, on
// its own goroutine, outside this lock entirely. That split is deliberate:
// it makes the self-deadlock CLAUDE.md warns about structurally impossible
// here, rather than something to remember to avoid.
package buildstate

import (
	"context"
	"sync"
)

// State tracks one build run at a time.
type State struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	log     []string
}

// Start begins a run, returning a cancelable context for it. ok is false if
// a build is already running — the caller should refuse to start a second
// one concurrently rather than silently interleaving two runs' output.
func (s *State) Start(parent context.Context) (ctx context.Context, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil, false
	}
	ctx, cancel := context.WithCancel(parent)
	s.running = true
	s.cancel = cancel
	s.log = s.log[:0]
	return ctx, true
}

// Finish marks the run as no longer active. Safe to call even if Start was
// never called (e.g. Resolve failed before any build began).
func (s *State) Finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running = false
	s.cancel = nil
}

// Cancel interrupts the running build's context, if one is running.
// Reports whether there was one to cancel.
func (s *State) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running || s.cancel == nil {
		return false
	}
	s.cancel()
	return true
}

// IsRunning reports whether a build is currently in progress.
func (s *State) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// AppendLog records one progress line.
func (s *State) AppendLog(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = append(s.log, line)
}

// LogSnapshot returns a copy of the log so far — safe for the caller to hand
// straight to a UI update without racing further appends.
func (s *State) LogSnapshot() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.log...)
}
