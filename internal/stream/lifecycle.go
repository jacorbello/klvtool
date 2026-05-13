package stream

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// StopReason describes why a streaming run exited.
type StopReason string

const (
	// ReasonRunning is the zero value — the run has not stopped.
	ReasonRunning StopReason = ""
	// ReasonSignal: SIGINT or SIGTERM observed.
	ReasonSignal StopReason = "signal"
	// ReasonDuration: --duration elapsed.
	ReasonDuration StopReason = "duration"
	// ReasonIdle: --idle-timeout elapsed with no activity.
	ReasonIdle StopReason = "idle"
	// ReasonMaxPackets: --max-packets limit reached.
	ReasonMaxPackets StopReason = "max-packets"
	// ReasonMaxRecords: --max-records limit reached.
	ReasonMaxRecords StopReason = "max-records"
	// ReasonMaxBytes: --max-bytes limit reached.
	ReasonMaxBytes StopReason = "max-bytes"
	// ReasonEOF: the underlying source returned io.EOF.
	ReasonEOF StopReason = "eof"
	// ReasonError: an unrecoverable error stopped the run.
	ReasonError StopReason = "error"
)

// StopOptions configures the lifecycle. Zero values mean "no limit".
type StopOptions struct {
	Duration    time.Duration
	IdleTimeout time.Duration
	MaxPackets  int64
	MaxRecords  int64
	MaxBytes    int64
}

// Summary captures what happened during a streaming run. Returned by the
// closure that Spawn hands back.
type Summary struct {
	Reason   StopReason
	Packets  int64
	Records  int64
	Bytes    int64
	Duration time.Duration
}

// String formats the summary as a single line suitable for stderr.
//
//	stream: 12,345 TS packets, 421 KLV records, ran 30.0s, exit=signal
func (s Summary) String() string {
	return fmt.Sprintf(
		"stream: %s TS packets, %s KLV records, ran %s, exit=%s",
		thousands(s.Packets),
		thousands(s.Records),
		s.Duration.Round(100*time.Millisecond),
		string(s.Reason),
	)
}

// Counters is the producer-facing handle the demux + writer use to
// register progress. Methods are safe to call from any goroutine.
type Counters struct {
	packets atomic.Int64
	records atomic.Int64
	bytes   atomic.Int64

	// lastActivity stores Unix-nano of the most recent observed activity,
	// read by the idle watchdog. Updated lock-free.
	lastActivity atomic.Int64

	cancelOnce sync.Once
	cancel     context.CancelFunc
	reason     atomic.Value // StopReason
	opts       StopOptions
}

// AddPacket increments the TS packet counter; if --max-packets is set
// and the new value reaches it, cancellation fires.
func (c *Counters) AddPacket() {
	c.lastActivity.Store(time.Now().UnixNano())
	n := c.packets.Add(1)
	if c.opts.MaxPackets > 0 && n >= c.opts.MaxPackets {
		c.fire(ReasonMaxPackets)
	}
}

// AddRecord increments the decoded-record counter.
func (c *Counters) AddRecord() {
	c.lastActivity.Store(time.Now().UnixNano())
	n := c.records.Add(1)
	if c.opts.MaxRecords > 0 && n >= c.opts.MaxRecords {
		c.fire(ReasonMaxRecords)
	}
}

// AddBytes accounts for n raw bytes copied (record command uses this).
func (c *Counters) AddBytes(n int64) {
	if n <= 0 {
		return
	}
	c.lastActivity.Store(time.Now().UnixNano())
	total := c.bytes.Add(n)
	if c.opts.MaxBytes > 0 && total >= c.opts.MaxBytes {
		c.fire(ReasonMaxBytes)
	}
}

// MarkActivity refreshes the idle watchdog without incrementing any
// counter. Sources whose readers fire frequently but rarely deliver
// useful work (e.g. RTSP keepalive frames) call this to keep the
// watchdog quiet.
func (c *Counters) MarkActivity() {
	c.lastActivity.Store(time.Now().UnixNano())
}

