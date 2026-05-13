package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jacorbello/klvtool/internal/model"
)

type httpSource struct {
	body       io.ReadCloser
	scheme     string
	remoteAddr string
}

func (h *httpSource) Read(p []byte) (int, error) { return h.body.Read(p) }
func (h *httpSource) Close() error               { return h.body.Close() }
func (h *httpSource) Scheme() string             { return h.scheme }
func (h *httpSource) RemoteAddr() string         { return h.remoteAddr }

func openHTTP(ctx context.Context, u *url.URL, opts Options) (Source, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, model.InvalidUsage(fmt.Errorf("http request %q: %w", u.String(), err))
	}
	for k, vs := range opts.Headers {
		// http.Header.Set replaces any prior value, then Add appends the
		// remainder — preserves multi-valued semantics for callers who
		// supply multiple --header flags for the same key.
		for i, v := range vs {
			if i == 0 {
				req.Header.Set(k, v)
			} else {
				req.Header.Add(k, v)
			}
		}
	}
	// Default Accept: MPEG-TS payloads are typically video/MP2T. Don't
	// overwrite a caller-supplied Accept header.
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "video/MP2T, application/octet-stream;q=0.9, */*;q=0.1")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, model.BackendExecution(fmt.Errorf("http %s %s: %w", req.Method, u.String(), err))
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return nil, model.InvalidUsage(fmt.Errorf("http %s %s: %s", req.Method, u.String(), resp.Status))
		default:
			return nil, model.BackendExecution(fmt.Errorf("http %s %s: %s", req.Method, u.String(), resp.Status))
		}
	}
	scheme := strings.ToLower(u.Scheme)
	src := &httpSource{body: resp.Body, scheme: scheme, remoteAddr: u.Host}
	// Symmetric with TCP/UDP/RTSP: close the body when ctx fires so a
	// stalled Read in the consumer unblocks even if the consumer
	// doesn't call Close itself (e.g. lifecycle cancel via signal /
	// duration / counter limit). The HTTP request's ctx already plumbs
	// down to the transport, but a long-lived chunked body without
	// fresh chunks won't observe ctx without an explicit body close.
	go func() {
		<-ctx.Done()
		_ = resp.Body.Close()
	}()
	return src, nil
}
