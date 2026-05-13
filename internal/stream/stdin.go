package stream

import (
	"context"
	"io"
	"os"
)

type stdinSource struct {
	r io.Reader
}

func (s *stdinSource) Read(p []byte) (int, error) { return s.r.Read(p) }

// Close on stdin is a no-op — closing os.Stdin can confuse a parent shell
// or a TTY-attached terminal. The stream lifecycle relies on the context
// or process exit to stop the read.
func (s *stdinSource) Close() error   { return nil }
func (s *stdinSource) Scheme() string { return "stdin" }
func (s *stdinSource) RemoteAddr() string {
	return "stdin"
}

func openStdin(_ context.Context) (Source, error) {
	return &stdinSource{r: os.Stdin}, nil
}
