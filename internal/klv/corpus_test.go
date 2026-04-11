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

	// Bootstrap: when -update is set and minimal.hex is missing, synthesize
	// it from buildPacket so the initial commit doesn't need hand-computed
	// bytes.
	if update {
		minimalPath := filepath.Join(corpusRoot, "minimal.hex")
		if _, err := os.Stat(minimalPath); os.IsNotExist(err) {
			if err := os.MkdirAll(corpusRoot, 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			packet := buildPacket(t, map[int][]byte{
				2:  make([]byte, 8),
				65: {19},
			})
			if err := os.WriteFile(minimalPath, []byte(hex.EncodeToString(packet)), 0o644); err != nil {
				t.Fatalf("write minimal.hex: %v", err)
			}
		}
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
