# GStreamer Extraction Parity Design

Date: 2026-04-09
Status: Proposed

## Summary

Implement a real GStreamer extraction path that is interchangeable with FFmpeg at the `klvtool extract` boundary. Backend choice must not change the CLI contract, manifest semantics, payload outputs, warning behavior, or error classification for the same input and extraction mode.

The immediate target remains stream-level extraction. The design must also preserve a clean path to future packet-level extraction and MISB/ST parsing without redefining the user-facing contract or forcing a second backend divergence later.

Implementation must follow true TDD. New parity assertions and failing tests are written first, then backend and normalization code is added until the tests pass.

## Goals

- Make `--backend ffmpeg` and `--backend gstreamer` observably interchangeable for stream-level extraction.
- Keep backend-specific complexity inside backend adapters and extract-layer normalization.
- Define a canonical extraction model that is richer than the current FFmpeg implementation so both backends can converge on a shared long-term contract.
- Preserve a forward path to packet-level extraction and MISB/ST standard or schema-based parsing.
- Enforce parity with automated tests instead of informal behavior matching.

## Non-Goals

- Implement packet-level extraction in this milestone.
- Implement MISB/ST parsing in this milestone.
- Add backend-specific CLI flags or backend-specific manifest variants.
- Build a full Go-native MPEG-TS parser as part of the initial GStreamer parity work.

## Design Principles

- Observable parity matters more than internal similarity.
- Backend differences are acceptable internally but must be normalized before reaching CLI or output layers.
- The canonical model should represent the intended product contract, not just the current FFmpeg output shape.
- Missing metadata must be represented consistently across backends.
- Test-first development is required for every behavior change.

## Canonical Extraction Contract

`internal/extract` owns the canonical extraction contract. Backend implementations adapt their tool-specific behavior into the same normalized `RunResult` and `PayloadRecord` outputs.

For the same input and extraction mode, both backends should aim to produce:

- The same number of records.
- The same record ordering.
- The same `RecordID` values.
- The same payload bytes.
- The same manifest field meanings and serialized structure.
- The same warning semantics where the source data supports them.
- The same typed error category for equivalent failures.

The default extraction mode remains stream-level. One extracted record represents one normalized extracted stream artifact. The internal model must not assume this is the final semantic unit of the product, because packet-level extraction will be added later.

## Architecture

The existing high-level flow remains:

`CLI -> extract orchestrator -> backend -> output writers`

The change is that `extract` becomes the explicit parity boundary rather than a thin pass-through for backend outputs.

### Extract Layer Responsibilities

- Select the backend using existing `auto`, `ffmpeg`, and `gstreamer` behavior.
- Define the canonical extraction output requirements.
- Normalize backend descriptors and backend selection output.
- Provide shared rules for deterministic ordering, stable record ID assignment, and missing metadata handling.
- Ensure equivalent backend failures surface as the same `model.Error` codes.

### Backend Responsibilities

Each backend should conceptually implement the same stages, even if the code initially stays in a single package or file:

- Discovery: identify relevant MPEG-TS data streams and enough source metadata to support stable ordering.
- Extraction: materialize payload bytes for each discovered stream.
- Normalization: convert backend-native probe or extraction results into canonical `PayloadRecord` values.

This staging must exist for both FFmpeg and GStreamer. FFmpeg is already implemented, but its current behavior should be treated as incomplete relative to the new canonical contract.

### Output Layer Responsibilities

`internal/cli` and `internal/output` remain backend-agnostic. They consume canonical extraction results only. No backend-specific branches should be introduced in CLI status output, manifest writing, or payload writing logic.

## GStreamer Backend Design

GStreamer may require a more complex implementation than FFmpeg. That complexity is acceptable if it stays local to the backend package and preserves interchangeability.

The planned GStreamer backend shape is:

1. A discovery path using GStreamer tooling to identify candidate data streams and stable source identifiers.
2. An extraction path that runs one or more GStreamer pipelines to dump the selected data stream payloads into temporary files or buffers.
3. A normalization path that converts discovered stream metadata and extracted bytes into canonical `PayloadRecord` values.

The exact pipeline syntax is an implementation detail. The design requirement is that the backend can produce the same observable outputs as FFmpeg for the same fixture inputs.

If a single pipeline cannot deliver reliable discovery and extraction parity, a multi-step GStreamer workflow is acceptable.

## FFmpeg Backend Alignment

FFmpeg should not remain the de facto specification. As the canonical extraction contract is introduced, FFmpeg may need follow-up changes so both backends converge on the same normalized behavior.

