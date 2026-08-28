package evaluation

import (
	"testing"

	"github.com/z46-dev/sigil/service"
)

func evaluationSnapshot(t *testing.T, platform, renderer string) (snapshot service.Snapshot) {
	var err error

	snapshot = service.Snapshot{
		SchemaVersion: service.SchemaVersion,
		Scope:         service.ModeDevice.String(),
		Browser: service.BrowserSignals{
			Platform:            platform,
			ScreenWidth:         1920,
			ScreenHeight:        1080,
			ColorDepth:          24,
			PixelRatio:          1,
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			Rendering:           service.RenderingSignals{WebGLVendor: "vendor", WebGLRenderer: renderer},
			Fonts:               service.FontSignals{DetectedHash: "fonts"},
		},
	}

	if snapshot.SnapshotID, err = service.IdentifySnapshotForMode(snapshot.Browser, service.ModeDevice); err != nil {
		t.Fatalf("identify evaluation snapshot: %v", err)
	}

	return
}

// TestEvaluate verifies replay metrics count correct returns and false-new decisions.
func TestEvaluate(t *testing.T) {
	var (
		first  service.Snapshot = evaluationSnapshot(t, "windows", "gpu")
		second service.Snapshot = first
		report Report
		err    error
	)

	second.Browser.Rendering.WebGLRenderer = "changed"
	if second.SnapshotID, err = service.IdentifySnapshotForMode(second.Browser, service.ModeDevice); err != nil {
		t.Fatalf("identify changed snapshot: %v", err)
	}

	if report, err = Evaluate(service.ModeDevice, []Observation{
		{Label: "device-a", Browser: "chromium", Scenario: "first", Snapshot: first},
		{Label: "device-a", Browser: "firefox", Scenario: "return", Snapshot: first},
		{Label: "device-a", Browser: "webkit", Scenario: "changed", Snapshot: second},
	}); err != nil {
		t.Fatalf("evaluate observations: %v", err)
	}

	if report.Metrics.Observations != 3 || report.Metrics.ReturningAttempts != 2 || report.Metrics.Correct < 2 {
		t.Fatalf("unexpected evaluation metrics: %+v", report.Metrics)
	}
}
