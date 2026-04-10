package extract

import (
	"context"
	"reflect"
	"testing"
)

type runSelectorStub struct {
	response ExtractionResponse
	err      error
	calls    int
}

func (s *runSelectorStub) Select(req ExtractionRequest) (ExtractionResponse, error) {
	s.calls++
	return s.response, s.err
}

type runBackendStub struct {
	descriptor    BackendDescriptor
	version       string
	records       []PayloadRecord
	extractArg    string
	versionArg    context.Context
	extractArgCtx context.Context
}

func (b *runBackendStub) Descriptor() BackendDescriptor {
	return b.descriptor
}

func (b *runBackendStub) Version(ctx context.Context) (string, error) {
	b.versionArg = ctx
	return b.version, nil
}

func (b *runBackendStub) Extract(ctx context.Context, path string) ([]PayloadRecord, error) {
	b.extractArgCtx = ctx
	b.extractArg = path
	return b.records, nil
}

func TestExtractorRunCanonicalizesEquivalentRecordOrder(t *testing.T) {
	tsid := uint16Ptr(7)

	gotA := runExtractorRun(t, []PayloadRecord{
		{
			PID:               0x045,
			Payload:           []byte("same"),
			TransportStreamID: tsid,
		},
		{
			PID:     0x045,
			Payload: []byte("same"),
		},
	})
	gotB := runExtractorRun(t, []PayloadRecord{
		{
			PID:     0x045,
			Payload: []byte("same"),
		},
		{
			PID:               0x045,
			Payload:           []byte("same"),
			TransportStreamID: tsid,
		},
	})

	if !reflect.DeepEqual(gotA.Records, gotB.Records) {
		t.Fatalf("expected canonicalized records to match across backend orderings\nA: %#v\nB: %#v", gotA.Records, gotB.Records)
	}
	if len(gotA.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(gotA.Records))
	}
	if gotA.Records[0].TransportStreamID != nil {
		t.Fatal("expected nil TransportStreamID record first")
	}
	if gotA.Records[0].RecordID != "klv-001" || gotA.Records[1].RecordID != "klv-002" {
		t.Fatalf("expected stable record ids, got %#v", []string{gotA.Records[0].RecordID, gotA.Records[1].RecordID})
	}
	for i, result := range []RunResult{gotA, gotB} {
		if result.BackendVersion != "1.2.3" {
			t.Fatalf("run %d: expected backend version 1.2.3, got %q", i, result.BackendVersion)
		}
		for j, record := range result.Records {
			if record.Warnings == nil {
				t.Fatalf("run %d record %d: expected warnings to be non-nil", i, j)
			}
			if len(record.Warnings) != 0 {
				t.Fatalf("run %d record %d: expected empty warnings, got %v", i, j, record.Warnings)
			}
		}
	}
}

func runExtractorRun(t *testing.T, records []PayloadRecord) RunResult {
	t.Helper()

	backend := &runBackendStub{
		descriptor: BackendDescriptor{Name: BackendFFmpeg, Healthy: true},
		version:    "1.2.3",
		records:    records,
	}
	selector := &runSelectorStub{
		response: ExtractionResponse{
			Selected: BackendDescriptor{Name: BackendFFmpeg, Healthy: true},
			Backends: []BackendDescriptor{{Name: BackendFFmpeg, Healthy: true}},
		},
	}
	extractor := &Extractor{
		Selector: selector,
		Backends: map[BackendName]Backend{
			BackendFFmpeg: backend,
		},
	}

	got, err := extractor.Run(context.Background(), RunRequest{
		InputPath: "input.ts",
		Backend:   BackendAuto,
		Backends:  []BackendDescriptor{{Name: BackendFFmpeg, Healthy: true}},
	})
	if err != nil {
		t.Fatalf("expected run to succeed, got error: %v", err)
	}

	if selector.calls != 1 {
		t.Fatalf("expected selector to be called once, got %d", selector.calls)
	}
	if backend.extractArg != "input.ts" {
		t.Fatalf("expected backend extract path input.ts, got %q", backend.extractArg)
	}
	if backend.versionArg == nil {
		t.Fatal("expected version context to be passed to backend")
	}
	if backend.extractArgCtx == nil {
		t.Fatal("expected extract context to be passed to backend")
	}

	return got
}
