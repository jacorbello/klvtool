package extract

import (
	"context"
	"errors"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
)

type stubBackend struct {
	descriptor BackendDescriptor
	version    string
	versionErr error
	records    []PayloadRecord
	extractErr error
}

func (s stubBackend) Descriptor() BackendDescriptor { return s.descriptor }
func (s stubBackend) Version(ctx context.Context) (string, error) {
	return s.version, s.versionErr
}
func (s stubBackend) Extract(ctx context.Context, path string) ([]PayloadRecord, error) {
	return s.records, s.extractErr
}

func TestRunExtractsPayloadsFromHealthyBackend(t *testing.T) {
	backend := stubBackend{
		descriptor: BackendDescriptor{Name: "ffmpeg", Healthy: true},
		version:    "7.1",
		records: []PayloadRecord{
			{RecordID: "klv-001", PID: 0x102, Payload: []byte{0x01}},
		},
	}
	ext := NewExtractor(backend)

	result, err := ext.Run(context.Background(), RunRequest{
		InputPath: "input.ts",
		Backend:   BackendDescriptor{Name: "ffmpeg", Healthy: true},
	})
	if err != nil {
		t.Fatalf("expected successful extraction, got error: %v", err)
	}
	if result.BackendVersion != "7.1" {
		t.Fatalf("expected version 7.1, got %q", result.BackendVersion)
	}
	if len(result.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(result.Records))
	}
}

func TestRunFailsWhenBackendUnhealthy(t *testing.T) {
	backend := stubBackend{
		descriptor: BackendDescriptor{Name: "ffmpeg", Healthy: false},
	}
	ext := NewExtractor(backend)

	_, err := ext.Run(context.Background(), RunRequest{
		InputPath: "input.ts",
		Backend:   BackendDescriptor{Name: "ffmpeg", Healthy: false},
	})
	if err == nil {
		t.Fatal("expected error for unhealthy backend")
	}
	var typed *model.Error
	if !errors.As(err, &typed) || typed.Code != model.CodeMissingDependency {
		t.Fatalf("expected missing dependency error, got %v", err)
	}
}

func TestRunFailsWhenExtractorNotInitialized(t *testing.T) {
	var ext *Extractor
	_, err := ext.Run(context.Background(), RunRequest{})
	if err == nil {
		t.Fatal("expected error for nil extractor")
	}
}

func TestRunWrapsBackendVersionError(t *testing.T) {
	backend := stubBackend{
		descriptor: BackendDescriptor{Name: "ffmpeg", Healthy: true},
		versionErr: errors.New("version failed"),
	}
	ext := NewExtractor(backend)

	_, err := ext.Run(context.Background(), RunRequest{
		InputPath: "input.ts",
		Backend:   BackendDescriptor{Name: "ffmpeg", Healthy: true},
	})
	if err == nil {
		t.Fatal("expected error when version fails")
	}
}

func TestRunWrapsBackendExtractError(t *testing.T) {
	backend := stubBackend{
		descriptor: BackendDescriptor{Name: "ffmpeg", Healthy: true},
		version:    "7.1",
		extractErr: errors.New("extract failed"),
	}
	ext := NewExtractor(backend)

	_, err := ext.Run(context.Background(), RunRequest{
		InputPath: "input.ts",
		Backend:   BackendDescriptor{Name: "ffmpeg", Healthy: true},
	})
	if err == nil {
		t.Fatal("expected error when extraction fails")
	}
}
