package extract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacorbello/klvtool/internal/model"
)

// BackendDescriptor is the normalized, extract-layer view of a backend health report.
type BackendDescriptor struct {
	Name    string
	Healthy bool
	Tools   []string
}

// RawPayloadRecord captures one extracted payload and the transport metadata
// needed to persist it into the manifest/output layer.
type RawPayloadRecord struct {
	RecordID          string
	PID               uint16
	TransportStreamID *uint16
	PacketOffset      *int64
	PacketIndex       *int64
	ContinuityCounter *uint8
	PTS               *int64
	DTS               *int64
	Payload           []byte
	Warnings          []string
}

// PayloadRecord is kept as a compatibility alias for existing callers.
type PayloadRecord = RawPayloadRecord

// RunRequest captures the input path and validated backend descriptor.
type RunRequest struct {
	InputPath string
	Backend   BackendDescriptor
}

// RunResult captures the backend, resolved version, and extracted payloads.
type RunResult struct {
	Backend        BackendDescriptor
	BackendVersion string
	Records        []PayloadRecord
}

// Backend is the minimal interface an extraction backend must satisfy for orchestration.
type Backend interface {
	Descriptor() BackendDescriptor
	Version(context.Context) (string, error)
	Extract(context.Context, string) ([]PayloadRecord, error)
}

// Extractor orchestrates backend invocation.
type Extractor struct {
	backend Backend
}

// NewExtractor constructs an extractor backed by the provided backend implementation.
func NewExtractor(backend Backend) *Extractor {
	return &Extractor{backend: backend}
}

// Run resolves the backend version and extracts payload records.
func (e *Extractor) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	var result RunResult

	if e == nil || e.backend == nil {
		return result, model.BackendExecution(fmt.Errorf("extractor is not initialized"))
	}

	if !req.Backend.Healthy {
		return result, model.MissingDependency(fmt.Errorf("backend is unavailable: %s", req.Backend.Name))
	}

	version, err := e.backend.Version(ctx)
	if err != nil {
		return result, wrapBackendError(err)
	}

	records, err := e.backend.Extract(ctx, req.InputPath)
	if err != nil {
		return result, wrapBackendError(err)
	}

	result.Backend = req.Backend
	result.BackendVersion = version
	result.Records = CanonicalizeRecords(records)
	return result, nil
}

func wrapBackendError(err error) error {
	if err == nil {
		return nil
	}
	var typed *model.Error
	if errors.As(err, &typed) {
		return err
	}
	return model.BackendExecution(err)
}
