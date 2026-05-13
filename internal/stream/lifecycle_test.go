package stream

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSpawnCleanFinalizeIsEOF(t *testing.T) {
	_, _, finalize := Spawn(context.Background(), StopOptions{})
	sum := finalize()
	if sum.Reason != ReasonEOF {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonEOF)
	}
}

func TestSpawnDurationFires(t *testing.T) {
	ctx, _, finalize := Spawn(context.Background(), StopOptions{Duration: 50 * time.Millisecond})
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonDuration {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonDuration)
	}
}

func TestSpawnIdleTimeoutFires(t *testing.T) {
	ctx, c, finalize := Spawn(context.Background(), StopOptions{IdleTimeout: 100 * time.Millisecond})
	// Mark one activity, then sit idle; watchdog should fire ~100ms later.
	c.MarkActivity()
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonIdle {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonIdle)
	}
}

func TestSpawnIdleTimeoutResetsOnActivity(t *testing.T) {
	// Use generous timeouts so goroutine-scheduling jitter on slow runners
	// (CI, WSL) doesn't make activity arrive after the idle watchdog.
	ctx, c, finalize := Spawn(context.Background(), StopOptions{
		IdleTimeout: 500 * time.Millisecond,
		Duration:    1500 * time.Millisecond,
	})
	c.AddPacket() // prime the watchdog so a slow scheduler doesn't trip it
	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.AddPacket()
			}
		}
	}()
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonDuration {
		t.Errorf("expected duration to win over idle, got reason=%q (%+v)", sum.Reason, sum)
	}
	if sum.Packets == 0 {
		t.Errorf("expected non-zero packet count, got %d", sum.Packets)
	}
}

func TestSpawnMaxPacketsFires(t *testing.T) {
	ctx, c, finalize := Spawn(context.Background(), StopOptions{MaxPackets: 5})
	for i := 0; i < 10; i++ {
		c.AddPacket()
	}
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonMaxPackets {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonMaxPackets)
	}
	if sum.Packets < 5 {
		t.Errorf("packets = %d, want >= 5", sum.Packets)
	}
}

func TestSpawnMaxRecordsFires(t *testing.T) {
	ctx, c, finalize := Spawn(context.Background(), StopOptions{MaxRecords: 3})
	for i := 0; i < 5; i++ {
		c.AddRecord()
	}
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonMaxRecords {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonMaxRecords)
	}
}

func TestSpawnMaxBytesFires(t *testing.T) {
	ctx, c, finalize := Spawn(context.Background(), StopOptions{MaxBytes: 1024})
	c.AddBytes(500)
	c.AddBytes(600) // total 1100, over threshold
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonMaxBytes {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonMaxBytes)
	}
	if sum.Bytes < 1024 {
		t.Errorf("bytes = %d, want >= 1024", sum.Bytes)
	}
}

func TestSpawnExplicitStopWins(t *testing.T) {
	ctx, c, finalize := Spawn(context.Background(), StopOptions{Duration: time.Hour})
	c.Stop(ReasonError)
	<-ctx.Done()
	sum := finalize()
	if sum.Reason != ReasonError {
		t.Errorf("reason = %q, want %q", sum.Reason, ReasonError)
	}
}

func TestSummaryStringFormat(t *testing.T) {
	s := Summary{
		Reason:   ReasonSignal,
		Packets:  12345,
		Records:  421,
		Duration: 30 * time.Second,
	}
	got := s.String()
	for _, want := range []string{"12,345", "421", "30s", "exit=signal"} {
		if !strings.Contains(got, want) {
			t.Errorf("Summary.String() = %q; missing %q", got, want)
		}
	}
}

func TestThousandsFormat(t *testing.T) {
	cases := map[int64]string{
		0:          "0",
		1:          "1",
		999:        "999",
		1000:       "1,000",
		12345:      "12,345",
		1234567:    "1,234,567",
		-12345:     "-12,345",
		1000000000: "1,000,000,000",
	}
	for in, want := range cases {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %q, want %q", in, got, want)
		}
	}
}
