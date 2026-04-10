# klvtool

`klvtool` is a Go CLI for working with MPEG-TS video assets and KLV payloads.

The current bootstrap milestone ships three top-level commands:

- `klvtool doctor` checks local `ffmpeg` and `gstreamer` availability, reports detected versions, and prints install guidance.
- `klvtool extract` validates backend health, selects `gstreamer` or `ffmpeg`, and writes extracted payloads plus a JSON manifest line to `manifest.ndjson`.
- `klvtool packetize` replays a raw extraction checkpoint directory and writes packet checkpoint output plus a packet manifest.

`ffmpeg` and `gstreamer` are both supported extraction backends. `--backend auto` prefers `gstreamer` when it is healthy and falls back to `ffmpeg`, while explicit backend requests do not fall back.

## Usage

```bash
make test
make build
./bin/klvtool doctor
./bin/klvtool extract --input testdata/fixtures/sample.ts --out /tmp/klvtool-raw --backend auto
./bin/klvtool packetize --input /tmp/klvtool-raw --out /tmp/klvtool-packets --mode best-effort
```

The `testdata/fixtures/sample.ts` path is a local fixture reference. If that file is not present in your checkout, use any equivalent MPEG-TS sample that exposes KLV payloads for the extract and packetize smoke flow.

## Dependencies

At least one media backend is required. Either can be used independently via `--backend`, or `--backend auto` (the default) will prefer GStreamer and fall back to FFmpeg.

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

### GStreamer

| Tool | Purpose |
|------|---------|
| `gst-launch-1.0` | Pipeline execution for payload extraction |
| `gst-inspect-1.0` | Element and plugin introspection |
| `gst-discoverer-1.0` | Stream discovery and metadata inspection |
| `tsdemux` module | MPEG-TS demuxer element required by the extraction pipeline |

| Platform | Install |
|----------|---------|
| macOS (Homebrew) | `brew install gstreamer` |
| Debian / Ubuntu / WSL | `sudo apt update && sudo apt install gstreamer1.0-tools` |
| Other | <https://gstreamer.freedesktop.org/download/> |

Verify the required demux element is present:

```bash
gst-inspect-1.0 tsdemux
```

### Verifying the installation

Run `klvtool doctor` to check that all required tools and modules are detected:

```bash
klvtool doctor
```

## Development

- `make fmt` runs `go fmt ./...`
- `make lint` runs `golangci-lint`
- `make test` runs unit and package tests
- `make test-integration` runs integration coverage and skips extraction if fixtures or backends are absent
- `make build` builds `./bin/klvtool`
