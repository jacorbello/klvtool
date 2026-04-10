package extract

import (
	"context"
	"errors"
	"fmt"

	"github.com/jacorbello/klvtool/internal/model"
)

// BackendName is the normalized identifier for an extraction backend.
type BackendName string

const (
	BackendAuto      BackendName = "auto"
	BackendGStreamer BackendName = "gstreamer"
	BackendFFmpeg    BackendName = "ffmpeg"
)

// BackendDescriptor is the normalized, extract-layer view of a backend health report.
type BackendDescriptor struct {
	Name    BackendName
	Healthy bool
	Tools   []string
}

// PayloadRecord captures one extracted payload and the transport metadata needed
// to persist it into the manifest/output layer.
type PayloadRecord struct {
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

// RunRequest captures the input path, backend preference, and validated backend
// catalog used for backend resolution.
type RunRequest struct {
	InputPath string
	Backend   BackendName
	Backends  []BackendDescriptor
}

// RunResult captures the selected backend, resolved version, and extracted payloads.
type RunResult struct {
	Selected       BackendDescriptor
	Backends       []BackendDescriptor
	BackendVersion string
	Records        []PayloadRecord
}

// Backend is the minimal interface an extraction backend must satisfy for orchestration.
type Backend interface {
	Descriptor() BackendDescriptor
	Version(context.Context) (string, error)
	Extract(context.Context, string) ([]PayloadRecord, error)
}

// Selector chooses a backend for a request.
type Selector interface {
	Select(ExtractionRequest) (ExtractionResponse, error)
}

// ExtractionRequest captures the backend preference and backend catalog used for selection.
type ExtractionRequest struct {
	Backend  BackendName
	Backends []BackendDescriptor
}

// ExtractionResponse captures the selected backend and the normalized backend catalog.
type ExtractionResponse struct {
	Selected BackendDescriptor
	Backends []BackendDescriptor
}

// Extractor orchestrates backend selection and invocation.
type Extractor struct {
	Selector Selector
	Backends map[BackendName]Backend
}

// NewExtractor constructs an extractor backed by the provided backend implementations.
func NewExtractor(backends ...Backend) *Extractor {
	registry := make(map[BackendName]Backend, len(backends))
	for _, backend := range backends {
		if backend == nil {
			continue
		}
		descriptor, ok := normalizeBackendDescriptor(backend.Descriptor())
		if !ok {
			continue
		}
		registry[descriptor.Name] = backend
	}

	return &Extractor{
		Selector: NewSelector(),
		Backends: registry,
	}
}

// Run chooses a backend, resolves its version, and extracts payload records.
func (e *Extractor) Run(ctx context.Context, req RunRequest) (RunResult, error) {
	var result RunResult

	if e == nil {
		return result, model.BackendExecution(fmt.Errorf("extractor is not initialized"))
	}

	selector := e.Selector
	if selector == nil {
		selector = NewSelector()
	}

	selection, err := selector.Select(ExtractionRequest{
		Backend:  req.Backend,
		Backends: req.Backends,
	})
	if err != nil {
		switch {
		case isUnsupportedBackendError(err):
			return result, model.UnsupportedBackend(err)
		default:
			return result, model.MissingDependency(err)
		}
	}

	backend, ok := e.Backends[selection.Selected.Name]
	if !ok {
		return result, model.UnsupportedBackend(fmt.Errorf("backend implementation unavailable: %s", selection.Selected.Name))
	}

	version, err := backend.Version(ctx)
	if err != nil {
		return result, wrapBackendError(err)
	}

	records, err := backend.Extract(ctx, req.InputPath)
	if err != nil {
		return result, wrapBackendError(err)
	}

	result.Selected = selection.Selected
	result.Backends = selection.Backends
	result.BackendVersion = version
	result.Records = CanonicalizeRecords(records)
	return result, nil
}

func isUnsupportedBackendError(err error) bool {
	return errors.Is(err, ErrUnsupportedBackend)
}

func wrapBackendError(err error) error {
	if err == nil {
		return nil
	}
	var typed *model.Error
	if containsModelError(err, &typed) {
		return err
	}
	return model.BackendExecution(err)
}

func containsModelError(err error, target **model.Error) bool {
	return errors.As(err, target)
}
