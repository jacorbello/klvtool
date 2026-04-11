package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const sampleFixture = "../testdata/fixtures/sample.ts"

func TestDecodeAgainstSampleTS(t *testing.T) {
	if _, err := os.Stat(sampleFixture); os.IsNotExist(err) {
		t.Skip("sample.ts fixture not present; run `make test-data` to download")
	}

	// Build klvtool once for the test.
	bin := filepath.Join(t.TempDir(), "klvtool")
	build := exec.Command("go", "build", "-o", bin, "../cmd/klvtool")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build: %v", err)
	}

	// Run decode with NDJSON.
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command(bin, "decode", "--input", sampleFixture, "--format", "ndjson")
	cmd.Stdout = out
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("decode: %v\nstderr: %s", err, errBuf.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("no NDJSON output; stderr: %s", errBuf.String())
	}

	type minimalRec struct {
		Schema    string `json:"schema"`
		LSVersion int    `json:"lsVersion"`
		Checksum  struct {
			Valid bool `json:"valid"`
		} `json:"checksum"`
		Items []struct {
			Tag int `json:"tag"`
		} `json:"items"`
		Diagnostics []struct {
			Severity string `json:"severity"`
		} `json:"diagnostics"`
	}

	var checksumValid, checksumInvalid int
	for i, line := range lines {
		var rec minimalRec
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d: invalid JSON: %v\n%s", i, err, line)
		}
		if rec.Schema != "urn:misb:KLV:bin:0601.19" {
			t.Errorf("line %d: schema = %s", i, rec.Schema)
		}
		// Mandatory items.
		hasTag := map[int]bool{}
		for _, it := range rec.Items {
			hasTag[it.Tag] = true
		}
		for _, tag := range []int{1, 2, 65} {
			if !hasTag[tag] {
				t.Errorf("line %d: missing mandatory tag %d", i, tag)
			}
		}
		if rec.Checksum.Valid {
			checksumValid++
		} else {
			checksumInvalid++
		}
	}

	if checksumValid == 0 {
		t.Errorf("no valid checksums across %d records", len(lines))
	}
	t.Logf("decoded %d records (%d valid, %d invalid checksums)", len(lines), checksumValid, checksumInvalid)
}

func TestDecodeTextFormatAgainstSampleTS(t *testing.T) {
	if _, err := os.Stat(sampleFixture); os.IsNotExist(err) {
		t.Skip("sample.ts fixture not present")
	}
	bin := filepath.Join(t.TempDir(), "klvtool")
	if err := exec.Command("go", "build", "-o", bin, "../cmd/klvtool").Run(); err != nil {
		t.Fatalf("build: %v", err)
	}
	out := &bytes.Buffer{}
	cmd := exec.Command(bin, "decode", "--input", sampleFixture, "--format", "text")
	cmd.Stdout = out
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.String(), "Packet 0") {
		t.Errorf("expected 'Packet 0' in text output, got: %s", out.String()[:min(200, len(out.String()))])
	}
}
