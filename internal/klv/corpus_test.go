package klv

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

const corpusRoot = "../../testdata/klv_packets"

var update bool

func init() {
	flag.BoolVar(&update, "update", false, "regenerate corpus fixtures and snapshots")
}

func TestCorpus(t *testing.T) {
	reg := NewRegistry()
	reg.Register(st0601.V19())

	// Bootstrap: when -update is set and a fixture is missing, synthesize it
	// so the initial commit doesn't need hand-computed bytes. Each entry
	// below guards a distinct decode path — see the comments.
	if update {
		if err := os.MkdirAll(corpusRoot, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeHex := func(name string, packet []byte) {
			p := filepath.Join(corpusRoot, name)
			if _, err := os.Stat(p); err == nil {
				return
			}
			if err := os.WriteFile(p, []byte(hex.EncodeToString(packet)), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}

		// minimal: mandatory items only — smoke test for Decode.
		writeHex("minimal.hex", buildPacket(t, map[int][]byte{
			2:  make([]byte, 8),
			65: {19},
		}))

		// error_indicator: Platform Pitch Angle (tag 6, int16) with the
		// 0x8000 sentinel. Guards C1 — FloatValue NaN must marshal as JSON
		// null rather than crashing json.Marshal.
		writeHex("error_indicator.hex", buildPacket(t, map[int][]byte{
			2:  make([]byte, 8),
			6:  {0x80, 0x00},
			65: {19},
		}))

		// imapb: Sensor Latitude (tag 13, FormatIMAPB, 4 bytes, -90..90).
		// Raw 0xC0000000 decodes to ~+45°, exercising fromIMAPB.
		writeHex("imapb.hex", buildPacket(t, map[int][]byte{
			2:  make([]byte, 8),
			13: {0xC0, 0x00, 0x00, 0x00},
			65: {19},
		}))

		// nested_ls: Security Local Set (tag 48) treated as an opaque
		// NestedValue passthrough. Guards the {specHint, raw} JSON shape.
		writeHex("nested_ls.hex", buildPacket(t, map[int][]byte{
			2:  make([]byte, 8),
			48: {0x01, 0x02, 0x03},
			65: {19},
		}))

		// long_form_length: forces the outer BER length into long form
		// (0x81 LL) even though the value fits in short form. Guards the
		// non-canonical BER length path through the top-level Decode entry
		// point (checksum covers the wire length bytes, so the decoder must
		// not canonicalize).
		longFormPacket, _, _, _ := buildLongFormPacket(t, map[int][]byte{
			2:  make([]byte, 8),
			65: {19},
		})
		writeHex("long_form_length.hex", longFormPacket)
	}

	entries, err := os.ReadDir(corpusRoot)
	if err != nil {
		t.Skipf("corpus dir missing: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".hex") {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			hexPath := filepath.Join(corpusRoot, name)
			snapPath := strings.TrimSuffix(hexPath, ".hex") + ".json"

			raw, err := os.ReadFile(hexPath)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			cleaned := strings.Join(strings.Fields(string(raw)), "")
			packet, err := hex.DecodeString(cleaned)
			if err != nil {
				t.Fatalf("hex decode: %v", err)
			}

			rec, err := Decode(reg, packet)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			got, err := json.MarshalIndent(rec, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if update {
				if err := os.WriteFile(snapPath, got, 0o644); err != nil {
					t.Fatalf("write snapshot: %v", err)
				}
				return
			}

			want, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("snapshot missing; run with -update to create: %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("snapshot mismatch for %s\n--- got ---\n%s\n--- want ---\n%s",
					name, got, want)
			}
		})
	}
}
