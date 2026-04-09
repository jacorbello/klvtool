package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestManifestMarshalJSON(t *testing.T) {
	transportStreamID := uint16(42)
	packetOffset := int64(128)
	packetIndex := int64(9)
	continuityCounter := uint8(7)
	pts := int64(90000)
	dts := int64(89900)

	got, err := json.Marshal(Manifest{
		SchemaVersion:   "1",
		SourceInputPath: "/tmp/input.ts",
		BackendName:     "ffmpeg",
		BackendVersion:  "7.1",
		Records: []Record{
			{
				RecordID:          "rec-001",
				PID:               136,
				TransportStreamID: &transportStreamID,
				PacketOffset:      &packetOffset,
				PacketIndex:       &packetIndex,
				ContinuityCounter: &continuityCounter,
				PTS:               &pts,
				DTS:               &dts,
				PayloadPath:       "payloads/rec-001.bin",
				PayloadSize:       4,
				PayloadHash:       "sha256:abc",
				Warnings:          []string{"discontinuity"},
			},
			{
				RecordID:    "rec-002",
				PID:         256,
				PayloadPath: "payloads/rec-002.bin",
				PayloadSize: 0,
				PayloadHash: "sha256:def",
				Warnings:    []string{},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	want := `{"schemaVersion":"1","sourceInputPath":"/tmp/input.ts","backendName":"ffmpeg","backendVersion":"7.1","records":[{"recordId":"rec-001","pid":136,"transportStreamId":42,"packetOffset":128,"packetIndex":9,"continuityCounter":7,"pts":90000,"dts":89900,"payloadPath":"payloads/rec-001.bin","payloadSize":4,"payloadHash":"sha256:abc","warnings":["discontinuity"]},{"recordId":"rec-002","pid":256,"payloadPath":"payloads/rec-002.bin","payloadSize":0,"payloadHash":"sha256:def","warnings":[]}]}`
	if string(got) != want {
		t.Fatalf("unexpected manifest json\nwant: %s\ngot:  %s", want, string(got))
	}
}

func TestErrorTyping(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		code Code
	}{
		{name: "invalid usage", err: InvalidUsage(fmt.Errorf("bad flag")), code: CodeInvalidUsage},
		{name: "missing dependency", err: MissingDependency(fmt.Errorf("ffprobe")), code: CodeMissingDependency},
		{name: "unsupported backend", err: UnsupportedBackend(fmt.Errorf("backend-x")), code: CodeUnsupportedBackend},
		{name: "backend execution failure", err: BackendExecution(fmt.Errorf("exit 2")), code: CodeBackendExecution},
		{name: "backend parse failure", err: BackendParse(fmt.Errorf("invalid json")), code: CodeBackendParse},
		{name: "output write failure", err: OutputWrite(fmt.Errorf("disk full")), code: CodeOutputWrite},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == nil {
				t.Fatal("expected typed error")
			}
			if tc.err.Code != tc.code {
				t.Fatalf("expected code %q, got %q", tc.code, tc.err.Code)
			}
			if !errors.Is(tc.err, &Error{Code: tc.code}) {
				t.Fatalf("expected errors.Is to match code %q", tc.code)
			}
			var typed *Error
			if !errors.As(tc.err, &typed) {
				t.Fatalf("expected errors.As to match %T", tc.err)
			}
			if typed.Code != tc.code {
				t.Fatalf("expected typed code %q, got %q", tc.code, typed.Code)
			}
			if got := tc.err.Error(); got == "" {
				t.Fatal("expected non-empty error string")
			}
		})
	}
}
