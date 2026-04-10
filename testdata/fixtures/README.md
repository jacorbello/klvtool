# Fixture Guidance

This directory holds MPEG-TS samples used by `klvtool extract` integration tests.

## sample.ts

- **Source:** [Esri FMV tutorial dataset](https://www.arcgis.com/sharing/rest/content/items/55ec6f32d5e342fcbfba376ca2cc409a/data)
- **Original filename:** `Truck.ts` from `FMV_tutorial_data.zip`
- **Size:** ~98 MB
- **SHA256:** `8667276b2c2fb36baa089b00e3978f55893cacf6e0d8f6e6d480bb934747cc79`
- **Provisioning:** Run `make test-data` to download and verify automatically.

This file is gitignored. Integration tests skip cleanly when it is absent.

## Adding New Fixtures

- Only add fixtures that are legally redistributable.
- Do not add assets that contain sensitive telemetry, location data, or personal information.
- Document provenance in this file.
