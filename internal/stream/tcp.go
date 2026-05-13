package stream

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/jacorbello/klvtool/internal/model"
)

type tcpSource struct {
	conn       net.Conn
	scheme     string
	remoteAddr string
}

func (t *tcpSource) Read(p []byte) (int, error) { return t.conn.Read(p) }
func (t *tcpSource) Close() error               { return t.conn.Close() }
func (t *tcpSource) Scheme() string             { return t.scheme }
func (t *tcpSource) RemoteAddr() string         { return t.remoteAddr }

func openTCP(ctx context.Context, u *url.URL, _ Options) (Source, error) {
	if u.Host == "" {
		return nil, model.InvalidUsage(fmt.Errorf("tcp URL %q: missing host:port", u.String()))
	}
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, model.BackendExecution(fmt.Errorf("dial tcp %q: %w", u.Host, err))
	}
	src := &tcpSource{conn: conn, scheme: "tcp", remoteAddr: u.Host}
	// Closing the conn from the context unblocks any pending Read so the
	// lifecycle can shut down cleanly on Ctrl-C or --duration.
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	return src, nil
}
