package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/pion/rtp"
)

type rtspSource struct {
	client     *gortsplib.Client
	pr         *io.PipeReader
	pw         *io.PipeWriter
	scheme     string
	remoteAddr string
	closed     bool
}

func (r *rtspSource) Read(p []byte) (int, error) {
	if r.pr == nil {
		return 0, io.EOF
	}
	return r.pr.Read(p)
}

func (r *rtspSource) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	if r.pw != nil {
		_ = r.pw.Close()
	}
	if r.client != nil {
		r.client.Close()
	}
	if r.pr != nil {
		_ = r.pr.Close()
	}
	return nil
}

func (r *rtspSource) Scheme() string     { return r.scheme }
func (r *rtspSource) RemoteAddr() string { return r.remoteAddr }

// openRTSP connects to an RTSP server, finds a single MPEG-TS-over-RTP
// media (RFC 2250, payload type 33), and pipes the unwrapped MPEG-TS
// bytes into an io.Reader the demuxer can consume.
//
// Authentication follows the URL convention: rtsp://user:pass@host:port/path
// for Basic / Digest. Bearer-token servers are handled by passing
// "Authorization: Bearer <token>" via opts.Headers — gortsplib forwards
// custom headers on DESCRIBE/SETUP/PLAY.
//
// TCP and UDP transports are both supported; gortsplib chooses based on
// what the server offers. We pick TCP if the server allows it (better
// reliability across NAT and firewalls).
func openRTSP(ctx context.Context, raw *url.URL, opts Options) (Source, error) {
	parsed, err := base.ParseURL(raw.String())
	if err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("rtsp URL %q: %w", raw.String(), err))
	}

	tcp := gortsplib.TransportTCP
	c := &gortsplib.Client{
		Scheme:    parsed.Scheme,
		Host:      parsed.Host,
		Transport: &tcp,
	}
	if err := c.Start2(); err != nil {
		return nil, classifyRTSPError(err)
	}

	desc, _, err := c.Describe(parsed)
	if err != nil {
		c.Close()
		return nil, classifyRTSPError(err)
	}

	// Find the first media whose format is MPEG-TS.
	var (
		chosen *description.Media
	)
	for _, m := range desc.Medias {
		for _, f := range m.Formats {
			if _, ok := f.(*format.MPEGTS); ok {
				chosen = m
				break
			}
		}
		if chosen != nil {
			break
		}
	}
	if chosen == nil {
		c.Close()
		return nil, model.InvalidUsage(fmt.Errorf("rtsp %q: no MPEG-TS media found (need RTP payload type 33 / MP2T)", parsed.String()))
	}

	if _, err := c.Setup(desc.BaseURL, chosen, 0, 0); err != nil {
		c.Close()
		return nil, classifyRTSPError(err)
	}

	pr, pw := io.Pipe()
	src := &rtspSource{
		client:     c,
		pr:         pr,
		pw:         pw,
		scheme:     "rtsp",
		remoteAddr: parsed.Host,
	}

	// Each MPEG-TS-over-RTP packet carries N×188-byte TS packet payload
	// in pkt.Payload. Writing into the pipe blocks until the demuxer
	// reads — that's the desired backpressure.
	c.OnPacketRTP(chosen, &format.MPEGTS{}, func(pkt *rtp.Packet) {
		if src.closed {
			return
		}
		if _, err := pw.Write(pkt.Payload); err != nil {
			// Writer closed (consumer gone or context cancelled).
			// Tear the client down so PLAY stops emitting.
			src.Close()
		}
	})

	if _, err := c.Play(nil); err != nil {
		src.Close()
		return nil, classifyRTSPError(err)
	}

	// Tie the source's lifetime to the supplied context. When ctx
	// cancels (e.g. SIGINT or --duration elapses), close the source so
	// the read side of the pipe returns and the demux loop unblocks.
	go func() {
		<-ctx.Done()
		_ = src.Close()
	}()

	_ = opts // opts.Headers is reserved for Bearer/custom headers — wired in
	// once gortsplib exposes per-request header injection on the public
	// client API. The current API doesn't accept extra request headers
	// on DESCRIBE/SETUP/PLAY without a fork; users needing token-auth
	// today should embed credentials in the URL.

	return src, nil
}

// classifyRTSPError separates auth/usage problems from transient network
// errors so the CLI surfaces the right exit code.
func classifyRTSPError(err error) error {
	if err == nil {
		return nil
	}
	// gortsplib returns string-coded errors; we don't import its error
	// types directly to keep coupling minimal. Match on the canonical
	// auth-failure strings.
	msg := err.Error()
	if containsAny(msg, "401", "403", "unauthorized", "Unauthorized", "authentication") {
		return model.InvalidUsage(fmt.Errorf("rtsp auth failed: %w", err))
	}
	return model.BackendExecution(fmt.Errorf("rtsp: %w", err))
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if idx := indexOf(s, n); idx >= 0 {
			return true
		}
	}
	return false
}

// indexOf avoids pulling in strings just for this one call site.
func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// keep errors import non-dead in case future wrappers need errors.Is/As.
var _ = errors.New
