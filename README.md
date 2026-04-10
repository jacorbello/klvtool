# klvtool

`klvtool` is a Go CLI for working with MPEG-TS video assets and KLV payloads.

The current bootstrap milestone ships three top-level commands:

- `klvtool doctor` checks local `ffmpeg` availability, reports detected versions, and prints install guidance.
- `klvtool extract` validates backend health, extracts KLV/data payloads using `ffmpeg`, and writes extracted payloads plus a JSON manifest line to `manifest.ndjson`.
- `klvtool packetize` replays a raw extraction checkpoint directory and writes packet checkpoint output plus a packet manifest.

## Usage

```bash
make test
make build
./bin/klvtool doctor
./bin/klvtool extract --input testdata/fixtures/sample.ts --out /tmp/klvtool-raw
./bin/klvtool packetize --input /tmp/klvtool-raw --out /tmp/klvtool-packets --mode best-effort
```

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
