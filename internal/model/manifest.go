package model

// Manifest captures the shared JSON output for an extraction run.
type Manifest struct {
	SchemaVersion   string   `json:"schemaVersion"`
	SourceInputPath string   `json:"sourceInputPath"`
	BackendName     string   `json:"backendName"`
	BackendVersion  string   `json:"backendVersion"`
	Records         []Record `json:"records"`
}

// Record captures one extracted payload and the transport metadata needed to
// trace it back to the source stream.
type Record struct {
	RecordID          string   `json:"recordId"`
	PID               uint16   `json:"pid"`
	TransportStreamID *uint16  `json:"transportStreamId,omitempty"`
	PacketOffset      *int64   `json:"packetOffset,omitempty"`
	PacketIndex       *int64   `json:"packetIndex,omitempty"`
	ContinuityCounter *uint8   `json:"continuityCounter,omitempty"`
	PTS               *int64   `json:"pts,omitempty"`
	DTS               *int64   `json:"dts,omitempty"`
	PayloadPath       string   `json:"payloadPath"`
	PayloadSize       int64    `json:"payloadSize"`
	PayloadHash       string   `json:"payloadHash"`
	Warnings          []string `json:"warnings"`
}