Likely FFmpeg alignment areas include:

- Deterministic ordering rules made explicit rather than implicit.
- Normalized warning strings and missing metadata handling.
- Shared helper logic for record ID assignment and canonical field population.
- Richer metadata support later, if added to the canonical contract.

## Data Model Direction

The current `PayloadRecord` shape is the starting point for the canonical model. It already includes fields for transport and timing metadata such as:

- `PID`
- `TransportStreamID`
- `PacketOffset`
- `PacketIndex`
- `ContinuityCounter`
- `PTS`
- `DTS`
- `Warnings`

For this milestone:

- Stream-level extraction must be fully supported.
- Fields unavailable from a backend must remain unset in a consistent way.
- Backends must not invent different fallback conventions for missing values.

For future work:

- Packet-level extraction should be able to coexist with stream-level extraction.
- MISB/ST parsing should build on normalized extracted data rather than requiring a redesign of the extraction contract.

Naming and interfaces should avoid implying that one extracted stream is the final semantic object of the system.

## Error Handling

Equivalent failures must map to the same typed `model.Error` categories regardless of backend:

- `CodeInvalidUsage` for invalid inputs or unsupported modes.
- `CodeMissingDependency` for unavailable requested backends or missing required tools.
- `CodeBackendParse` when backend output cannot be interpreted into the canonical model.
- `CodeBackendExecution` when a backend tool runs and extraction cannot complete.
- `CodeOutputWrite` when writing payloads or manifest output fails.

Backend packages may wrap tool-specific errors, but by the time errors reach `extract.Run` and the CLI, equivalent scenarios should be indistinguishable except for backend-identifying context in the message text where useful.

## Testing Strategy

Implementation must follow true TDD:

1. Add or update failing tests that describe the intended parity behavior.
2. Implement the smallest change necessary to satisfy the new test.
3. Refactor while preserving green tests.

This applies to backend behavior, normalization rules, and CLI-observable parity.

### Required Test Layers

- Backend unit tests:
  Validate command construction, backend output parsing, normalization, warning behavior, and error mapping for FFmpeg and GStreamer independently.

- Extract-layer parity tests:
  Feed equivalent synthetic backend-native results through both backend adapters or normalization seams and assert identical canonical `PayloadRecord` outputs.

- Integration tests:
  Run `klvtool extract` against the same fixture with `--backend ffmpeg` and `--backend gstreamer`, then compare manifest content and payload bytes for equivalence.

### Test Expectations

- Record ordering must be asserted explicitly.
- `RecordID` values must be asserted explicitly.
- Payload byte equivalence must be asserted explicitly.
- Warning equivalence must be asserted explicitly.
- Error code equivalence must be asserted explicitly for matching failure scenarios.

Parity is not considered implemented until tests compare both backends directly.

## Implementation Constraints

- Keep new code inside existing package boundaries unless a clear shared helper package becomes necessary.
- Preserve the current CLI surface and output file layout.
- Avoid machine-specific assumptions about installed tool paths.
- Prefer small, focused helpers over broad refactors unrelated to parity.

## Open Decisions Resolved In This Design

- Backend parity target: strict interchangeability at the observable tool boundary.
- Canonical milestone target: richer shared contract, not merely copying current FFmpeg behavior.
- Canonical extraction unit today: stream-level by default, with packet-level support planned later.
- GStreamer complexity tolerance: higher backend-local complexity is acceptable when required for parity.
- Development methodology: true TDD is mandatory.

## Implementation Outline For Planning

The implementation plan should break work into small TDD increments:

1. Define canonical parity assertions and add failing extractor or integration tests.
2. Introduce shared normalization helpers for deterministic ordering, record IDs, warnings, and missing metadata semantics.
3. Refine the FFmpeg backend to conform to the explicit canonical rules where needed.
4. Implement GStreamer discovery and extraction behind the same canonical normalization rules.
5. Add direct cross-backend integration comparisons on shared fixtures.
6. Reserve clear extension points for future packet-level extraction and MISB/ST parsing without exposing them in the CLI yet.

## Success Criteria

The design is successful when:

- Running `klvtool extract` with FFmpeg or GStreamer on the same supported input produces equivalent manifests and payload outputs.
- Auto-selection can continue preferring GStreamer without making behavior less predictable than explicit FFmpeg execution.
- Parity is defended by tests at unit, extract, and integration levels.
- The resulting design can add packet-level extraction and MISB/ST parsing without reworking the CLI or manifest contract.
