package service

import "testing"

func representativeSignals() (signals BrowserSignals) {
	signals = BrowserSignals{
		UserAgent:           "Example/1.0",
		Platform:            "windows",
		Vendor:              "Example Vendor",
		Languages:           []string{"en-US", "en"},
		Timezone:            "America/New_York",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		ColorDepth:          24,
		PixelRatio:          1,
		HardwareConcurrency: 8,
		DeviceMemory:        8,
		MaxTouchPoints:      0,
		CookiesEnabled:      true,
		LocalStorage:        true,
		SessionStorage:      true,
		Rendering: RenderingSignals{
			CanvasSupported:     true,
			CanvasHash:          "canvas",
			WebGLSupported:      true,
			WebGLVendor:         "GPU Vendor",
			WebGLRenderer:       "GPU Renderer",
			WebGLExtensionsHash: "extensions",
		},
		Audio: AudioSignals{
			Supported: true,
			Hash:      "audio",
		},
		Fonts: FontSignals{
			DetectedCount: 12,
			DetectedHash:  "fonts",
			MetricsHash:   "metrics",
		},
	}

	return
}

func snapshotForTest(t *testing.T, signals BrowserSignals, mode Mode) (snapshot Snapshot) {
	var err error

	snapshot = Snapshot{
		SchemaVersion: SchemaVersion,
		Scope:         mode.String(),
		Browser:       signals,
	}

	if snapshot.SnapshotID, err = IdentifySnapshotForMode(signals, mode); err != nil {
		t.Fatalf("identify test snapshot: %v", err)
	}

	return
}

// TestMatchSnapshotAcrossBrowsers verifies browser-engine changes retain a device candidate.
func TestMatchSnapshotAcrossBrowsers(t *testing.T) {
	var (
		previous BrowserSignals = representativeSignals()
		current  BrowserSignals = previous
		result   MatchResult
	)

	current.UserAgent = "OtherBrowser/9.0"
	current.Vendor = "Other Vendor"
	current.Rendering.CanvasHash = "other-canvas"
	current.Rendering.WebGLExtensionsHash = "other-extensions"
	current.Audio.Hash = "other-audio"
	current.Fonts.MetricsHash = "other-metrics"

	if result = MatchSnapshot(snapshotForTest(t, current, ModeDevice), ModeDevice, []MatchCandidate{{
		VisitorID: "visitor-one",
		Signals:   previous,
	}}); result.Decision != "matched" || result.VisitorID != "visitor-one" {
		t.Fatalf("cross-browser device was not matched: %+v", result)
	}

	if result.Confidence < 0.9 {
		t.Fatalf("cross-browser confidence is unexpectedly low: %f", result.Confidence)
	}
}

// TestMatchSnapshotRejectsCollision verifies indistinguishable visitors are never merged.
func TestMatchSnapshotRejectsCollision(t *testing.T) {
	var (
		signals BrowserSignals = representativeSignals()
		result  MatchResult    = MatchSnapshot(snapshotForTest(t, signals, ModeDevice), ModeDevice, []MatchCandidate{
			{
				VisitorID: "visitor-one",
				Signals:   signals,
			}, {
				VisitorID: "visitor-two",
				Signals:   signals,
			},
		})
	)

	if result.Decision != "ambiguous" || !result.Collision || result.VisitorID != "" {
		t.Fatalf("collision was not rejected: %+v", result)
	}

	if result.Margin != 0 || result.RunnerUpScore != 1 {
		t.Fatalf("collision measurements are incorrect: %+v", result)
	}
}

// TestMatchSnapshotCreatesNewIdentity verifies substantially different devices stay separate.
func TestMatchSnapshotCreatesNewIdentity(t *testing.T) {
	var (
		previous, current BrowserSignals = representativeSignals(), representativeSignals()
		result            MatchResult
	)

	current.Platform = "android"
	current.ScreenWidth = 412
	current.ScreenHeight = 915
	current.ColorDepth = 30
	current.PixelRatio = 3
	current.HardwareConcurrency = 2
	current.DeviceMemory = 2
	current.MaxTouchPoints = 5
	current.Rendering.WebGLVendor = "Other GPU Vendor"
	current.Rendering.WebGLRenderer = "Other GPU Renderer"
	current.Fonts.DetectedHash = "other-fonts"

	if result = MatchSnapshot(snapshotForTest(t, current, ModeDevice), ModeDevice, []MatchCandidate{{
		VisitorID: "visitor-one",
		Signals:   previous,
	}}); result.Decision != "new" || result.VisitorID != "" {
		t.Fatalf("different device was incorrectly matched: %+v", result)
	}
}

// TestValidateSnapshotRejectsTampering verifies the server recomputes client identifiers.
func TestValidateSnapshotRejectsTampering(t *testing.T) {
	var (
		snapshot Snapshot = snapshotForTest(t, representativeSignals(), ModeDeviceAndBrowser)
		err      error
	)

	snapshot.Browser.UserAgent = "tampered"
	if _, err = ValidateSnapshot(snapshot); err == nil {
		t.Fatal("tampered snapshot unexpectedly validated")
	}
}
