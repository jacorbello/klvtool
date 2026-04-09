package extract

import (
	"errors"
	"fmt"
	"strings"
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
	catalog, backends := normalizedBackends(req.Backends)

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

func normalizedBackends(backends []BackendDescriptor) ([]BackendDescriptor, map[BackendName]BackendDescriptor) {
	catalog := make([]BackendDescriptor, 0, len(backends))
	index := make(map[BackendName]BackendDescriptor, len(backends))
	for _, backend := range backends {
		descriptor, ok := normalizeBackendDescriptor(backend)
		if !ok {
			continue
		}
		if _, exists := index[descriptor.Name]; exists {
			continue
		}
		catalog = append(catalog, descriptor)
		index[descriptor.Name] = descriptor
	}
	return catalog, index
}

func pickHealthyBackend(backends map[BackendName]BackendDescriptor, name BackendName) (BackendDescriptor, bool) {
	backend, ok := backends[name]
	if !ok || !backend.Healthy {
		return BackendDescriptor{}, false
	}
	return backend, true
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

func normalizeBackendDescriptor(backend BackendDescriptor) (BackendDescriptor, bool) {
	name, ok := normalizeCatalogBackendName(backend.Name)
	if !ok {
		return BackendDescriptor{}, false
	}
	backend.Name = name
	return backend, true
}

func normalizeCatalogBackendName(name BackendName) (BackendName, bool) {
	switch strings.ToLower(strings.TrimSpace(string(name))) {
	case string(BackendGStreamer):
		return BackendGStreamer, true
	case string(BackendFFmpeg):
		return BackendFFmpeg, true
	default:
		return "", false
	}
}
