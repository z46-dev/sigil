//go:build js && wasm

package service

import (
	"fmt"
	"strings"
	"syscall/js"
	"time"
)

func stringProperty(value js.Value, property string) (result string) {
	if value = value.Get(property); value.Type() == js.TypeString {
		result = value.String()
	}

	return
}

func numberProperty(value js.Value, property string) (result float64) {
	if value = value.Get(property); value.Type() == js.TypeNumber {
		result = value.Float()
	}

	return
}

func boolProperty(value js.Value, property string) (result bool) {
	if value = value.Get(property); value.Type() == js.TypeBoolean {
		result = value.Bool()
	}

	return
}

func stringArray(value js.Value) (result []string) {
	result = make([]string, 0, value.Length())

	for index := 0; index < value.Length(); index++ {
		if value.Index(index).Type() == js.TypeString {
			result = append(result, value.Index(index).String())
		}
	}

	return
}

func storageAvailable(global js.Value, name string) (available bool) {
	defer func() {
		if recover() != nil {
			available = false
		}
	}()

	available = global.Get(name).Type() == js.TypeObject
	return
}

func timezone(global js.Value) (name string) {
	defer func() {
		if recover() != nil {
			name = ""
		}
	}()

	name = global.Get("Intl").Get("DateTimeFormat").New().Call("resolvedOptions").Get("timeZone").String()
	return
}

func normalizedPlatform(navigator js.Value) (platform string) {
	var (
		userAgentData       js.Value = navigator.Get("userAgentData")
		userAgent           string   = strings.ToLower(stringProperty(navigator, "userAgent"))
		candidate, evidence string
	)

	if userAgentData.Type() == js.TypeObject {
		candidate = stringProperty(userAgentData, "platform")
	}

	if candidate == "" {
		candidate = stringProperty(navigator, "platform")
	}

	candidate = strings.ToLower(candidate)
	evidence = candidate + " " + userAgent

	switch {
	case strings.Contains(evidence, "android"):
		platform = "android"
	case strings.Contains(evidence, "iphone"), strings.Contains(evidence, "ipad"), strings.Contains(evidence, "ipod"):
		platform = "ios"
	case strings.Contains(evidence, "cros"):
		platform = "chromeos"
	case strings.Contains(evidence, "win"):
		platform = "windows"
	case strings.Contains(evidence, "mac"):
		if numberProperty(navigator, "maxTouchPoints") > 1 {
			platform = "ios"
		} else {
			platform = "macos"
		}
	case strings.Contains(evidence, "linux"):
		platform = "linux"
	default:
		platform = strings.TrimSpace(strings.ToLower(candidate))
	}

	return
}

// Collect observes the browser properties available without requesting permissions.
func Collect() (snapshot Snapshot, err error) {
	var (
		global    js.Value = js.Global()
		navigator js.Value = global.Get("navigator")
		screen    js.Value = global.Get("screen")
		languages js.Value = navigator.Get("languages")
		signals   BrowserSignals
	)

	if navigator.Type() != js.TypeObject || screen.Type() != js.TypeObject {
		err = fmt.Errorf("browser navigator or screen API is unavailable")
		return
	}

	signals = BrowserSignals{
		UserAgent:           stringProperty(navigator, "userAgent"),
		Platform:            normalizedPlatform(navigator),
		Vendor:              stringProperty(navigator, "vendor"),
		Timezone:            timezone(global),
		ScreenWidth:         int(numberProperty(screen, "width")),
		ScreenHeight:        int(numberProperty(screen, "height")),
		AvailableWidth:      int(numberProperty(screen, "availWidth")),
		AvailableHeight:     int(numberProperty(screen, "availHeight")),
		ColorDepth:          int(numberProperty(screen, "colorDepth")),
		PixelRatio:          numberProperty(global, "devicePixelRatio"),
		HardwareConcurrency: int(numberProperty(navigator, "hardwareConcurrency")),
		DeviceMemory:        numberProperty(navigator, "deviceMemory"),
		MaxTouchPoints:      int(numberProperty(navigator, "maxTouchPoints")),
		CookiesEnabled:      boolProperty(navigator, "cookieEnabled"),
		DoNotTrack:          stringProperty(navigator, "doNotTrack"),
		WebDriver:           boolProperty(navigator, "webdriver"),
		LocalStorage:        storageAvailable(global, "localStorage"),
		SessionStorage:      storageAvailable(global, "sessionStorage"),
		Rendering:           collectRendering(global),
		Fonts:               collectFonts(global),
	}

	if languages.Type() == js.TypeObject {
		signals.Languages = stringArray(languages)
	} else {
		var language string

		if language = stringProperty(navigator, "language"); language != "" {
			signals.Languages = []string{language}
		}
	}

	snapshot = Snapshot{
		SchemaVersion: SchemaVersion,
		Scope:         ModeDeviceAndBrowser.String(),
		CollectedAt:   time.Now().UTC(),
		Browser:       signals,
		mode:          ModeDeviceAndBrowser,
	}

	snapshot.SnapshotID, err = IdentifySnapshot(signals)
	return
}

// CollectAudio adds asynchronous audio evidence and refreshes the snapshot identifier.
func CollectAudio(snapshot Snapshot) (updated Snapshot, err error) {
	var identifyErr error

	updated = snapshot
	updated.Browser.Audio, err = collectAudio(js.Global())
	if updated.SnapshotID, identifyErr = IdentifySnapshotForMode(updated.Browser, updated.mode); identifyErr != nil {
		err = identifyErr
	}

	return
}
