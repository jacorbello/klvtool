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
	PacketIndex        int            `json:"packetIndex"`
	PacketStart        int            `json:"packetStart"`
	KeyStart           int            `json:"keyStart"`
	LengthStart        int            `json:"lengthStart"`
	ValueStart         int            `json:"valueStart"`
	PacketEndExclusive int            `json:"packetEndExclusive"`
	Key                []byte         `json:"key"`
	Length             int            `json:"length"`
	Value              []byte         `json:"value"`
	Classification     Classification `json:"classification"`
	Diagnostics        []Diagnostic   `json:"diagnostics"`
}

// PacketizedStream captures the packetization result for one raw extraction record.
type PacketizedStream struct {
	Source        extract.RawPayloadRecord `json:"source"`
	Mode          Mode                     `json:"mode"`
	ParserVersion string                   `json:"parserVersion"`
	Packets       []Packet                 `json:"packets"`
	Diagnostics   []Diagnostic             `json:"diagnostics"`
	ParsedCount   int                      `json:"parsedCount"`
	WarningCount  int                      `json:"warningCount"`
	ErrorCount    int                      `json:"errorCount"`
	Recovered     bool                     `json:"recovered"`
}
