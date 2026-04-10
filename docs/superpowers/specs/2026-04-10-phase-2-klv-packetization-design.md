# Phase 2 KLV Packetization Design

## Summary

Phase 2 adds a schema-agnostic KLV packetization stage after raw transport extraction. The goal is to parse extracted KLV/data payloads into ordered KLV packets without requiring MISB or other schema knowledge. This stage must be usable on its own, composable with upstream and downstream stages, and able to persist checkpoints for replay and troubleshooting.

The design keeps the existing transport extraction backends focused on MPEG-TS and payload extraction only. Packetization is introduced as a separate stage with its own typed artifacts, persistence format, diagnostics model, and orchestration entrypoints. Future schema decoders will plug into the packetized artifacts rather than directly consuming transport backend output.

## Goals

- Parse extracted payloads into KLV packets using generic KLV rules only.
- Support running each stage independently or as part of a full pipeline.
- Preserve strong troubleshooting and replay support through optional checkpoints at every stage.
- Keep transport extraction, packetization, and schema decoding isolated behind clear interfaces.
- Support both strict validation workflows and best-effort troubleshooting workflows.
- Establish stable contracts that future schema decoders can build on without changing transport logic.

## Non-Goals

- Implement schema-specific decoding such as MISB ST 0601.19 in phase 2.
- Rework transport extraction backends to understand KLV semantics.
- Optimize first for stream-first, low-memory online parsing across all stages.
- Redesign the existing raw extraction manifest unless a minimal compatibility extension is needed.

## Current Context

Today the codebase is structured around:

- CLI command wiring in `internal/cli`
- backend selection and extraction orchestration in `internal/extract`
- backend-specific raw payload extraction in `internal/backends/ffmpeg` and `internal/backends/gstreamer`
- shared output models in `internal/model`
- payload and manifest writers in `internal/output`

The current `extract` flow ends after raw payload extraction and writes payload files plus `manifest.ndjson`. This is a good boundary for phase 1, but phase 2 should not be folded into the backend implementations or the current `Record` model directly because that would blur transport concerns with KLV parsing concerns.

## Recommended Architecture

Introduce a staged pipeline above the current transport backends:

1. `transport extract`
2. `klv packetize`
3. `schema decode`

Each stage must provide:

- a typed in-memory artifact for composed execution
- a serializer/deserializer for persisted checkpoints
- a small interface for orchestration

The canonical execution path should be in-process composition using typed artifacts. Persisted checkpoints are materializations of stage boundaries for replay, troubleshooting, and inspection.

Transport backends remain responsible only for finding and extracting raw KLV/data payloads from MPEG-TS sources. Packetization operates on extracted payload artifacts, not directly on backend internals. Schema decoders operate on packetized artifacts, not on transport-specific records.

## Stage Contracts

### Stage 1: Raw Payload Artifact

Phase 1 already yields transport provenance plus raw payload data. Phase 2 should formalize that output as a named artifact instead of treating the current manifest record as the final cross-stage contract.

Recommended in-memory artifact: `RawPayloadRecord`

Responsibilities:

- hold transport provenance from extraction
- hold payload identity and raw payload content
- carry extraction warnings forward

Expected fields:

- stable record ID
- transport metadata already tracked today such as PID, transport stream ID, packet offset, packet index, continuity counter, PTS, DTS
- payload bytes for in-process composition
- payload identity metadata for persisted replay such as path, size, hash
- warnings

The in-memory object may contain raw bytes while the persisted representation stores path/hash/size and reloads bytes on demand.

### Stage 2: Packetized Artifact

Recommended artifact: `PacketizedStream`

Responsibilities:

- represent the packetization result for one raw payload artifact
- capture parser mode
- store stream-level diagnostics
- contain an ordered list of parsed packets

Expected fields:

- source raw payload artifact reference
- parser mode: `strict` or `best-effort`
- parser metadata and version
- stream-level diagnostics
- ordered `[]Packet`
- summary counts such as parsed packet count, warning count, error count

Recommended packet model: `Packet`

Responsibilities:

- represent one parsed KLV unit
- retain exact byte-level provenance within the raw payload
- carry generic classification only

Expected fields:

- packet index within the payload
- byte offsets within the raw payload for packet start, key start, length start, value start, and packet end
- raw key bytes
- decoded BER length value
- raw value bytes
- generic classification such as `universal_set`, `local_set`, `ber_oid`, or `unknown`
- packet-level diagnostics

Phase 2 must remain schema-agnostic. Generic classification is acceptable; semantic field decoding is not.

### Stage 3: Decoded Artifact

Recommended future artifact: `DecodedStream`

Responsibilities:

- capture the output of one schema decoder run over a packetized artifact
- retain decoder identity and diagnostics

Expected fields:

- source packetized artifact reference
- decoder ID and version
- decoded schema family/version if known
- decoded outputs
- diagnostics

Phase 3 is not implemented here, but phase 2 must be shaped so this artifact can be added without changing the earlier stages.

## Orchestration Model

The pipeline should support both stage-level and composed execution.

Recommended interfaces:

- `Extract(...) -> []RawPayloadRecord`
- `Packetize(...) -> []PacketizedStream`
- `Decode(...) -> []DecodedStream`

And a composed runner that can chain them:

- `RunPipeline(...) -> PipelineResult`

This allows the CLI and library callers to:

- run extraction only
- run packetization over saved raw payload checkpoints
- run decoding over saved packetized checkpoints
- run the full pipeline end-to-end

