package v1

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/z46-dev/sigil/service"
)

// TestDecodeRequest verifies the ingestion boundary accepts one strict JSON object.
func TestDecodeRequest(t *testing.T) {
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

	if err = decodeRequest(encoded, &decoded); err != nil {
		t.Fatalf("decode valid snapshot: %v", err)
	}

	if err = decodeRequest([]byte(`{"schemaVersion":3,"scope":"device","unknown":true}`), &decoded); err == nil {
		t.Fatal("unknown field unexpectedly decoded")
	}

	if err = decodeRequest(append(encoded, encoded...), &decoded); err == nil {
		t.Fatal("multiple JSON values unexpectedly decoded")
	}
}

// TestChallengeRegistry verifies challenges expire and cannot be replayed.
func TestChallengeRegistry(t *testing.T) {
	var (
		registry challengeRegistry = challengeRegistry{challenges: make(map[string]challengeRecord)}
		now      time.Time         = time.Now().UTC()
		issued   challengeResponse
		err      error
	)

	if issued, err = registry.issue(now, "browser-a"); err != nil {
		t.Fatalf("issue challenge: %v", err)
	}

	if registry.consume(issued.Challenge, now, "browser-b") {
		t.Fatal("challenge was accepted from a different user agent")
	}

	if issued, err = registry.issue(now, "browser-a"); err != nil {
		t.Fatalf("issue replacement challenge: %v", err)
	}

	if !registry.consume(issued.Challenge, now, "browser-a") {
		t.Fatal("fresh challenge was rejected")
	}

	if registry.consume(issued.Challenge, now, "browser-a") {
		t.Fatal("consumed challenge was accepted twice")
	}

	registry.challenges["expired"] = challengeRecord{expiresAt: now.Add(-time.Second), binding: "browser-a"}
	if registry.consume("expired", now, "browser-a") {
		t.Fatal("expired challenge was accepted")
	}

	for index := 0; index < maximumChallenges; index++ {
		registry.challenges[string(rune(index))] = challengeRecord{expiresAt: now.Add(time.Minute), binding: "browser-a"}
	}

	if _, err = registry.issue(now, "browser-a"); err == nil {
		t.Fatal("challenge registry accepted more than its capacity")
	}
}