// Stop fires cancellation with the given reason. Safe to call multiple
// times — the first call wins. Passing ReasonRunning is rejected so a
// programmer error doesn't produce a summary line with `exit=` and a
// blank reason; use one of the documented terminal reasons instead.
func (c *Counters) Stop(reason StopReason) {
	if reason == ReasonRunning {
		return
	}
	c.fire(reason)
}

func (c *Counters) fire(reason StopReason) {
	c.cancelOnce.Do(func() {
		c.reason.Store(reason)
		if c.cancel != nil {
			c.cancel()
		}
	})
}

// Spawn returns a context that cancels on any of the configured stop
// conditions, a Counters handle the producer uses to record progress,
// and a finalizer that returns the summary. The finalizer detaches the
// signal handler, so callers should defer it.
//
// Callers MUST call the returned finalizer before exit so the signal
// handler is torn down. The finalizer is safe to call multiple times.
func Spawn(parent context.Context, opts StopOptions) (context.Context, *Counters, func() Summary) {
	ctx, cancel := context.WithCancel(parent)
	c := &Counters{
		cancel: cancel,
		opts:   opts,
	}
	c.reason.Store(ReasonRunning)
	c.lastActivity.Store(time.Now().UnixNano())

	// Signal handling is done explicitly (rather than via
	// signal.NotifyContext) so we can distinguish a signal-driven cancel
	// from an explicit finalize() call. The watcher goroutine fires
	// ReasonSignal exclusively when a real SIGINT/SIGTERM arrives.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			c.fire(ReasonSignal)
		case <-ctx.Done():
			// Lifecycle ended without a signal; let the goroutine exit.
		}
	}()

	if opts.Duration > 0 {
		// AfterFunc schedules a single fire on the wall clock. We don't
		// hold the timer reference because the underlying cancel is
		// idempotent — if the run ends sooner, the goroutine inside
		// AfterFunc still fires harmlessly.
		time.AfterFunc(opts.Duration, func() { c.fire(ReasonDuration) })
	}

	var idleStop chan struct{}
	if opts.IdleTimeout > 0 {
		idleStop = make(chan struct{})
		go c.idleWatchdog(ctx, opts.IdleTimeout, idleStop)
	}

	start := time.Now()

	finalized := false
	finalize := func() Summary {
		if finalized {
			return c.snapshot(start)
		}
		finalized = true
		signal.Stop(sigCh)
		cancel()
		if idleStop != nil {
			close(idleStop)
		}
		// If nothing else fired and the producer just returned cleanly
		// (e.g. file source hit EOF without any limit firing), record
		// that as the reason so the summary doesn't claim "signal".
		if c.reason.Load() == ReasonRunning {
			c.reason.Store(ReasonEOF)
		}
		return c.snapshot(start)
	}
	return ctx, c, finalize
}

func (c *Counters) idleWatchdog(ctx context.Context, timeout time.Duration, stop <-chan struct{}) {
	// Tick at a fraction of the timeout so detection latency is bounded.
	// Cap at 1s so a long timeout doesn't make Ctrl-C feel laggy.
	tick := timeout / 4
	if tick < 100*time.Millisecond {
		tick = 100 * time.Millisecond
	}
	if tick > time.Second {
		tick = time.Second
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			last := time.Unix(0, c.lastActivity.Load())
			if time.Since(last) >= timeout {
				c.fire(ReasonIdle)
				return
			}
		}
	}
}

func (c *Counters) snapshot(start time.Time) Summary {
	reason, _ := c.reason.Load().(StopReason)
	return Summary{
		Reason:   reason,
		Packets:  c.packets.Load(),
		Records:  c.records.Load(),
		Bytes:    c.bytes.Load(),
		Duration: time.Since(start),
	}
}

// thousands formats an int64 with US-style grouping (e.g. 12345 →
// "12,345"). Cheap enough to keep dependency-free.
func thousands(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
