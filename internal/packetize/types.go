package packetize

import "github.com/jacorbello/klvtool/internal/extract"

// Mode describes how packetization handles malformed data.
type Mode string

const (
	ModeStrict     Mode = "strict"
	ModeBestEffort Mode = "best-effort"
)

// Classification identifies the broad shape of a parsed KLV packet.
type Classification string

const (
	ClassificationUnknown      Classification = "unknown"
	ClassificationUniversalSet Classification = "universal_set"
	ClassificationLocalSet     Classification = "local_set"
	ClassificationBEROID       Classification = "ber_oid"
)

// Diagnostic describes a packetization warning or error.
type Diagnostic struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Stage       string `json:"stage"`
	PacketIndex *int   `json:"packetIndex,omitempty"`
	ByteOffset  *int   `json:"byteOffset,omitempty"`
}

// Packet captures a parsed KLV packet and its byte provenance.
type Packet struct {
	PacketIndex    int
	PacketStart    int
	KeyStart       int
	LengthStart    int
	ValueStart     int
	PacketEnd      int
	Key            []byte
	Length         int
	Value          []byte
	Classification Classification
	Diagnostics    []Diagnostic
}

// PacketizedStream captures the packetization result for one raw extraction record.
type PacketizedStream struct {
	Source        extract.RawPayloadRecord
	Mode          Mode
	ParserVersion string
	Packets       []Packet
	Diagnostics   []Diagnostic
	ParsedCount   int
	WarningCount  int
	ErrorCount    int
	Recovered     bool
}
