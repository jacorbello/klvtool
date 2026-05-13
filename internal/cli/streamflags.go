package cli

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jacorbello/klvtool/internal/stream"
)

// streamFlagMode selects which streaming flags a command exposes. Decode
// gets --max-records but not --max-bytes; record gets --max-bytes but not
// --max-records; inspect/diagnose get neither. The flag set is shared so
// the descriptions and parse semantics stay consistent.
type streamFlagMode int

const (
	streamFlagsDecode streamFlagMode = iota
	streamFlagsInspect
	streamFlagsDiagnose
	streamFlagsRecord
)

// streamFlags holds the parsed values for the shared streaming flag set.
// Zero values mean "off".
type streamFlags struct {
	record          string
	recordOverwrite bool
	duration        time.Duration
	idleTimeout     time.Duration
	maxPackets      int64
	maxRecords      int64
	maxBytes        int64
	headers         headerFlag
	iface           string
}

// registerStreamFlags wires the shared streaming flags onto fs. The mode
// arg controls which flags appear so each command's --help only lists
// flags it can actually act on.
func registerStreamFlags(fs *flag.FlagSet, s *streamFlags, mode streamFlagMode) {
	if s.headers == nil {
		s.headers = headerFlag{}
	}
	fs.StringVar(&s.record, "record", "", "tee inbound source bytes to this file (works for any --input, including a regular file)")
	fs.BoolVar(&s.recordOverwrite, "record-overwrite", false, "allow --record to overwrite an existing file (default refuses; honors O_CREATE|O_EXCL semantics)")
	fs.DurationVar(&s.duration, "duration", 0, "stop after this wall-clock duration (e.g. 30s, 5m, 1h); 0 disables")
	fs.DurationVar(&s.idleTimeout, "idle-timeout", 0, "stop if no inbound bytes are observed for this duration; 0 disables")
	fs.Int64Var(&s.maxPackets, "max-packets", 0, "stop after this many TS packets are observed; 0 disables")
	if mode == streamFlagsDecode {
		fs.Int64Var(&s.maxRecords, "max-records", 0, "stop after this many KLV records are decoded; 0 disables")
	}
	if mode == streamFlagsRecord {
		// Use Int64 to keep the parser simple; humans typically pass small
		// values via int64 directly. A future enhancement could parse
		// "10M" style suffixes through a flag.Value implementation.
		fs.Int64Var(&s.maxBytes, "max-bytes", 0, "stop after this many bytes have been captured (record only); 0 disables")
	}
	fs.Var(&s.headers, "header", "extra HTTP request header in 'Key: Value' form (repeatable; e.g. -header \"Authorization: Bearer $TOKEN\"). HTTPS only — RTSP servers must embed credentials in the URL until token-auth is wired up.")
	fs.StringVar(&s.iface, "iface", "", "egress network interface for UDP multicast joins (Linux: device name like eth0, or a local IP)")
}

// stopOptions converts parsed flags into a stream.StopOptions. Called
// only when the input is a URL — file inputs ignore these flags except
// --record, which has its own handling on the source side.
func (s streamFlags) stopOptions() stream.StopOptions {
	return stream.StopOptions{
		Duration:    s.duration,
		IdleTimeout: s.idleTimeout,
		MaxPackets:  s.maxPackets,
		MaxRecords:  s.maxRecords,
		MaxBytes:    s.maxBytes,
	}
}

// streamOptions converts parsed flags into a stream.Options for Open.
func (s streamFlags) streamOptions() stream.Options {
	return stream.Options{
		Headers: map[string][]string(s.headers),
		Iface:   s.iface,
	}
}

// headerFlag implements flag.Value for the repeatable --header flag.
// Each Set call parses "Key: Value" and appends to the map; the same
// key can appear multiple times.
type headerFlag map[string][]string

func (h *headerFlag) String() string {
	if h == nil || len(*h) == 0 {
		return ""
	}
	keys := make([]string, 0, len(*h))
	for k := range *h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(*h))
	for _, k := range keys {
		for _, v := range (*h)[k] {
			parts = append(parts, fmt.Sprintf("%s: %s", k, v))
		}
	}
	return strings.Join(parts, ", ")
}

func (h *headerFlag) Set(raw string) error {
	idx := strings.Index(raw, ":")
	if idx < 0 {
		return fmt.Errorf("expected 'Key: Value', got %q", raw)
	}
	key := strings.TrimSpace(raw[:idx])
	val := strings.TrimSpace(raw[idx+1:])
	if key == "" {
		return fmt.Errorf("header key is empty in %q", raw)
	}
	if *h == nil {
		*h = headerFlag{}
	}
	(*h)[key] = append((*h)[key], val)
	return nil
}
