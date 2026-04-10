// Package ts parses MPEG-TS (ISO 13818-1) transport stream packets.
//
// It provides a streaming packet scanner, PAT/PMT parsing for stream
// discovery, and PES reassembly with PTS/DTS extraction. The parser
// reads 188-byte packets sequentially with constant memory usage.
//
// Key entry points:
//   - NewPacketScanner: sequential packet reading with PID-based payload filtering
//   - DiscoverStreams: one-call PAT/PMT parsing for stream inventory
//   - NewPESAssembler: reassembles TS packets into PES units with timing metadata
//   - EnrichRecords: attaches TS-level metadata to extract pipeline records
package ts
