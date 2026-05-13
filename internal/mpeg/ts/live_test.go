package ts

import (
	"bytes"
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestIsKLVCandidateStreamType locks the predicate so a future change has
// to update this test deliberately. KLV PID classification has security
// implications (wrong predicate -> silently drop decode work or decode
// the wrong stream).
func TestIsKLVCandidateStreamType(t *testing.T) {
	cases := []struct {
		name string
		typ  uint8
		want bool
	}{
		{"private_pes_0x06", 0x06, true},
		{"metadata_0x15", 0x15, true},
		{"user_private_0xC0", 0xC0, true},
		{"user_private_0xFF", 0xFF, true},
		{"video_h264_0x1B", 0x1B, false},
		{"audio_aac_0x0F", 0x0F, false},
		{"reserved_0x00", 0x00, false},
		{"video_mpeg2_0x02", 0x02, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsKLVCandidateStreamType(tc.typ); got != tc.want {
				t.Errorf("IsKLVCandidateStreamType(0x%02X) = %v, want %v", tc.typ, got, tc.want)
			}
		})
	}
}

// TestLiveDemuxEmitsPESForKLVPID feeds a synthetic stream containing a PAT,
// a PMT declaring a KLV PID (stream type 0x06), and two PES packets on
// that PID. The demux should reassemble them into PES units delivered
// through onPES. PAT/PMT packets must not be emitted as PES.
func TestLiveDemuxEmitsPESForKLVPID(t *testing.T) {
	stream := buildSyntheticKLVStream(t)

	var (
		pesCount    atomic.Int32
		packetCount atomic.Int32
	)
	d := NewLiveDemux(bytes.NewReader(stream))
	err := d.Run(context.Background(), true,
		func(_ Packet) { packetCount.Add(1) },
		func(unit PESUnit) {
			pesCount.Add(1)
			if unit.PID != 0x0300 {
				t.Errorf("emitted PES on PID 0x%04X, want 0x0300", unit.PID)
			}
			if len(unit.Payload) == 0 {
				t.Errorf("emitted PES with empty payload")
			}
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if packetCount.Load() == 0 {
		t.Fatal("no packets observed")
	}
	// Two PUSI-marked PES packets in the stream — the first emits when
	// the second's PUSI fires, the second is delivered via Flush on EOF.
	if got := pesCount.Load(); got != 2 {
		t.Errorf("PES units = %d, want 2", got)
	}
}

// TestLiveDemuxRespectsContextCancel verifies that a cancelled context
// stops the Run loop without consuming the entire input. Without this
// guarantee, a streaming source whose Close happens to race with a
// final block of buffered bytes could keep emitting after the lifecycle
// has declared the run over.
func TestLiveDemuxRespectsContextCancel(t *testing.T) {
	// Build a long stream so the loop has work to do; cancel before any
	// packet is consumed and assert no PES emerges.
	stream := bytes.Repeat(buildPacket(0x1FFF, 0, false, []byte{0xFF}), 1000)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	d := NewLiveDemux(bytes.NewReader(stream))
	start := time.Now()
	err := d.Run(ctx, true, nil, func(_ PESUnit) {
		t.Error("PES emitted after cancel")
	}, nil)
	if err != nil {
		t.Fatalf("Run on pre-cancelled ctx: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Run took %v on pre-cancelled ctx", elapsed)
	}
}

// TestLiveDemuxIgnoresNonKLVStreamTypes confirms that when klvOnly=true,
// PES units on PIDs whose PMT entry is not a KLV candidate (e.g. an H.264
// video stream at type 0x1B) are NOT emitted. Without this filter, a
// caller wiring decode against a typical FMV stream would see giant
// video PESes flooding the KLV pipeline.
func TestLiveDemuxIgnoresNonKLVStreamTypes(t *testing.T) {
	stream := buildSyntheticH264OnlyStream(t)
	d := NewLiveDemux(bytes.NewReader(stream))
	err := d.Run(context.Background(), true, nil, func(unit PESUnit) {
		t.Errorf("unexpected PES emitted on PID 0x%04X (klvOnly=true should suppress non-KLV)", unit.PID)
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

// buildSyntheticKLVStream constructs a minimal MPEG-TS stream containing:
//   - PAT announcing program 1, PMT PID 0x1000
//   - PMT declaring one elementary stream: PID 0x0300, type 0x06 (KLV-candidate)
//   - Two PES packets on PID 0x0300, each PUSI-marked, with a valid
//     PES start code so the assembler parses them cleanly
func buildSyntheticKLVStream(_ *testing.T) []byte {
	var buf bytes.Buffer

	patSection := []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	buf.Write(buildPacket(0x0000, 0, true, patSection))

	pmtSection := []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	buf.Write(buildPacket(0x1000, 0, true, pmtSection))

	// Two PES packets on PID 0x0300, with minimal PES header (no PTS/DTS).
	// PES start code + stream_id + packet_length (0 = unbounded) + flags + header_data_length.
	pesHeader := []byte{0x00, 0x00, 0x01, 0xFC, 0x00, 0x00, 0x80, 0x00, 0x00}
	klvBytes1 := append([]byte{}, pesHeader...)
	klvBytes1 = append(klvBytes1, 0xDE, 0xAD, 0xBE, 0xEF)
	buf.Write(buildPacket(0x0300, 0, true, klvBytes1))

	klvBytes2 := append([]byte{}, pesHeader...)
	klvBytes2 = append(klvBytes2, 0xCA, 0xFE, 0xBA, 0xBE)
	buf.Write(buildPacket(0x0300, 1, true, klvBytes2))

	return buf.Bytes()
}

// buildSyntheticH264OnlyStream advertises a single elementary stream of
// type 0x1B (H.264) — not a KLV candidate. Used to verify klvOnly=true
// suppresses PES emission on the video PID.
func buildSyntheticH264OnlyStream(_ *testing.T) []byte {
	var buf bytes.Buffer

	patSection := []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	buf.Write(buildPacket(0x0000, 0, true, patSection))

	// Stream type 0x1B (H.264) — must NOT be emitted as KLV PES.
	pmtSection := []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x1B, 0xE0, 0xC8, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	buf.Write(buildPacket(0x1000, 0, true, pmtSection))

	pesHeader := []byte{0x00, 0x00, 0x01, 0xE0, 0x00, 0x00, 0x80, 0x00, 0x00}
	pesBytes := append([]byte{}, pesHeader...)
	pesBytes = append(pesBytes, 0x00, 0x00, 0x00, 0x01, 0x09) // NAL access unit delimiter
	buf.Write(buildPacket(0x00C8, 0, true, pesBytes))

	return buf.Bytes()
}
