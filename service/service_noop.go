//go:build !js || !wasm

package service

import "fmt"

// Collect reports that browser signal collection is only available in WebAssembly.
func Collect() (snapshot Snapshot, err error) {
	err = fmt.Errorf("browser signal collection requires a js/wasm build")
	return
}

// CollectAudio reports that audio collection is only available in WebAssembly.
func CollectAudio(snapshot Snapshot) (updated Snapshot, err error) {
	updated = snapshot
	err = fmt.Errorf("audio collection requires a js/wasm build")
	return
}
