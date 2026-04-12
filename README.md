# klvtool

`klvtool` is a Go CLI for working with MPEG-TS video assets and KLV payloads.

Commands:

- `klvtool doctor` checks local `ffmpeg` availability, reports detected versions, and prints install guidance.
- `klvtool extract` validates backend health, extracts KLV/data payloads using `ffmpeg`, and writes extracted payloads plus a JSON manifest line to `manifest.ndjson`.
- `klvtool packetize` replays a raw extraction checkpoint directory and writes packet checkpoint output plus a packet manifest.
- `klvtool decode` decodes MISB ST 0601.19 KLV from an MPEG-TS file into typed records with structural and per-tag validation.

## Usage

```bash
make test
make build
./bin/klvtool doctor
./bin/klvtool extract --input testdata/fixtures/sample.ts --out /tmp/klvtool-raw
./bin/klvtool packetize --input /tmp/klvtool-raw --out /tmp/klvtool-packets --mode best-effort
./bin/klvtool decode --input testdata/fixtures/sample.ts --format ndjson
./bin/klvtool decode --input testdata/fixtures/sample.ts --format text --raw
```

### Decode flags

| Flag | Default | Description |
|------|---------|-------------|
| `--input` | (required) | Path to MPEG-TS input file |
| `--format` | `ndjson` | Output format: `ndjson` or `text` |
| `--raw` | `false` | Include raw bytes (base64) and units per item |
| `--strict` | `false` | Exit 1 if any error-severity diagnostic is emitted |
| `--pid` | `0` (all) | Limit to a specific KLV data stream PID |
| `--out` | stdout | Write output to a file instead of stdout |
| `--schema` | (auto) | Override auto-detection with a specific spec URN |

The `testdata/fixtures/sample.ts` path is a local fixture reference. If that file is not present in your checkout, use any equivalent MPEG-TS sample that exposes KLV payloads for the extract and packetize smoke flow.

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
