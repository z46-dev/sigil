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
			WebGPUSupported:     true,
			WebGPUVendor:        "GPU Vendor",
			WebGPUArchitecture:  "GPU Architecture",
			WebGPUDevice:        "GPU Device",
			WebGPUFeaturesHash:  "gpu-features",
			WebGPULimitsHash:    "gpu-limits",
			WebGPURenderHash:    "gpu-render",
			WebGPUComputeHash:   "gpu-compute",
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
	current.Rendering.WebGPUFeaturesHash = "other-gpu-features"
	current.Rendering.WebGPULimitsHash = "other-gpu-limits"
	current.Rendering.WebGPURenderHash = "other-gpu-render"
	current.Rendering.WebGPUComputeHash = "other-gpu-compute"
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

// TestMatchSnapshotAcrossBravePrivateMode verifies farbled hardware values do not displace stable device evidence.
func TestMatchSnapshotAcrossBravePrivateMode(t *testing.T) {
	var (
		regular     BrowserSignals = representativeSignals()
		private     BrowserSignals = regular
		distractor  BrowserSignals
		regularID   string
		privateID   string
		result      MatchResult
		err         error
		identifyErr error
	)

	regular.HardwareConcurrency = 2
	regular.DeviceMemory = 0.5
	private.HardwareConcurrency = 11
	private.DeviceMemory = 8
	private.Rendering.CanvasHash = "private-canvas"
	private.Audio.Hash = "private-audio"
	distractor = private
	distractor.ScreenWidth = 1280
	distractor.ScreenHeight = 720

	if regularID, err = IdentifySnapshotForMode(regular, ModeDevice); err != nil {
		t.Fatalf("identify regular Brave snapshot: %v", err)
	}
	if privateID, identifyErr = IdentifySnapshotForMode(private, ModeDevice); identifyErr != nil || privateID != regularID {
		t.Fatalf("Brave private mode changed the device key: regular=%q private=%q err=%v", regularID, privateID, identifyErr)
	}

	result = MatchSnapshot(snapshotForTest(t, private, ModeDevice), ModeDevice, []MatchCandidate{
		{VisitorID: "regular-visitor", SnapshotID: regularID, Signals: regular},
		{VisitorID: "distractor", Signals: distractor},
	})
	if result.Decision != "matched" || result.VisitorID != "regular-visitor" || result.Collision {
		t.Fatalf("Brave private observation did not match its regular visitor: %+v", result)
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
	current.Rendering.WebGPUVendor = "Other WebGPU Vendor"
	current.Rendering.WebGPUArchitecture = "Other WebGPU Architecture"
	current.Rendering.WebGPUDevice = "Other WebGPU Device"
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

// TestServerEvidenceAndAggressiveness verifies network evidence is measured without dominating a match.
func TestServerEvidenceAndAggressiveness(t *testing.T) {
	var (
		signals  BrowserSignals = representativeSignals()
		snapshot Snapshot       = snapshotForTest(t, signals, ModeDeviceAndBrowser)
		result   MatchResult
	)

	snapshot.Server = ServerSignals{
		NetworkPrefixHash: "network-a",
		Protocol:          "HTTP/2",
		TLS:               true,
		IP: IPClassification{
			ASN:             64500,
			ASNOrganization: "Example Network",
			CountryCode:     "US",
			City:            "New York",
		},
	}
	result = MatchSnapshot(snapshot, ModeDeviceAndBrowser, []MatchCandidate{{
		VisitorID: "visitor-one",
		Signals:   signals,
		Server:    snapshot.Server,
	}})

	if result.Decision != "matched" || result.Aggressiveness.Level != "high" || result.Aggressiveness.Score != 100 {
		t.Fatalf("server evidence was not reflected in the result: %+v", result)
	}

	var foundNetwork bool
	for _, evidence := range result.Evidence {
		if evidence.Name == "network-prefix" {
			foundNetwork = true
		}
	}

	if !foundNetwork {
		t.Fatalf("network evidence is missing: %+v", result.Evidence)
	}
}
