# klvtool

`klvtool` is a Go CLI for working with MPEG-TS video assets and KLV payloads.

The initial roadmap is to ship two top-level commands:

- `doctor` for checking local media-tool dependencies and environment health
- `extract` for pulling KLV payloads and metadata out of file-based MPEG-TS inputs

This repository starts with a minimal CLI baseline and will grow into backend-aware extraction, manifest generation, and diagnostics.
