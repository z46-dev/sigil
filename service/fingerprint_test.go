package service

import (
	"strings"
	"testing"
)

// TestIdentifySnapshot verifies the identifier's determinism and sensitivity to signal changes.
func TestIdentifySnapshot(t *testing.T) {
	var (
		signals BrowserSignals = BrowserSignals{
			UserAgent:           "Example/1.0",
			Languages:           []string{"en-US", "en"},
			Timezone:            "America/New_York",
			ScreenWidth:         1920,
			ScreenHeight:        1080,
			HardwareConcurrency: 8,
		}
		first, second, changed string
		err                    error
	)

	if first, err = IdentifySnapshot(signals); err != nil {
		t.Fatalf("identify first snapshot: %v", err)
	}

	if second, err = IdentifySnapshot(signals); err != nil {
		t.Fatalf("identify second snapshot: %v", err)
	}

	if first != second {
		t.Fatalf("same signals produced different identifiers: %q and %q", first, second)
	}

	if !strings.HasPrefix(first, "sg4c_") {
		t.Fatalf("identifier %q does not carry the schema version", first)
	}

	signals.ScreenWidth = 2560
	if changed, err = IdentifySnapshot(signals); err != nil {
		t.Fatalf("identify changed snapshot: %v", err)
	}

	if first == changed {
		t.Fatal("changed signals produced the same identifier")
	}
}

// TestIdentityModes verifies browser changes do not alter the conservative device key.
func TestIdentityModes(t *testing.T) {
	var (
		signals BrowserSignals = BrowserSignals{
			UserAgent:           "BrowserA/1.0",
			Platform:            "Win32",
			ScreenWidth:         1920,
			ScreenHeight:        1080,
			HardwareConcurrency: 8,
			Rendering: RenderingSignals{
				CanvasHash:    "canvas-a",
				WebGLVendor:   "Vendor",
				WebGLRenderer: "Renderer",
			},
			Audio: AudioSignals{Supported: true, Hash: "audio-a"},
			Fonts: FontSignals{DetectedCount: 4, DetectedHash: "fonts", MetricsHash: "metrics-a"},
		}
		changed                      BrowserSignals = signals
		deviceFirst, deviceChanged   string
		browserFirst, browserChanged string
		err                          error
	)

	changed.UserAgent = "BrowserB/2.0"
	changed.Rendering.CanvasHash = "canvas-b"
	changed.Audio.Hash = "audio-b"
	changed.Fonts.MetricsHash = "metrics-b"
	changed.HardwareConcurrency = 32
	changed.DeviceMemory = 16
	changed.ScreenWidth = 1080
	changed.ScreenHeight = 1920

	if deviceFirst, err = IdentifySnapshotForMode(signals, ModeDevice); err != nil {
		t.Fatalf("identify first device: %v", err)
	}

	if deviceChanged, err = IdentifySnapshotForMode(changed, ModeDevice); err != nil {
		t.Fatalf("identify changed device: %v", err)
	}

	if deviceFirst != deviceChanged {
		t.Fatalf("browser-only changes altered device identifier: %q and %q", deviceFirst, deviceChanged)
	}

	if browserFirst, err = IdentifySnapshotForMode(signals, ModeBrowser); err != nil {
		t.Fatalf("identify first browser: %v", err)
	}

	if browserChanged, err = IdentifySnapshotForMode(changed, ModeBrowser); err != nil {
		t.Fatalf("identify changed browser: %v", err)
	}

	if browserFirst == browserChanged {
		t.Fatal("browser changes did not alter browser identifier")
	}

	if !strings.HasPrefix(deviceFirst, "sg4d_") || !strings.HasPrefix(browserFirst, "sg4b_") {
		t.Fatalf("mode identifiers are not domain-separated: %q and %q", deviceFirst, browserFirst)
	}
}

// TestWebGPUIdentityEvidence verifies WebGPU evidence contributes to its intended scopes.
func TestWebGPUIdentityEvidence(t *testing.T) {
	var (
		signals BrowserSignals = BrowserSignals{Rendering: RenderingSignals{
			WebGPUVendor:       "vendor-a",
			WebGPUArchitecture: "architecture-a",
			WebGPUDevice:       "device-a",
			WebGPUFeaturesHash: "features-a",
			WebGPULimitsHash:   "limits-a",
			WebGPURenderHash:   "render-a",
			WebGPUComputeHash:  "compute-a",
		}}
		changed                   BrowserSignals = signals
		deviceFirst, deviceNext   string
		browserFirst, browserNext string
		err                       error
	)

	changed.Rendering.WebGPUDevice = "device-b"
	if deviceFirst, err = IdentifySnapshotForMode(signals, ModeDevice); err != nil {
		t.Fatalf("identify first WebGPU device: %v", err)
	}
	if deviceNext, err = IdentifySnapshotForMode(changed, ModeDevice); err != nil || deviceFirst == deviceNext {
		t.Fatalf("WebGPU adapter did not affect device identity: first=%q next=%q err=%v", deviceFirst, deviceNext, err)
	}

	changed = signals
	changed.Rendering.WebGPURenderHash = "render-b"
	if browserFirst, err = IdentifySnapshotForMode(signals, ModeBrowser); err != nil {
		t.Fatalf("identify first WebGPU browser: %v", err)
	}
	if browserNext, err = IdentifySnapshotForMode(changed, ModeBrowser); err != nil || browserFirst == browserNext {
		t.Fatalf("WebGPU render did not affect browser identity: first=%q next=%q err=%v", browserFirst, browserNext, err)
	}

	changed = signals
	changed.Rendering.WebGPUComputeHash = "compute-b"
	if browserNext, err = IdentifySnapshotForMode(changed, ModeBrowser); err != nil || browserFirst == browserNext {
		t.Fatalf("WebGPU compute did not affect browser identity: first=%q next=%q err=%v", browserFirst, browserNext, err)
	}
}

// TestParseMode verifies the public mode names and rejects ambiguous values.
func TestParseMode(t *testing.T) {
	var (
		mode Mode
		err  error
	)

	if mode, err = ParseMode("device-and-browser"); err != nil || mode != ModeDeviceAndBrowser {
		t.Fatalf("parse combined mode: mode=%d err=%v", mode, err)
	}

	if _, err = ParseMode("person"); err == nil {
		t.Fatal("unknown mode unexpectedly succeeded")
	}
}

// TestCollectOutsideBrowser verifies unsupported runtimes fail clearly.
func TestCollectOutsideBrowser(t *testing.T) {
	var err error

	if _, err = Collect(); err == nil {
		t.Fatal("non-browser collection unexpectedly succeeded")
	}
}
