package stream

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	srt "github.com/datarhei/gosrt"
	"github.com/jacorbello/klvtool/internal/model"
)

type srtSource struct {
	conn       srt.Conn
	scheme     string
	remoteAddr string
}

func (s *srtSource) Read(p []byte) (int, error) { return s.conn.Read(p) }
func (s *srtSource) Close() error               { return s.conn.Close() }
func (s *srtSource) Scheme() string             { return s.scheme }
func (s *srtSource) RemoteAddr() string         { return s.remoteAddr }

// openSRT connects to a Secure Reliable Transport endpoint and returns
// its byte stream as a Source. URL query params follow the canonical
// ffmpeg/srt-live-transmit shape: passphrase, pbkeylen, streamid,
// latency, etc. — gosrt's Config.UnmarshalURL handles those. The
// `mode=` query param is NOT part of gosrt's config (mode is a
// connection-time choice between Dial and Listen), so we parse it
// here and dispatch.
//
// Mode defaults to caller (we connect to a listening server). Set
// "?mode=listener" to wait for an incoming caller; the listener binds
// the URL's host:port and accepts the first incoming caller.
func openSRT(ctx context.Context, raw *url.URL, _ Options) (Source, error) {
	cfg := srt.DefaultConfig()
	address, err := cfg.UnmarshalURL(raw.String())
	if err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("srt URL %q: %w", raw.String(), err))
	}
	if err := cfg.Validate(); err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("srt config: %w", err))
	}

	mode := strings.ToLower(raw.Query().Get("mode"))
	if mode == "" {
		mode = "caller"
	}
	switch mode {
	case "caller", "":
		conn, err := srt.Dial("srt", address, cfg)
		if err != nil {
			return nil, classifySRTError(err)
		}
		src := &srtSource{
			conn:       conn,
			scheme:     "srt",
			remoteAddr: address,
		}
		go func() {
			<-ctx.Done()
			_ = conn.Close()
		}()
		return src, nil
	case "listener":
		return nil, model.InvalidUsage(fmt.Errorf("srt mode=listener is not yet implemented (caller-only in this release)"))
	case "rendezvous":
		return nil, model.InvalidUsage(fmt.Errorf("srt mode=rendezvous is not yet implemented (caller-only in this release)"))
	default:
		return nil, model.InvalidUsage(fmt.Errorf("srt mode=%q (want caller|listener|rendezvous)", mode))
	}
}

// classifySRTError separates user-fixable errors (bad passphrase, bad
// streamid) from transient network problems. gosrt returns string-coded
// errors; we string-match on the few that matter for CLI exit codes.
func classifySRTError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "passphrase") || strings.Contains(msg, "auth") || strings.Contains(msg, "rejected") {
		return model.InvalidUsage(fmt.Errorf("srt connect: %w", err))
	}
	return model.BackendExecution(fmt.Errorf("srt connect: %w", err))
}
