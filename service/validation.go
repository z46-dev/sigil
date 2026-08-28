package service

import "fmt"

func validHash(value string) (valid bool) {
	valid = len(value) <= 128
	return
}

func validText(value string, maximum int) (valid bool) {
	valid = len(value) <= maximum
	return
}

// ValidateSnapshot verifies schema, scope, sizes, and the client-supplied content identifier.
func ValidateSnapshot(snapshot Snapshot) (mode Mode, err error) {
	var expected string

	if snapshot.SchemaVersion != SchemaVersion {
		err = fmt.Errorf("unsupported schema version %d", snapshot.SchemaVersion)
		return
	}

	if mode, err = ParseMode(snapshot.Scope); err != nil {
		return
	}

	if !validText(snapshot.Browser.UserAgent, 1024) || !validText(snapshot.Browser.Platform, 128) ||
		!validText(snapshot.Browser.Vendor, 256) || !validText(snapshot.Browser.Timezone, 128) ||
		!validText(snapshot.Browser.DoNotTrack, 64) || !validText(snapshot.Browser.Rendering.WebGLVendor, 512) ||
		!validText(snapshot.Browser.Rendering.WebGLRenderer, 1024) {
		err = fmt.Errorf("browser metadata exceeds size limits")
		return
	}

	if len(snapshot.Browser.Languages) > 32 {
		err = fmt.Errorf("too many browser languages")
		return
	}

	for _, language := range snapshot.Browser.Languages {
		if len(language) > 128 {
			err = fmt.Errorf("browser language exceeds size limit")
			return
		}
	}

	if len(snapshot.SnapshotID) > 128 || !validHash(snapshot.Browser.Rendering.CanvasHash) ||
		!validHash(snapshot.Browser.Rendering.WebGLExtensionsHash) || !validHash(snapshot.Browser.Audio.Hash) ||
		!validHash(snapshot.Browser.Fonts.DetectedHash) || !validHash(snapshot.Browser.Fonts.MetricsHash) {
		err = fmt.Errorf("signal hash exceeds size limit")
		return
	}

	if snapshot.Browser.ScreenWidth < 0 || snapshot.Browser.ScreenWidth > 32768 ||
		snapshot.Browser.ScreenHeight < 0 || snapshot.Browser.ScreenHeight > 32768 ||
		snapshot.Browser.AvailableWidth < 0 || snapshot.Browser.AvailableWidth > 32768 ||
		snapshot.Browser.AvailableHeight < 0 || snapshot.Browser.AvailableHeight > 32768 ||
		snapshot.Browser.ColorDepth < 0 || snapshot.Browser.ColorDepth > 128 ||
		snapshot.Browser.PixelRatio < 0 || snapshot.Browser.PixelRatio > 16 ||
		snapshot.Browser.HardwareConcurrency < 0 || snapshot.Browser.HardwareConcurrency > 1024 ||
		snapshot.Browser.DeviceMemory < 0 || snapshot.Browser.DeviceMemory > 1024 ||
		snapshot.Browser.MaxTouchPoints < 0 || snapshot.Browser.MaxTouchPoints > 100 ||
		snapshot.Browser.Fonts.DetectedCount < 0 || snapshot.Browser.Fonts.DetectedCount > 100_000 {
		err = fmt.Errorf("numeric browser signal is outside accepted range")
		return
	}

	if expected, err = IdentifySnapshotForMode(snapshot.Browser, mode); err != nil {
		return
	}

	if snapshot.SnapshotID != expected {
		err = fmt.Errorf("snapshot identifier does not match signal content")
	}

	return
}
