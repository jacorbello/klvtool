# klvtool

`klvtool` is a Go CLI for working with MPEG-TS video assets and KLV payloads.

Commands:

- `klvtool version` prints the embedded version; use `klvtool version --check` to compare against the latest GitHub release.
- `klvtool update` updates an installed binary using `go install` when possible, or by downloading the matching release archive (see [Installation](#installation)).
- `klvtool doctor` checks local `ffmpeg` availability, reports detected versions, and prints install guidance.
- `klvtool inspect` inspects an MPEG-TS file and reports stream inventory, packet counts, PES timing, and transport diagnostics.
- `klvtool extract` validates backend health, extracts KLV/data payloads using `ffmpeg`, and writes extracted payloads plus a JSON manifest line to `manifest.ndjson`.
- `klvtool packetize` replays a raw extraction checkpoint directory and writes packet checkpoint output plus a packet manifest.
- `klvtool decode` decodes MISB ST 0601.19 KLV from an MPEG-TS file into typed records with structural and per-tag validation.

## Installation

Install a **release build** (no Go toolchain required) or use **`go install`** if you already have Go1.23+.

### From GitHub releases

1. Open [releases](https://github.com/jacorbello/klvtool/releases) and download the archive for your OS and CPU:

   | OS | CPU | File |
   |----|-----|------|
   | Linux | x86_64 | `klvtool_linux_amd64.tar.gz` |
   | Linux | arm64 | `klvtool_linux_arm64.tar.gz` |
   | macOS | Intel | `klvtool_darwin_amd64.tar.gz` |
   | macOS | Apple Silicon | `klvtool_darwin_arm64.tar.gz` |
   | Windows | x86_64 | `klvtool_windows_amd64.zip` |
   | Windows | arm64 | `klvtool_windows_arm64.zip` |

2. Verify the download (optional but recommended): the release includes `checksums.txt` with SHA256 sums for each asset.

3. Extract the archive. The `klvtool` executable (or `klvtool.exe` on Windows) is at the top level of the archive.

4. Put the binary on your `PATH` (or run it from a fixed directory). On Unix you may need `chmod +x klvtool` if permissions were lost when copying.

5. Confirm it runs:

   ```bash
   klvtool version
   klvtool doctor
   ```

### With Go

If Go is installed and `GOBIN` (or `GOPATH/bin`) is on your `PATH`:

```bash
go install github.com/jacorbello/klvtool/cmd/klvtool@latest
```

Use a specific tag instead of `@latest` if you need a pinned version.

### Updating an installed binary

- **`klvtool update`** fetches the latest release from GitHub. If `go` is on your `PATH`, it runs `go install` for the new tag; otherwise it downloads the release archive that matches your OS and CPU, verifies `checksums.txt`, and replaces the running binary (on Unix) or writes `klvtool.exe.new` next to the current executable on Windows so you can swap files after closing any running instances.
- **`klvtool update --dry-run`** prints what would happen without changing anything.
- **`klvtool update --prefer-binary`** forces the release-archive path even when `go` is available.
- Release builds embed a version string; **`dev`** checkouts skip the update command (same as `version --check`).

## Usage

```bash
make test
make build
./bin/klvtool doctor
./bin/klvtool inspect --input testdata/fixtures/sample.ts
./bin/klvtool extract --input testdata/fixtures/sample.ts --out /tmp/klvtool-raw
./bin/klvtool packetize --input /tmp/klvtool-raw --out /tmp/klvtool-packets --mode best-effort
./bin/klvtool decode --input testdata/fixtures/sample.ts --format ndjson
./bin/klvtool decode --input testdata/fixtures/sample.ts --format text --raw
./bin/klvtool decode --input testdata/fixtures/sample.ts --format csv --out /tmp/decode.csv
```

### Decode flags

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | (required) | Path to MPEG-TS input file |
| `--format` | `ndjson` | Output format: `ndjson`, `text`, or `csv` |
| `--raw` | `false` | Include raw bytes (base64 in NDJSON; hex `0x...` in text and CSV) and units per item in text mode; in CSV, units are always included and `--raw` adds a `raw` column |
| `--strict` | `false` | Exit 1 if any error-severity diagnostic is emitted |
| `--pid` | `0` (all) | Limit to a specific KLV data stream PID |
| `--out` | stdout | Write output to a file instead of stdout |
| `--schema` | (auto) | Override auto-detection with a specific spec URN |

The `testdata/fixtures/sample.ts` path is a local fixture reference. If that file is not present in your checkout, use any equivalent MPEG-TS sample that exposes KLV payloads for the extract and packetize smoke flow.

## Troubleshooting Source Issues

When you are diagnosing a bad source file, use the commands in this order:

1. `klvtool doctor` to verify the local runtime.
2. `klvtool inspect` to see whether the MPEG-TS structure looks sane.
3. `klvtool decode` to validate KLV parsing and per-tag semantics.
4. `klvtool extract` to capture raw payload artifacts.
5. `klvtool packetize` to isolate KLV packet framing and recovery behavior.

### Scenario: `ffmpeg` or `ffprobe` is missing

If the tool fails before it even reads the transport stream, check the local backend first:

```bash
klvtool doctor
```

Use this when `extract` or `decode` reports a missing dependency or backend failure. If `doctor` reports an unhealthy or missing `ffmpeg` backend, fix the local install before spending time on the source file itself.

### Scenario: Confirm whether the MPEG-TS contains metadata streams

Start with a structural scan of the transport stream:

```bash
klvtool inspect --input <input.ts>
```

This tells you whether the file exposes likely metadata-bearing PIDs such as `type=0x06 (Private Data)` or `type=0x15 (Metadata PES)`, how many packets were seen on each PID, and whether there are transport-level diagnostics such as continuity gaps. If you do not see any plausible metadata/data stream, the problem may be the source asset rather than the KLV parser.

### Scenario: Find the right PID before decoding

Use `inspect` to identify a likely metadata PID, then decode only that stream:

```bash
klvtool inspect --input <input.ts>
klvtool decode --input <input.ts> --pid <pid> --format text
```

This is the fastest way to separate “the file has multiple data streams” from “this specific PID contains decodable KLV.” If decoding succeeds only when pinned to one PID, keep using `--pid` for the rest of your analysis and issue reports.

### Scenario: Investigate parsing or semantic validation failures

Ask `decode` for the most operator-friendly view and make diagnostics affect the exit code:

```bash
klvtool decode --input <input.ts> --format text --raw --strict
```

Use this when you need to understand why a source does not decode cleanly. `--format text` makes diagnostics easier to read in the terminal, `--raw` includes raw bytes and units for each item, and `--strict` exits with code `1` if any error-severity diagnostic is emitted. This is the right path when the stream exists but fields look malformed, out of range, or semantically inconsistent.

### Scenario: Capture machine-readable decode output for analysis

If you need to diff records, feed another tool, or attach structured evidence to a bug report, write NDJSON:

```bash
klvtool decode --input <input.ts> --format ndjson --out /tmp/decode.ndjson
```

This keeps the decoded record stream in a form that is easy to grep, archive, or post-process. It is useful when the source is intermittently bad and you need a durable artifact instead of terminal output.

For spreadsheet or Python workflows, use CSV (long/tidy layout: one row per packet and tag, with `packetIndex` grouping rows that belong to the same decoded packet):

```bash
klvtool decode --input <input.ts> --format csv --out /tmp/decode.csv
```

### Scenario: Separate extraction issues from KLV packet parsing issues

When you need raw artifacts, extract first and then packetize them explicitly:

```bash
klvtool extract --input <input.ts> --out /tmp/klvtool-raw
klvtool packetize --input /tmp/klvtool-raw --out /tmp/klvtool-packets --mode strict
```

If `extract` fails, the issue is likely at the transport or backend layer. If `extract` succeeds but `packetize` fails in `strict` mode, the source likely contains malformed or incomplete KLV framing. Re-run in best-effort mode to see whether recovery is possible:

```bash
klvtool packetize --input /tmp/klvtool-raw --out /tmp/klvtool-packets --mode best-effort
```

This workflow is the most useful one to preserve when filing a bug because it captures the raw payloads, the extraction manifest, and the packetization result as separate checkpoints.

## Dependencies

FFmpeg is required as the extraction backend.

### FFmpeg

| Tool | Purpose |
|------|---------|
| `ffmpeg` | Media demuxing and payload extraction |
| `ffprobe` | Stream discovery and metadata inspection |

| Platform | Install |
|----------|---------|
| macOS (Homebrew) | `brew install ffmpeg` |
| Debian / Ubuntu / WSL | `sudo apt update && sudo apt install ffmpeg` |
| Other | <https://ffmpeg.org/download.html> |

### Verifying the installation

Run `klvtool doctor` to check that all required tools are detected:

```bash
klvtool doctor
```

## Development

- `make fmt` runs `go fmt ./...`
- `make lint` runs `golangci-lint`
- `make test` runs unit and package tests
- `make test-integration` runs integration coverage and skips extraction if fixtures or backends are absent
- `make build` builds `./bin/klvtool`
