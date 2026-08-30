package service

import "time"

type (
	Mode uint8

	// AudioSignals contains the output of an offline audio rendering probe.
	AudioSignals struct {
		Supported bool   `json:"supported"`
		Hash      string `json:"hash"`
	}

	// FontSignals summarizes detected fonts without retaining their names.
	FontSignals struct {
		DetectedCount int    `json:"detectedCount"`
		DetectedHash  string `json:"detectedHash"`
		MetricsHash   string `json:"metricsHash"`
	}

	// RenderingSignals contains rendering outputs that can vary across graphics stacks.
	RenderingSignals struct {
		CanvasSupported     bool   `json:"canvasSupported"`
		CanvasHash          string `json:"canvasHash"`
		WebGLSupported      bool   `json:"webglSupported"`
		WebGLVendor         string `json:"webglVendor"`
		WebGLRenderer       string `json:"webglRenderer"`
		WebGLExtensionsHash string `json:"webglExtensionsHash"`
	}

	// IPClassification contains server-derived network ownership and location data.
	IPClassification struct {
		ASN             uint   `json:"asn,omitempty"`
		ASNOrganization string `json:"asnOrganization,omitempty"`
		CountryCode     string `json:"countryCode,omitempty"`
		City            string `json:"city,omitempty"`
		OpenProxy       bool   `json:"openProxy"`
		Tor             bool   `json:"tor"`
		Hosting         bool   `json:"hosting"`
		Malicious       bool   `json:"malicious"`
	}

	// ServerSignals contains trusted request properties observed by the server.
	ServerSignals struct {
		NetworkPrefixHash string           `json:"networkPrefixHash,omitempty"`
		Protocol          string           `json:"protocol,omitempty"`
		TLS               bool             `json:"tls"`
		IP                IPClassification `json:"ip,omitempty"`
	}

	// Aggressiveness describes the privacy impact of the signals used for a match.
	Aggressiveness struct {
		Score       int    `json:"score"`
		Level       string `json:"level"`
		SignalCount int    `json:"signalCount"`
	}

	// Fingerprint stores the durable server-side identity assigned to a visitor.
	Fingerprint struct {
		ID               int    `json:"id" gosqlite:"id,primary,increment"`
		Identifier       string `json:"identifier" gosqlite:"identifier"`
		Mode             Mode   `json:"mode" gosqlite:"mode"`
		CreatedAt        string `json:"createdAt" gosqlite:"created_at"`
		LastSeenAt       string `json:"lastSeenAt" gosqlite:"last_seen_at"`
		ObservationCount int    `json:"observationCount" gosqlite:"observation_count"`
	}

	// Observation stores one immutable signal snapshot associated with a visitor.
	Observation struct {
		ID          int    `json:"id" gosqlite:"id,primary,increment"`
		VisitorID   string `json:"visitorId" gosqlite:"visitor_id"`
		SnapshotID  string `json:"snapshotId" gosqlite:"snapshot_id"`
		Mode        Mode   `json:"mode" gosqlite:"mode"`
		SignalsJSON string `json:"-" gosqlite:"signals_json"`
		ServerJSON  string `json:"-" gosqlite:"server_json"`
		ObservedAt  string `json:"observedAt" gosqlite:"observed_at"`
	}

	// BrowserSignals contains the permissionless browser properties in the current schema.
	BrowserSignals struct {
		UserAgent           string           `json:"userAgent"`
		Platform            string           `json:"platform"`
		Vendor              string           `json:"vendor"`
		Languages           []string         `json:"languages"`
		Timezone            string           `json:"timezone"`
		ScreenWidth         int              `json:"screenWidth"`
		ScreenHeight        int              `json:"screenHeight"`
		AvailableWidth      int              `json:"availableWidth"`
		AvailableHeight     int              `json:"availableHeight"`
		ColorDepth          int              `json:"colorDepth"`
		PixelRatio          float64          `json:"pixelRatio"`
		HardwareConcurrency int              `json:"hardwareConcurrency"`
		DeviceMemory        float64          `json:"deviceMemory"`
		MaxTouchPoints      int              `json:"maxTouchPoints"`
		CookiesEnabled      bool             `json:"cookiesEnabled"`
		DoNotTrack          string           `json:"doNotTrack"`
		WebDriver           bool             `json:"webDriver"`
		LocalStorage        bool             `json:"localStorage"`
		SessionStorage      bool             `json:"sessionStorage"`
		Rendering           RenderingSignals `json:"rendering"`
		Audio               AudioSignals     `json:"audio"`
		Fonts               FontSignals      `json:"fonts"`
	}

	// Snapshot is a versioned observation and its deterministic content identifier.
	Snapshot struct {
		SchemaVersion int            `json:"schemaVersion"`
		SnapshotID    string         `json:"snapshotId"`
		Scope         string         `json:"scope"`
		CollectedAt   time.Time      `json:"collectedAt"`
		Browser       BrowserSignals `json:"browser"`
		Server        ServerSignals  `json:"-"`
		mode          Mode
	}
)

const SchemaVersion int = 3

const (
	ModeDevice Mode = 1 << iota
	ModeBrowser

	ModeDeviceAndBrowser Mode = ModeDevice | ModeBrowser
)
