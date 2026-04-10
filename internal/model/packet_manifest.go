package model

import "encoding/json"

const PacketSchemaVersion = "2"

// PacketManifest captures the packet checkpoint output for one extraction record.
type PacketManifest struct {
	SchemaVersion string                `json:"schemaVersion"`
	SourcePath    string                `json:"sourcePath"`
	Records       []PacketManifestEntry `json:"records"`
}

func (m PacketManifest) MarshalJSON() ([]byte, error) {
	type alias PacketManifest
	if m.Records == nil {
		m.Records = []PacketManifestEntry{}
	}
	return json.Marshal(alias(m))
}

// PacketManifestEntry captures the per-record manifest summary for a packet checkpoint.
type PacketManifestEntry struct {
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

func (e PacketManifestEntry) MarshalJSON() ([]byte, error) {
	type alias PacketManifestEntry
	if e.Diagnostics == nil {
		e.Diagnostics = []PacketDiagnostic{}
	}
	return json.Marshal(alias(e))
}

// PacketCheckpoint captures one parsed raw record and its packet checkpoint output.
type PacketCheckpoint struct {
	SchemaVersion string             `json:"schemaVersion"`
	RecordID      string             `json:"recordId"`
	Mode          string             `json:"mode"`
	ParserVersion string             `json:"parserVersion"`
	ParsedCount   int                `json:"parsedCount"`
	WarningCount  int                `json:"warningCount"`
	ErrorCount    int                `json:"errorCount"`
	Recovered     bool               `json:"recovered"`
	Packets       []PacketRecord     `json:"packets"`
	Diagnostics   []PacketDiagnostic `json:"diagnostics"`
}

func (c PacketCheckpoint) MarshalJSON() ([]byte, error) {
	type alias PacketCheckpoint
	if c.SchemaVersion == "" {
		c.SchemaVersion = PacketSchemaVersion
	}
	if c.Packets == nil {
		c.Packets = []PacketRecord{}
	}
	if c.Diagnostics == nil {
		c.Diagnostics = []PacketDiagnostic{}
	}
	return json.Marshal(alias(c))
}

// PacketRecord captures one parsed packet in a packet checkpoint.
type PacketRecord struct {
	PacketIndex    int                `json:"packetIndex"`
	PacketStart    int                `json:"packetStart"`
	KeyStart       int                `json:"keyStart"`
	LengthStart    int                `json:"lengthStart"`
	ValueStart     int                `json:"valueStart"`
	PacketEnd      int                `json:"packetEnd"`
	RawKeyHex      string             `json:"rawKeyHex"`
	Length         int                `json:"length"`
	RawValueHex    string             `json:"rawValueHex"`
	Classification string             `json:"classification"`
	Diagnostics    []PacketDiagnostic `json:"diagnostics"`
}

func (r PacketRecord) MarshalJSON() ([]byte, error) {
	type alias PacketRecord
	if r.Diagnostics == nil {
		r.Diagnostics = []PacketDiagnostic{}
	}
	return json.Marshal(alias(r))
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
