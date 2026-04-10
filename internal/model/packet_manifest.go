package model

import "encoding/json"

// PacketManifest captures the packet checkpoint output for one extraction record.
type PacketManifest struct {
	SchemaVersion string             `json:"schemaVersion"`
	SourcePath    string             `json:"sourcePath"`
	Records       []PacketCheckpoint `json:"records"`
}

func (m PacketManifest) MarshalJSON() ([]byte, error) {
	type alias PacketManifest
	if m.Records == nil {
		m.Records = []PacketCheckpoint{}
	}
	return json.Marshal(alias(m))
}

// PacketCheckpoint captures one parsed raw record and its packet checkpoint output.
type PacketCheckpoint struct {
	RecordID      string             `json:"recordId"`
	Mode          string             `json:"mode"`
	ParserVersion string             `json:"parserVersion"`
	ParsedCount   int                `json:"parsedCount"`
	WarningCount  int                `json:"warningCount"`
	ErrorCount    int                `json:"errorCount"`
	Recovered     bool               `json:"recovered"`
	PacketPath    string             `json:"packetPath"`
	Diagnostics   []PacketDiagnostic `json:"diagnostics"`
}

func (c PacketCheckpoint) MarshalJSON() ([]byte, error) {
	type alias PacketCheckpoint
	if c.Diagnostics == nil {
		c.Diagnostics = []PacketDiagnostic{}
	}
	return json.Marshal(alias(c))
}

// PacketDiagnostic captures one warning or error emitted while packetizing a raw record.
type PacketDiagnostic struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Stage       string `json:"stage"`
	PacketIndex *int   `json:"packetIndex,omitempty"`
	ByteOffset  *int   `json:"byteOffset,omitempty"`
}
