package v1

import (
	"encoding/json"
	"testing"

	"github.com/z46-dev/sigil/service"
)

// TestDecodeSnapshot verifies the ingestion boundary accepts one strict JSON object.
func TestDecodeSnapshot(t *testing.T) {
	var (
		snapshot service.Snapshot
		encoded  []byte
		decoded  service.Snapshot
		err      error
	)

	snapshot = service.Snapshot{SchemaVersion: service.SchemaVersion, Scope: service.ModeDevice.String()}
	if encoded, err = json.Marshal(snapshot); err != nil {
		t.Fatalf("encode snapshot: %v", err)
	}

	if err = decodeSnapshot(encoded, &decoded); err != nil {
		t.Fatalf("decode valid snapshot: %v", err)
	}

	if err = decodeSnapshot([]byte(`{"schemaVersion":3,"scope":"device","unknown":true}`), &decoded); err == nil {
		t.Fatal("unknown field unexpectedly decoded")
	}

	if err = decodeSnapshot(append(encoded, encoded...), &decoded); err == nil {
		t.Fatal("multiple JSON values unexpectedly decoded")
	}
}
