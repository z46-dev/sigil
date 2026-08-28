//go:build js && wasm

package service

import (
	"sort"
	"strconv"
	"strings"
	"syscall/js"
)

var fontCandidates []string = []string{
	"Arial", "Arial Black", "Calibri", "Cambria", "Candara", "Comic Sans MS",
	"Consolas", "Courier New", "DejaVu Sans", "Fira Code", "Georgia", "Helvetica",
	"Impact", "Liberation Sans", "Menlo", "Monaco", "Noto Sans", "Roboto",
	"Segoe UI", "Tahoma", "Times New Roman", "Trebuchet MS", "Ubuntu", "Verdana",
}

func textWidth(context js.Value, font, sample string) (width float64) {
	context.Set("font", font)
	width = context.Call("measureText", sample).Get("width").Float()

	return
}

func detectedFonts(context js.Value, sample string) (detected []string) {
	var (
		baseFamilies []string  = []string{"monospace", "sans-serif", "serif"}
		baseWidths   []float64 = make([]float64, len(baseFamilies))
	)

	for index, family := range baseFamilies {
		baseWidths[index] = textWidth(context, "72px "+family, sample)
	}

	for _, candidate := range fontCandidates {
		for index, family := range baseFamilies {
			if textWidth(context, "72px \""+candidate+"\","+family, sample) != baseWidths[index] {
				detected = append(detected, candidate)
				break
			}
		}
	}

	sort.Strings(detected)
	return
}

func fontMetrics(context js.Value, sample string) (hash string) {
	var (
		families []string = []string{"monospace", "sans-serif", "serif", "system-ui"}
		metrics  []string = make([]string, 0, len(families))
	)

	for _, family := range families {
		metrics = append(metrics, strconv.FormatFloat(textWidth(context, "72px "+family, sample), 'f', 6, 64))
	}

	hash = digestSignal(strings.Join(metrics, "|"))
	return
}

// collectFonts detects a conservative font set and hashes all retained output.
func collectFonts(global js.Value) (signals FontSignals) {
	defer func() {
		if recover() != nil {
			signals = FontSignals{}
		}
	}()

	var (
		canvas   js.Value = global.Get("document").Call("createElement", "canvas")
		context  js.Value = canvas.Call("getContext", "2d")
		detected []string
		sample   string = "mmmmmmmmmmlliWWΩ漢"
	)

	if context.IsNull() || context.IsUndefined() {
		return
	}

	detected = detectedFonts(context, sample)
	signals.DetectedCount = len(detected)
	signals.DetectedHash = digestSignal(strings.Join(detected, "\n"))
	signals.MetricsHash = fontMetrics(context, sample)
	return
}
