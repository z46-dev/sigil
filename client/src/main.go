//go:build js && wasm

package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/z46-dev/golog"
	"github.com/z46-dev/sigil/service"
)

var (
	log                *golog.Logger = golog.New().Prefix("[SIGIL]", golog.BoldRed).Timestamp()
	collect            js.Func
	jsonParse, promise js.Value = js.Global().Get("JSON").Get("parse"), js.Global().Get("Promise")
)

func encodeSnapshot(snapshot service.Snapshot) (value js.Value, err error) {
	var encoded []byte

	if encoded, err = json.Marshal(snapshot); err == nil {
		value = jsonParse.Invoke(string(encoded))
	}

	return
}

func requestedMode(args []js.Value) (mode service.Mode, err error) {
	mode = service.ModeDeviceAndBrowser
	if len(args) == 0 || args[0].IsUndefined() || args[0].IsNull() {
		return
	}

	if args[0].Type() != js.TypeObject {
		err = fmt.Errorf("collect options must be an object")
		return
	}

	var value js.Value = args[0].Get("mode")
	if value.IsUndefined() {
		return
	}

	if value.Type() != js.TypeString {
		err = fmt.Errorf("collect mode must be a string")
		return
	}

	mode, err = service.ParseMode(value.String())
	return
}

func performCollection(resolve, reject js.Value, mode service.Mode) {
	var (
		snapshot service.Snapshot
		value    js.Value
		err      error
	)

	if snapshot, err = service.Collect(); err != nil {
		reject.Invoke(js.Global().Get("Error").New(err.Error()))
		return
	}

	if snapshot, err = service.CollectAudio(snapshot); err != nil {
		log.Infof("Audio probe unavailable: %v", err)
	}

	if snapshot, err = service.ApplyMode(snapshot, mode); err != nil {
		reject.Invoke(js.Global().Get("Error").New(err.Error()))
		return
	}

	if value, err = encodeSnapshot(snapshot); err != nil {
		reject.Invoke(js.Global().Get("Error").New(err.Error()))
		return
	}

	resolve.Invoke(value)
}

// collectSnapshot exposes an asynchronous, serializable signal snapshot to JavaScript callers.
func collectSnapshot(this js.Value, args []js.Value) (result any) {
	var (
		mode service.Mode
		err  error
	)

	if mode, err = requestedMode(args); err != nil {
		result = promise.Call("reject", js.Global().Get("TypeError").New(err.Error()))
		return
	}

	var executor js.Func = js.FuncOf(func(this js.Value, args []js.Value) (o any) {
		go performCollection(args[0], args[1], mode)
		return
	})

	result = promise.New(executor)
	executor.Release()
	return
}

func main() {
	collect = js.FuncOf(collectSnapshot)

	js.Global().Get("window").Set("sigil", js.ValueOf(map[string]any{
		"collect":       collect,
		"schemaVersion": service.SchemaVersion,
	}))

	log.Info("Browser collector ready")

	select {}
}
