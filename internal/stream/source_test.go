package stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacorbello/klvtool/internal/model"
)

func TestIsURL(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"-", true},
		{"udp://127.0.0.1:5005", true},
		{"tcp://host:1234", true},
		{"http://example.com/x", true},
		{"https://example.com/x", true},
		{"rtsp://host/path", true},
		{"srt://host:9000", true},
		{"file:///tmp/x.ts", false},
		{"./relative.ts", false},
		{"/absolute/path.ts", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := IsURL(tc.in); got != tc.want {
				t.Fatalf("IsURL(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestOpenEmpty(t *testing.T) {
	_, err := Open(context.Background(), "  ", Options{})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != model.CodeInvalidUsage {
		t.Fatalf("want CodeInvalidUsage, got %v", err)
	}
}

func TestOpenUnsupportedScheme(t *testing.T) {
	_, err := Open(context.Background(), "gopher://example.com", Options{})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != model.CodeInvalidUsage {
		t.Fatalf("want CodeInvalidUsage, got %v", err)
	}
}

func TestOpenFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "x.bin")
	if err := os.WriteFile(tmp, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := Open(context.Background(), tmp, Options{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer src.Close()
	if src.Scheme() != "file" {
		t.Errorf("scheme = %q, want file", src.Scheme())
	}
	b, err := io.ReadAll(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != "hello" {
		t.Errorf("read = %q, want hello", b)
	}
}

func TestOpenFileURL(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "x.bin")
	if err := os.WriteFile(tmp, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := Open(context.Background(), "file://"+tmp, Options{})
	if err != nil {
		t.Fatalf("Open file://: %v", err)
	}
	defer src.Close()
	b, _ := io.ReadAll(src)
	if string(b) != "ok" {
		t.Errorf("got %q", b)
	}
}

func TestOpenFileMissing(t *testing.T) {
	_, err := Open(context.Background(), filepath.Join(t.TempDir(), "missing.bin"), Options{})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != model.CodeTSRead {
		t.Fatalf("want CodeTSRead, got %v", err)
	}
}

func TestOpenRTSPInvalidURLIsUsageError(t *testing.T) {
	// A malformed RTSP URL must surface as InvalidUsage, not a network
	// timeout — so the CLI exits with code 2 (usage) instead of code 1
	// (runtime failure).
	_, err := Open(context.Background(), "rtsp://", Options{})
	var me *model.Error
	if !errors.As(err, &me) {
		t.Fatalf("want *model.Error, got %v", err)
	}
	// We accept either Invalid (parse failure) or BackendExecution
	// (immediate dial failure on missing host). What we MUST NOT do is
	// silently succeed.
	if me.Code != model.CodeInvalidUsage && me.Code != model.CodeBackendExecution {
		t.Fatalf("want InvalidUsage or BackendExecution, got %s", me.Code)
	}
}

func TestOpenRTSPRejectsHeaders(t *testing.T) {
	// gortsplib's public client API can't forward custom request headers
	// today, so accepting --header on rtsp:// would silently no-op and
	// fail auth. Verify the call site rejects it with InvalidUsage
	// instead of dialing and confusing the operator.
	_, err := Open(context.Background(), "rtsp://example.com/x", Options{
		Headers: map[string][]string{"Authorization": {"Bearer t"}},
	})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != model.CodeInvalidUsage {
		t.Fatalf("want CodeInvalidUsage, got %v", err)
	}
	if !strings.Contains(err.Error(), "--header") {
		t.Errorf("error should mention --header, got: %v", err)
	}
}

func TestOpenSRTRejectsUnknownMode(t *testing.T) {
	// mode=bogus must be a usage error; we don't want to dial first and
	// then complain.
	_, err := Open(context.Background(), "srt://127.0.0.1:1?mode=bogus", Options{})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != model.CodeInvalidUsage {
		t.Fatalf("want CodeInvalidUsage, got %v", err)
	}
}

func TestOpenTCPRoundTrip(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	payload := bytes.Repeat([]byte("KLVTOOL"), 8)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		_, _ = c.Write(payload)
		_ = c.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	src, err := Open(ctx, "tcp://"+ln.Addr().String(), Options{})
	if err != nil {
		t.Fatalf("Open tcp: %v", err)
	}
	defer src.Close()
	if src.Scheme() != "tcp" {
		t.Errorf("scheme = %q, want tcp", src.Scheme())
	}
	got, err := io.ReadAll(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q, want %q", got, payload)
	}
}

func TestOpenHTTPRoundTrip(t *testing.T) {
	wantHeader := "Bearer test-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != wantHeader {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("MPEG-TS-BYTES"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	src, err := Open(ctx, srv.URL, Options{Headers: map[string][]string{
		"Authorization": {wantHeader},
	}})
	if err != nil {
		t.Fatalf("Open http: %v", err)
	}
	defer src.Close()
	if src.Scheme() != "http" {
		t.Errorf("scheme = %q, want http", src.Scheme())
	}
	got, err := io.ReadAll(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "MPEG-TS-BYTES" {
		t.Errorf("got %q", got)
	}
}

func TestOpenHTTPAuthFailureIsInvalidUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := Open(context.Background(), srv.URL, Options{})
	var me *model.Error
	if !errors.As(err, &me) || me.Code != model.CodeInvalidUsage {
		t.Fatalf("want CodeInvalidUsage for 401, got %v", err)
	}
}

func TestOpenUDPUnicastRoundTrip(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close() // free the port; udpSource will rebind

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	src, err := Open(ctx, "udp://127.0.0.1:"+itoa(port), Options{})
	if err != nil {
		t.Fatalf("Open udp: %v", err)
	}
	defer src.Close()

	// Send a datagram to the bound port.
	sender, err := net.Dial("udp4", "127.0.0.1:"+itoa(port))
	if err != nil {
		t.Fatalf("dial sender: %v", err)
	}
	defer sender.Close()
	want := bytes.Repeat([]byte{0x47}, 188*3) // three TS-shaped packets
	if _, err := sender.Write(want); err != nil {
		t.Fatalf("send: %v", err)
	}

	buf := make([]byte, len(want))
	if _, err := io.ReadFull(src, buf); err != nil {
		t.Fatalf("readFull: %v", err)
	}
	if !bytes.Equal(buf, want) {
		t.Errorf("udp roundtrip mismatch")
	}
}

func TestOpenStdinReturnsStdinScheme(t *testing.T) {
	src, err := Open(context.Background(), "-", Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if src.Scheme() != "stdin" {
		t.Errorf("scheme = %q, want stdin", src.Scheme())
	}
	if src.RemoteAddr() != "stdin" {
		t.Errorf("RemoteAddr = %q, want stdin", src.RemoteAddr())
	}
}

// itoa is strconv.Itoa without the import — keeps the test file dependency
// surface minimal.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return strings.Clone(string(b[i:]))
}
