# Bootstrap Architecture

`klvtool` is structured as a small CLI-first Go module with narrow internal packages:

- `internal/cli` owns argument parsing, usage text, and command execution.
- `internal/envcheck` detects `ffmpeg` tools plus platform-specific install guidance.
- `internal/extract` orchestrates extraction.
- `internal/backends/ffmpeg` isolates command construction and backend-specific behavior.
- `internal/output` writes payload files and `manifest.ndjson`.
- `internal/model` contains shared manifest and typed error models.

## Current Milestone

The bootstrap scope is intentionally narrow:

- `klvtool doctor` reports backend health.
- `klvtool extract` supports `--input`, `--out`, and `--backend ffmpeg`.
- `ffmpeg` is the sole extraction backend.

## Extraction Flow

1. `klvtool extract` validates flags and runs environment detection.
2. The CLI converts backend health into a normalized descriptor.
3. `internal/extract` resolves the backend version.
4. The backend extracts payload bytes.
5. `internal/output` writes payload files under `payloads/` and writes a single manifest JSON line to `manifest.ndjson`.

The manifest includes source path, backend name, backend version, and normalized record metadata. Missing transport metadata is left unset when possible, and backend warnings are preserved instead of guessed away.
