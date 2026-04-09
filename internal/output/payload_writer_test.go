package output

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
)

func TestWritePayloadComputesHashAndCreatesDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "payloads", "nested")
	payload := []byte{0x01, 0x02, 0x03}

	got, err := WritePayload(dir, "record-001", payload)
	if err != nil {
		t.Fatalf("write payload: %v", err)
	}

	wantHash := sha256.Sum256(payload)
	if got.Hash != "sha256:"+hex.EncodeToString(wantHash[:]) {
		t.Fatalf("unexpected payload hash: got %q", got.Hash)
	}
	if got.Size != int64(len(payload)) {
		t.Fatalf("unexpected payload size: got %d want %d", got.Size, len(payload))
	}

	wantPath := filepath.Join(dir, "record-001.bin")
	if got.Path != wantPath {
		t.Fatalf("unexpected payload path: got %q want %q", got.Path, wantPath)
	}

	fileBytes, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read payload file: %v", err)
	}
	if string(fileBytes) != string(payload) {
		t.Fatalf("unexpected payload file contents: got %v want %v", fileBytes, payload)
	}
}

func TestWritePayloadWrapsDirectoryErrors(t *testing.T) {
	root := t.TempDir()
	blockingPath := filepath.Join(root, "payloads")
	if err := os.WriteFile(blockingPath, []byte("file"), 0o600); err != nil {
		t.Fatalf("create blocking file: %v", err)
	}

	_, err := WritePayload(blockingPath, "record-001", []byte("payload"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, &model.Error{Code: model.CodeOutputWrite}) {
		t.Fatalf("expected output write failure, got %v", err)
	}
}
