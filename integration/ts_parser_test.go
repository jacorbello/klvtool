package integration

import (
	"errors"
	"io"
	"os"
	"testing"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

const tsParserFixturePath = "../testdata/fixtures/sample.ts"

func TestDiscoverStreamsFromSampleTS(t *testing.T) {
	if _, err := os.Stat(tsParserFixturePath); os.IsNotExist(err) {
		t.Skip("sample.ts not available — run 'make test-data' to download")
	}

	file, err := os.Open(tsParserFixturePath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = file.Close() }()

	table, offset, err := ts.DiscoverStreams(file)
	if err != nil {
		t.Fatalf("DiscoverStreams: %v", err)
	}

	if offset == 0 {
		t.Error("offset should be > 0 after discovery")
	}
	if len(table.Programs) == 0 {
		t.Fatal("expected at least one program")
	}

	for pn, streams := range table.Programs {
		t.Logf("Program %d:", pn)
		for _, s := range streams {
			t.Logf("  PID=0x%04X StreamType=0x%02X", s.PID, s.StreamType)
		}
	}

	foundData := false
	for _, streams := range table.Programs {
		for _, s := range streams {
			if s.StreamType == 0x06 || s.StreamType == 0x15 {
				foundData = true
				break
			}
		}
	}
	if !foundData {
		t.Error("expected at least one data/metadata stream (type 0x06 or 0x15)")
	}
}

func TestFullScanPESReassemblyFromSampleTS(t *testing.T) {
	if _, err := os.Stat(tsParserFixturePath); os.IsNotExist(err) {
		t.Skip("sample.ts not available — run 'make test-data' to download")
	}

	file, err := os.Open(tsParserFixturePath)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = file.Close() }()

	table, _, err := ts.DiscoverStreams(file)
	if err != nil {
		t.Fatalf("DiscoverStreams: %v", err)
	}

	dataPIDs := make(map[uint16]bool)
	for _, streams := range table.Programs {
		for _, s := range streams {
			if s.StreamType == 0x06 || s.StreamType == 0x15 || s.StreamType >= 0xC0 {
				dataPIDs[s.PID] = true
			}
		}
	}
	if len(dataPIDs) == 0 {
		t.Skip("no data PIDs found")
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek: %v", err)
	}

	scanner := ts.NewPacketScanner(file, ts.ScanConfig{PayloadPIDs: dataPIDs})
	asm := ts.NewPESAssembler()

	var totalPackets int64
	var pesUnits int

	for {
		pkt, err := scanner.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("scan error at packet %d: %v", totalPackets, err)
		}
		totalPackets++

		if unit := asm.Feed(pkt); unit != nil {
			pesUnits++
			if unit.PacketCount == 0 {
				t.Error("PES unit with 0 packets")
			}
		}
	}

	for range asm.Flush() {
		pesUnits++
	}

	t.Logf("Total packets: %d", totalPackets)
	t.Logf("PES units from data streams: %d", pesUnits)

	if totalPackets == 0 {
		t.Error("expected > 0 total packets")
	}
	if pesUnits == 0 {
		t.Error("expected > 0 PES units from data streams")
	}
}
