package extract

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacorbello/klvtool/internal/envcheck"
)

var (
	ErrNoHealthyBackend   = errors.New("no healthy backend available")
	ErrUnsupportedBackend = errors.New("unsupported backend")
	ErrBackendUnavailable = errors.New("requested backend is unavailable")
)

// NewSelector returns the default backend selector.
func NewSelector() Selector {
	return backendSelector{}
}

type backendSelector struct{}

// Select chooses the backend that should handle an extraction request.
func (backendSelector) Select(req ExtractionRequest) (ExtractionResponse, error) {
	return SelectBackend(req)
}

// SelectBackend chooses the requested backend or resolves the best healthy backend for auto mode.
func SelectBackend(req ExtractionRequest) (ExtractionResponse, error) {
	catalog, backends := normalizedBackends(req.Report)

	requested := normalizeBackendName(req.Backend)
	switch requested {
	case BackendAuto:
		if backend, ok := pickHealthyBackend(backends, BackendGStreamer); ok {
			return ExtractionResponse{Selected: backend, Backends: catalog}, nil
		}
		if backend, ok := pickHealthyBackend(backends, BackendFFmpeg); ok {
			return ExtractionResponse{Selected: backend, Backends: catalog}, nil
		}
		return ExtractionResponse{Backends: catalog}, fmt.Errorf("%w: auto", ErrNoHealthyBackend)
	case BackendGStreamer, BackendFFmpeg:
		backend, ok := backends[requested]
		if !ok {
			return ExtractionResponse{Backends: catalog}, fmt.Errorf("%w: %s", ErrBackendUnavailable, requested)
		}
		if !backend.Healthy {
			return ExtractionResponse{Backends: catalog}, fmt.Errorf("%w: %s", ErrBackendUnavailable, requested)
		}
		return ExtractionResponse{Selected: backend, Backends: catalog}, nil
	default:
		return ExtractionResponse{Backends: catalog}, fmt.Errorf("%w: %s", ErrUnsupportedBackend, req.Backend)
	}
}

func normalizedBackends(report envcheck.Report) ([]BackendDescriptor, map[BackendName]BackendDescriptor) {
	catalog := make([]BackendDescriptor, 0, len(report.Backends))
	backends := make(map[BackendName]BackendDescriptor, len(report.Backends))
	for _, backend := range report.Backends {
		name := normalizeBackendName(BackendName(backend.Name))
		if name == "" {
			continue
		}
		descriptor := BackendDescriptor{
			Name:    name,
			Healthy: backend.Healthy,
			Tools:   backendToolNames(backend.Tools),
		}
		catalog = append(catalog, descriptor)
		backends[name] = descriptor
	}
	return catalog, backends
}

func pickHealthyBackend(backends map[BackendName]BackendDescriptor, name BackendName) (BackendDescriptor, bool) {
	backend, ok := backends[name]
	if !ok || !backend.Healthy {
		return BackendDescriptor{}, false
	}
	return backend, true
}

func backendToolNames(tools []envcheck.ToolHealth) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		names = append(names, tool.Name)
	}
	return names
}

func normalizeBackendName(name BackendName) BackendName {
	switch strings.ToLower(strings.TrimSpace(string(name))) {
	case "", string(BackendAuto):
		return BackendAuto
	case string(BackendGStreamer):
		return BackendGStreamer
	case string(BackendFFmpeg):
		return BackendFFmpeg
	default:
		return BackendName(strings.ToLower(strings.TrimSpace(string(name))))
	}
}
