//go:build js && wasm

package service

import (
	"sort"
	"strings"
	"syscall/js"
)

func canvasSignals(global js.Value) (supported bool, hash string) {
	defer func() {
		if recover() != nil {
			supported = false
			hash = ""
		}
	}()

	var (
		canvas  js.Value = global.Get("document").Call("createElement", "canvas")
		context js.Value
	)

	canvas.Set("width", 300)
	canvas.Set("height", 80)
	context = canvas.Call("getContext", "2d")
	if context.IsNull() || context.IsUndefined() {
		return
	}

	context.Set("textBaseline", "alphabetic")
	context.Set("fillStyle", "#f60")
	context.Call("fillRect", 12, 14, 96, 38)
	context.Set("fillStyle", "#069")
	context.Set("font", "18px Arial")
	context.Call("fillText", "Sigil Ω漢🔒", 8, 42)
	context.Set("globalCompositeOperation", "multiply")
	context.Set("fillStyle", "rgba(102, 204, 0, 0.7)")
	context.Call("beginPath")
	context.Call("arc", 128, 34, 22, 0, 2*3.141592653589793)
	context.Call("fill")

	supported = true
	hash = digestSignal(canvas.Call("toDataURL").String())
	return
}

func webGLParameter(context js.Value, parameter int) (result string) {
	var value js.Value = context.Call("getParameter", parameter)

	if value.Type() == js.TypeString {
		result = value.String()
	}

	return
}

func webGLExtensions(context js.Value) (hash string) {
	var (
		values     js.Value = context.Call("getSupportedExtensions")
		extensions []string
	)

	if values.IsNull() || values.IsUndefined() {
		return
	}

	extensions = stringArray(values)
	sort.Strings(extensions)
	hash = digestSignal(strings.Join(extensions, "\n"))
	return
}

func webGLSignals(global js.Value) (supported bool, vendor, renderer, extensionsHash string) {
	defer func() {
		if recover() != nil {
			supported = false
			vendor = ""
			renderer = ""
			extensionsHash = ""
		}
	}()

	var (
		canvas                           js.Value = global.Get("document").Call("createElement", "canvas")
		context                          js.Value = canvas.Call("getContext", "webgl")
		debugInfo                        js.Value
		unmaskedVendor, unmaskedRenderer string
	)

	if context.IsNull() || context.IsUndefined() {
		context = canvas.Call("getContext", "experimental-webgl")
	}

	if context.IsNull() || context.IsUndefined() {
		return
	}

	vendor = webGLParameter(context, 7936)
	renderer = webGLParameter(context, 7937)
	debugInfo = context.Call("getExtension", "WEBGL_debug_renderer_info")

	if !debugInfo.IsNull() && !debugInfo.IsUndefined() {
		if unmaskedVendor = webGLParameter(context, debugInfo.Get("UNMASKED_VENDOR_WEBGL").Int()); unmaskedVendor != "" {
			vendor = unmaskedVendor
		}

		if unmaskedRenderer = webGLParameter(context, debugInfo.Get("UNMASKED_RENDERER_WEBGL").Int()); unmaskedRenderer != "" {
			renderer = unmaskedRenderer
		}
	}

	supported = true
	extensionsHash = webGLExtensions(context)
	return
}

// collectRendering gathers canvas and WebGL evidence without retaining rendered pixels.
func collectRendering(global js.Value) (signals RenderingSignals) {
	signals.CanvasSupported, signals.CanvasHash = canvasSignals(global)
	signals.WebGLSupported, signals.WebGLVendor, signals.WebGLRenderer, signals.WebGLExtensionsHash = webGLSignals(global)
	return
}
