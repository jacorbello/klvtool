// Package stream turns an --input value (a file path or a URL) into an
// io.ReadCloser. The factory dispatches on URL scheme so the CLI can
// route a single --input through one consistent code path regardless of
// whether the bytes come from disk, stdin, a UDP listener, a TCP dial, an
// HTTP body, an RTSP SETUP/PLAY, or an SRT caller/listener.
//
// Each scheme implementation lives in its own file; Source is the small
// interface the consumer sees.
package stream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jacorbello/klvtool/internal/model"
)

// Source is an io.ReadCloser plus a small amount of metadata used for
// the run-summary line and structured error wrapping. The Close method
// must be safe to call from a goroutine other than the one reading;
// streaming lifecycle relies on closing the source to unblock a stalled
// Read when the context cancels.
type Source interface {
	Read(p []byte) (int, error)
	Close() error
	// Scheme returns the URL scheme that produced this source (e.g.
	// "udp", "rtsp", "file"). "-" returns "stdin".
	Scheme() string
	// RemoteAddr returns a human-readable description of where bytes are
	// coming from (host:port, file path, "stdin"). Used only in the
	// stream summary line — not parsed by anything.
	RemoteAddr() string
}

// Options controls per-scheme behavior. Fields not applicable to the
// resolved scheme are ignored.
type Options struct {
	// Headers carries Authorization, custom auth, and any other HTTP/RTSP
	// request headers. The slice form supports multi-valued headers like
	// "Set-Cookie" even though klvtool itself only emits, never reads.
	Headers map[string][]string
	// Iface selects the egress network interface for multicast UDP
	// (Linux: device name like "eth0", or a local IP address). Empty
	// means "use the OS default".
	Iface string
}

// Open resolves raw to a Source. raw may be:
//
//   - a bare filesystem path ("./mission.ts", "/tmp/cap.ts") — file Source
//   - "-" — stdin Source
//   - a URL with a recognized scheme: file://, udp://, tcp://, http://,
//     https://, rtsp://, srt://
//
// Errors are wrapped with model.InvalidUsage for unparseable input or
// unsupported schemes, and model.BackendExecution for connect-time
// failures. Callers receive a context they must keep alive for the
// lifetime of the read loop — sources may use it to unblock pending
// reads on cancellation.
func Open(ctx context.Context, raw string, opts Options) (Source, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, model.InvalidUsage(errors.New("input is empty"))
	}
	if raw == "-" {
		return openStdin(ctx)
	}
	// A bare path won't parse as a URL with a scheme. url.Parse is lenient
	// — it will happily parse "./foo.ts" with an empty scheme. Detect a
	// real scheme by looking for "://" before falling through to file.
	if !strings.Contains(raw, "://") {
		return openFile(ctx, raw)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("parse input URL %q: %w", raw, err))
	}
	switch strings.ToLower(u.Scheme) {
	case "file":
		return openFile(ctx, u.Path)
	case "udp":
		return openUDP(ctx, u, opts)
	case "tcp":
		return openTCP(ctx, u, opts)
	case "http", "https":
		return openHTTP(ctx, u, opts)
	case "rtsp":
		return openRTSP(ctx, u, opts)
	case "srt":
		return openSRT(ctx, u, opts)
	default:
		return nil, model.InvalidUsage(fmt.Errorf("unsupported scheme %q in input %q", u.Scheme, raw))
	}
}

// IsURL reports whether raw should be opened via Open rather than the
// existing file-based code path. The CLI uses this to decide whether to
// skip os.Stat / IsRegular gating that only makes sense for filesystem
// inputs.
func IsURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "-" {
		return true
	}
	if !strings.Contains(raw, "://") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "file":
		// file:// is just a more verbose path; treat it as a file input
		// so the existing ffmpeg backend continues to handle it.
		return false
	case "":
		return false
	default:
		return true
	}
}
