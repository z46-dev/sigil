package service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type (
	deviceIdentityInput struct {
		Platform            string  `json:"platform"`
		ScreenShortSide     int     `json:"screenShortSide"`
		ScreenLongSide      int     `json:"screenLongSide"`
		ColorDepth          int     `json:"colorDepth"`
		PixelRatio          float64 `json:"pixelRatio"`
		HardwareConcurrency int     `json:"hardwareConcurrency"`
		DeviceMemory        float64 `json:"deviceMemory"`
		MaxTouchPoints      int     `json:"maxTouchPoints"`
		WebGLVendor         string  `json:"webglVendor"`
		WebGLRenderer       string  `json:"webglRenderer"`
		FontCount           int     `json:"fontCount"`
		FontsHash           string  `json:"fontsHash"`
	}

	browserIdentityInput struct {
		UserAgent           string       `json:"userAgent"`
		Vendor              string       `json:"vendor"`
		Languages           []string     `json:"languages"`
		Timezone            string       `json:"timezone"`
		CookiesEnabled      bool         `json:"cookiesEnabled"`
		DoNotTrack          string       `json:"doNotTrack"`
		WebDriver           bool         `json:"webDriver"`
		LocalStorage        bool         `json:"localStorage"`
		SessionStorage      bool         `json:"sessionStorage"`
		CanvasHash          string       `json:"canvasHash"`
		WebGLExtensionsHash string       `json:"webglExtensionsHash"`
		Audio               AudioSignals `json:"audio"`
		FontMetricsHash     string       `json:"fontMetricsHash"`
	}
)

// String returns the public name for an identity mode.
func (mode Mode) String() (name string) {
	switch mode {
	case ModeDevice:
		name = "device"
	case ModeBrowser:
		name = "browser"
	case ModeDeviceAndBrowser:
		name = "device-and-browser"
	}

	return
}

// ParseMode converts a public identity scope name into a mode.
func ParseMode(value string) (mode Mode, err error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "device":
		mode = ModeDevice
	case "browser":
		mode = ModeBrowser
	case "device-and-browser", "combined", "":
		mode = ModeDeviceAndBrowser
	default:
		err = fmt.Errorf("unknown fingerprint mode %q", value)
	}

	return
}

func digestSignal(value string) (digest string) {
	digest = digestBytes([]byte(value))
	return
}

func digestBytes(value []byte) (digest string) {
	var sum [sha256.Size]byte = sha256.Sum256(value)
	digest = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

func orderedDimensions(width, height int) (shortSide, longSide int) {
	if shortSide, longSide = width, height; shortSide > longSide {
		shortSide, longSide = longSide, shortSide
	}

	return
}

func deviceIdentity(signals BrowserSignals) (identity deviceIdentityInput) {
	identity.ScreenShortSide, identity.ScreenLongSide = orderedDimensions(signals.ScreenWidth, signals.ScreenHeight)
	identity.Platform = signals.Platform
	identity.ColorDepth = signals.ColorDepth
	identity.PixelRatio = signals.PixelRatio
	identity.HardwareConcurrency = signals.HardwareConcurrency
	identity.DeviceMemory = signals.DeviceMemory
	identity.MaxTouchPoints = signals.MaxTouchPoints
	identity.WebGLVendor = signals.Rendering.WebGLVendor
	identity.WebGLRenderer = signals.Rendering.WebGLRenderer
	identity.FontCount = signals.Fonts.DetectedCount
	identity.FontsHash = signals.Fonts.DetectedHash

	return
}

func browserIdentity(signals BrowserSignals) (identity browserIdentityInput) {
	identity = browserIdentityInput{
		UserAgent:           signals.UserAgent,
		Vendor:              signals.Vendor,
		Languages:           signals.Languages,
		Timezone:            signals.Timezone,
		CookiesEnabled:      signals.CookiesEnabled,
		DoNotTrack:          signals.DoNotTrack,
		WebDriver:           signals.WebDriver,
		LocalStorage:        signals.LocalStorage,
		SessionStorage:      signals.SessionStorage,
		CanvasHash:          signals.Rendering.CanvasHash,
		WebGLExtensionsHash: signals.Rendering.WebGLExtensionsHash,
		Audio:               signals.Audio,
		FontMetricsHash:     signals.Fonts.MetricsHash,
	}

	return
}

// IdentifySnapshotForMode generates a domain-separated identifier for one identity scope.
func IdentifySnapshotForMode(signals BrowserSignals, mode Mode) (identifier string, err error) {
	var encoded []byte

	switch mode {
	case ModeDevice:
		encoded, err = json.Marshal(struct {
			SchemaVersion int                 `json:"schemaVersion"`
			Scope         string              `json:"scope"`
			Device        deviceIdentityInput `json:"device"`
		}{
			SchemaVersion: SchemaVersion,
			Scope:         mode.String(),
			Device:        deviceIdentity(signals),
		})
	case ModeBrowser:
		encoded, err = json.Marshal(struct {
			SchemaVersion int                  `json:"schemaVersion"`
			Scope         string               `json:"scope"`
			Browser       browserIdentityInput `json:"browser"`
		}{
			SchemaVersion: SchemaVersion,
			Scope:         mode.String(),
			Browser:       browserIdentity(signals),
		})
	case ModeDeviceAndBrowser:
		encoded, err = json.Marshal(struct {
			SchemaVersion int            `json:"schemaVersion"`
			Scope         string         `json:"scope"`
			Browser       BrowserSignals `json:"browser"`
		}{
			SchemaVersion: SchemaVersion,
			Scope:         mode.String(),
			Browser:       signals,
		})
	default:
		err = fmt.Errorf("invalid fingerprint mode %d", mode)
		return
	}

	if err != nil {
		err = fmt.Errorf("encode browser signals: %w", err)
		return
	}

	identifier = fmt.Sprintf("sg%d%s_%s", SchemaVersion, modePrefix(mode), digestSignal(string(encoded)))
	return
}

func modePrefix(mode Mode) (prefix string) {
	switch mode {
	case ModeDevice:
		prefix = "d"
	case ModeBrowser:
		prefix = "b"
	case ModeDeviceAndBrowser:
		prefix = "c"
	}

	return
}

// IdentifySnapshot generates a combined device-and-browser identifier.
func IdentifySnapshot(signals BrowserSignals) (identifier string, err error) {
	identifier, err = IdentifySnapshotForMode(signals, ModeDeviceAndBrowser)
	return
}

// ApplyMode selects an identity scope and refreshes a snapshot's identifier.
func ApplyMode(snapshot Snapshot, mode Mode) (updated Snapshot, err error) {
	updated = snapshot
	updated.mode = mode
	updated.Scope = mode.String()
	updated.SnapshotID, err = IdentifySnapshotForMode(updated.Browser, mode)

	return
}
