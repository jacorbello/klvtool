# KLV Packet Regression Corpus

Each `*.hex` file contains one raw UAS Datalink LS packet, hex-encoded, whitespace allowed. Tests in `internal/klv/corpus_test.go` decode each file and snapshot the NDJSON output at the adjacent `*.json` path.

To bootstrap / regenerate fixtures:
1. `go test ./internal/klv/ -run TestCorpus -update` — writes any missing `.hex`/`.json` pairs using the test-helper packet builder, then writes/updates every snapshot.
2. Add new hex fixtures manually by placing `testdata/klv_packets/<name>.hex` and re-running with `-update`.
