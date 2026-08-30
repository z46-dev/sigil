package db

import (
	"path/filepath"
	"testing"

	"github.com/z46-dev/gosqlite"
	"github.com/z46-dev/sigil/service"
)

func initializeTestDatabase(t *testing.T) (testDriver *gosqlite.Driver) {
	var err error

	if testDriver, err = gosqlite.Begin(filepath.Join(t.TempDir(), "sigil-test.db")); err != nil {
		t.Fatalf("open test database: %v", err)
	}

	if Fingerprints, err = gosqlite.Register(testDriver, service.Fingerprint{}); err != nil {
		t.Fatalf("register fingerprints: %v", err)
	}

	if Observations, err = gosqlite.Register(testDriver, service.Observation{}); err != nil {
		t.Fatalf("register observations: %v", err)
	}

	if _, err = Fingerprints.Migrate(gosqlite.MigrationOptions{}); err != nil {
		t.Fatalf("migrate fingerprints: %v", err)
	}

	if _, err = Observations.Migrate(gosqlite.MigrationOptions{}); err != nil {
		t.Fatalf("migrate observations: %v", err)
	}

	return
}

func databaseTestSnapshot(t *testing.T) (snapshot service.Snapshot) {
	var err error

	snapshot = service.Snapshot{
		SchemaVersion: service.SchemaVersion,
		Scope:         service.ModeDevice.String(),
		Browser: service.BrowserSignals{
			Platform:            "windows",
			ScreenWidth:         1920,
			ScreenHeight:        1080,
			ColorDepth:          24,
			PixelRatio:          1,
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			Rendering: service.RenderingSignals{
				WebGLVendor:   "GPU Vendor",
				WebGLRenderer: "GPU Renderer",
			},
			Fonts: service.FontSignals{DetectedCount: 10, DetectedHash: "fonts"},
		},
	}

	if snapshot.SnapshotID, err = service.IdentifySnapshotForMode(snapshot.Browser, service.ModeDevice); err != nil {
		t.Fatalf("identify snapshot: %v", err)
	}

	return
}

// TestMatchAndRecord verifies a visitor is reused without duplicating an identical observation.
func TestMatchAndRecord(t *testing.T) {
	var (
		testDriver  *gosqlite.Driver = initializeTestDatabase(t)
		first, next service.MatchResult
		err         error
	)

	defer testDriver.Close()

	if first, err = MatchAndRecord(databaseTestSnapshot(t)); err != nil {
		t.Fatalf("record first snapshot: %v", err)
	}

	if first.Decision != "new" || first.VisitorID == "" {
		t.Fatalf("first snapshot did not create a visitor: %+v", first)
	}

	if next, err = MatchAndRecord(databaseTestSnapshot(t)); err != nil {
		t.Fatalf("record returning snapshot: %v", err)
	}

	if next.Decision != "matched" || next.VisitorID != first.VisitorID {
		t.Fatalf("returning snapshot did not reuse visitor: first=%+v next=%+v", first, next)
	}

	var (
		fingerprints []*service.Fingerprint
		observations []*service.Observation
	)

	if fingerprints, err = Fingerprints.SelectAll(); err != nil {
		t.Fatalf("list fingerprints: %v", err)
	}

	if observations, err = Observations.SelectAll(); err != nil {
		t.Fatalf("list observations: %v", err)
	}

	if len(fingerprints) != 1 || fingerprints[0].ObservationCount != 2 || len(observations) != 1 {
		t.Fatalf("unexpected persistence state: fingerprints=%+v observations=%+v", fingerprints, observations)
	}
}

// TestMatchAndRecordRetainsNetworkChanges verifies network evidence is not lost to snapshot deduplication.
func TestMatchAndRecordRetainsNetworkChanges(t *testing.T) {
	var (
		testDriver   *gosqlite.Driver = initializeTestDatabase(t)
		snapshot     service.Snapshot = databaseTestSnapshot(t)
		observations []*service.Observation
		err          error
	)

	defer testDriver.Close()

	snapshot.Server.NetworkPrefixHash = "network-a"
	if _, err = MatchAndRecord(snapshot); err != nil {
		t.Fatalf("record first network: %v", err)
	}

	snapshot.Server.NetworkPrefixHash = "network-b"
	if _, err = MatchAndRecord(snapshot); err != nil {
		t.Fatalf("record changed network: %v", err)
	}

	if observations, err = Observations.SelectAll(); err != nil {
		t.Fatalf("list observations: %v", err)
	}

	if len(observations) != 2 {
		t.Fatalf("network change was deduplicated: observations=%+v", observations)
	}
}
