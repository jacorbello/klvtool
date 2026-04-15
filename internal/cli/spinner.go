package cli

import (
	"fmt"
	"io"
	"sync"
	"time"
)

var spinFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinner displays an animated progress indicator on a writer (typically stderr).
// It is safe for concurrent use: call stop() exactly once when the work is done.
type spinner struct {
	w      io.Writer
	done   chan struct{}
	wg     sync.WaitGroup
	color  colorizer
	pretty bool
}

// startSpinner begins animating a spinner with the given message.
// Call the returned stop function when the work completes; stop clears the
// spinner line so subsequent output is clean. The stop function is safe to
// call multiple times.
func startSpinner(w io.Writer, color colorizer, pretty bool, msg string) func() {
	s := &spinner{w: w, done: make(chan struct{}), color: color, pretty: pretty}
	if !pretty || w == nil {
		return func() {}
	}
	s.wg.Add(1)
	go s.run(msg)
	var once sync.Once
	return func() { once.Do(func() { close(s.done); s.wg.Wait() }) }
}

func (s *spinner) run(msg string) {
	defer s.wg.Done()
	tick := time.NewTicker(80 * time.Millisecond)
	defer tick.Stop()

	i := 0
	for {
		select {
		case <-s.done:
			s.clear()
			return
		case <-tick.C:
			frame := spinFrames[i%len(spinFrames)]
			_, _ = fmt.Fprintf(s.w, "\r%s %s", s.color.cyan(frame), msg)
			i++
		}
	}
}

func (s *spinner) clear() {
	// Overwrite the spinner line with spaces, then return to start of line.
	_, _ = fmt.Fprintf(s.w, "\r%*s\r", 60, "")
}
