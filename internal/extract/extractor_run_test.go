package extract

import (
	"context"
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

func TestExtractorRunCanonicalizesRecords(t *testing.T) {
	backend := &runBackendStub{
		descriptor: BackendDescriptor{Name: BackendFFmpeg, Healthy: true},
		version:    "1.2.3",
		records: []PayloadRecord{
			{PID: 0x1bd, Payload: []byte("b")},
			{PID: 0x045, Payload: []byte("a")},
		},
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
	if got.BackendVersion != "1.2.3" {
		t.Fatalf("expected backend version 1.2.3, got %q", got.BackendVersion)
	}
	if len(got.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got.Records))
	}
	if got.Records[0].PID != 0x045 {
		t.Fatalf("expected first pid 0x045, got %#x", got.Records[0].PID)
	}
	if got.Records[0].RecordID != "klv-001" {
		t.Fatalf("expected first record id klv-001, got %q", got.Records[0].RecordID)
	}
	if got.Records[1].RecordID != "klv-002" {
		t.Fatalf("expected second record id klv-002, got %q", got.Records[1].RecordID)
	}
}