The phase outputs should be designed so callers can pipe stages together in memory while still choosing to persist none, some, or all intermediate results.

## Checkpointing Strategy

Checkpointing should be supported at every stage but should not be mandatory on every run.

Required behavior:

- support persisting no checkpoints
- support persisting selected stages
- support persisting all stages

This keeps the common path efficient while enabling detailed replay and inspection when troubleshooting problematic videos.

Checkpoint persistence should be explicit policy, not an accidental side effect of calling a stage. The policy belongs in orchestration and CLI configuration, not inside the parser core.

Recommended persisted artifacts:

- raw payload checkpoint: current payload files plus manifest, formalized as a raw payload artifact
- packetized checkpoint: a packet manifest with offsets, keys, BER lengths, classifications, and diagnostics, plus optional raw packet/value dumps if needed later
- decoded checkpoint: future schema-specific decoded output plus decoder metadata

## Parsing Behavior

Phase 2 performs generic KLV parsing only:

1. read key bytes
2. decode BER length
3. slice value bytes
4. repeat until payload end or unrecoverable parse failure

The parser must not require schema knowledge to emit packets. Schema knowledge begins in phase 3.

The initial implementation should optimize for correctness, clear diagnostics, and replayability rather than maximum streaming sophistication.

## Error Handling And Diagnostics

Phase 2 must support both strict and best-effort modes.

### Strict Mode

- stop on the first malformed packet boundary or inconsistent length/value condition
- mark the stream as failed
- return diagnostics precise enough to explain why parsing stopped

Use case:

- validation workflows
- regression testing
- confirming whether a stream is structurally valid

### Best-Effort Mode

- continue parsing where recovery is safe
- emit packet-level and stream-level diagnostics
- clearly mark recovery points and uncertainty

Use case:

- troubleshooting damaged or unusual streams
- extracting maximum information from imperfect payloads

Diagnostics should be structured rather than plain strings. A structured diagnostic should include at least:

- severity
- code
- message
- byte offset or packet index when available
- stage identity

This will allow future CLI and reporting layers to filter, summarize, and render errors consistently.

## Extensibility For Schema Decoders

Phase 2 should prepare for a registry-driven phase 3.

Required future behavior:

- explicit decoder flag wins when provided
- otherwise use auto-detect against registered decoders
- return a structured "no matching decoder" outcome rather than failing packetization

Each future decoder should expose:

- stable decoder ID/name
- matching logic based on packet keys or packetized stream characteristics
- decode entrypoint
- decoder version metadata for persisted output

The decoder registry should be structured so contributors can add new decoders without editing transport backends or packet parser internals.

## Package Shape

The package structure should preserve the existing separation of concerns.

Recommended direction:

- keep transport extraction orchestration in `internal/extract` or evolve it into a broader pipeline orchestrator
- introduce a dedicated packetization package, likely `internal/packetize` or `internal/klv`
- keep persistence writers/readers in `internal/output` unless the number of checkpoint formats justifies a dedicated artifact persistence package later
- keep shared persisted models in `internal/model` only if they are true cross-package output contracts

The main architectural rule is that backend packages must not depend on packetization or schema decoding packages.

## Compatibility Strategy

The current raw extraction output should remain valid during phase 2 work. If the current `manifest.ndjson` format needs adjustment, prefer additive or parallel outputs rather than breaking the existing extraction milestone.

A clean option is:

- keep current extraction output behavior intact
- add phase-2 packet checkpoint output separately
- add new CLI surfaces that can operate from raw checkpoints into packetized checkpoints

This lowers migration risk and keeps verification focused.

## Testing Strategy

Phase 2 should ship with focused unit and pipeline-level tests.

Required coverage areas:

- BER length decoding
- packet boundary detection
- malformed packet handling
- strict versus best-effort divergence
- packet classification behavior
- persisted packet manifest serialization/deserialization
- replay from saved raw payload checkpoints into packetization without requiring transport backends

Test design guidance:

- keep packetization tests independent from `ffmpeg` and `gstreamer`
- use small fixture payloads with known packet boundaries
- add malformed fixtures that force recovery and diagnostics behavior
- prefer table-driven tests for parser mode and error cases

## Tradeoffs

Chosen tradeoff:

- prefer typed stage artifacts plus optional persistence over a filesystem-only boundary

Why:

- better composed performance through in-memory handoff
- cleaner APIs for future decoders
- more reliable replay and testing through explicit contracts

Deferred tradeoff:

- do not start with a fully stream-first iterator boundary

Why:

- it adds complexity around retry, replay, random access, and diagnostics before the tool has proven that payload sizes require it

The design still leaves room to add stream-oriented implementations later behind the same stage interfaces if memory pressure becomes a real problem.

## Open Decisions Deferred To Planning

These items are intentionally left for implementation planning rather than design revision:

- exact package names and file layout
- exact JSON shape for persisted packet checkpoints
- whether packet raw value bytes are always embedded in packet checkpoint files or optionally externalized
- exact CLI surface for running individual stages versus the full pipeline

These are implementation details within the approved architecture, not unresolved design contradictions.

## Recommendation

Implement phase 2 as a separate packetization layer with:

- a `RawPayloadRecord` stage contract
- a `PacketizedStream` and `Packet` stage contract
- strict and best-effort parse modes
- configurable checkpoint persistence across stages
- a future decoder registry seam for explicit override or auto-detection

This preserves the current extraction architecture, gives the tool a strong troubleshooting workflow, and creates a clean foundation for many future schema decoders.
