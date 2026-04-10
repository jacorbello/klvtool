# Fixture Guidance

This directory is reserved for MPEG-TS samples used by `klvtool extract` integration tests.

## Expected Fixture

- `sample.ts`: a small MPEG-TS file that contains at least one KLV or other data stream that `ffmpeg` can expose through `ffprobe` and extract through `ffmpeg`.

## Requirements

- Only commit fixtures that are legally redistributable.
- Do not commit assets that contain sensitive telemetry, location data, or personal information.
- Prefer short clips with the minimum payload needed to exercise `klvtool doctor` and `klvtool extract`.
- Document the provenance of any committed fixture in this file.

Until a vetted sample is added, integration tests that depend on `testdata/fixtures/sample.ts` will skip cleanly.
