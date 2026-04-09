package extract

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

// Backend is the minimal interface an extraction backend must satisfy for orchestration.
type Backend interface {
	Descriptor() BackendDescriptor
}

// Selector chooses a backend for a request.
type Selector interface {
	Select(ExtractionRequest) (ExtractionResponse, error)
}
