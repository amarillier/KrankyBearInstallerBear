package buildstate

import (
	"context"
	"testing"
)

func TestState_StartRefusesConcurrentRun(t *testing.T) {
	var s State
	ctx1, ok := s.Start(context.Background())
	if !ok || ctx1 == nil {
		t.Fatalf("first Start should succeed, got ok=%v ctx=%v", ok, ctx1)
	}
	if _, ok := s.Start(context.Background()); ok {
		t.Error("second concurrent Start should be refused")
	}
	if !s.IsRunning() {
		t.Error("IsRunning should be true while a build is in progress")
	}

	s.Finish()
	if s.IsRunning() {
		t.Error("IsRunning should be false after Finish")
	}
	if _, ok := s.Start(context.Background()); !ok {
		t.Error("Start should succeed again after Finish")
	}
}

func TestState_CancelInterruptsContext(t *testing.T) {
	var s State
	ctx, ok := s.Start(context.Background())
	if !ok {
		t.Fatal("Start failed")
	}

	if !s.Cancel() {
		t.Error("Cancel should report true when a build is running")
	}
	select {
	case <-ctx.Done():
	default:
		t.Error("the context returned by Start should be canceled after Cancel()")
	}
}

func TestState_CancelWithNoRunReportsFalse(t *testing.T) {
	var s State
	if s.Cancel() {
		t.Error("Cancel should report false when nothing is running")
	}
}

func TestState_LogSnapshotIsIndependentCopy(t *testing.T) {
	var s State
	if _, ok := s.Start(context.Background()); !ok {
		t.Fatal("Start failed")
	}
	s.AppendLog("first")
	snap := s.LogSnapshot()
	s.AppendLog("second")

	if len(snap) != 1 || snap[0] != "first" {
		t.Fatalf("snapshot = %v, want [\"first\"] (unaffected by the later append)", snap)
	}
	if full := s.LogSnapshot(); len(full) != 2 {
		t.Fatalf("expected 2 log lines after the second append, got %v", full)
	}
}
