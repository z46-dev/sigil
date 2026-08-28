//go:build js && wasm

package service

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"
	"time"
)

type audioResult struct {
	signals AudioSignals
	err     error
}

func audioConstructor(global js.Value) (constructor js.Value) {
	constructor = global.Get("OfflineAudioContext")
	if constructor.Type() != js.TypeFunction {
		constructor = global.Get("webkitOfflineAudioContext")
	}

	return
}

func hashAudioBuffer(buffer js.Value) (hash string) {
	var (
		samples js.Value = buffer.Call("getChannelData", 0)
		encoded []byte   = make([]byte, samples.Length()*4)
	)

	for index := range samples.Length() {
		binary.LittleEndian.PutUint32(encoded[index*4:], math.Float32bits(float32(samples.Index(index).Float())))
	}

	hash = digestBytes(encoded)
	return
}

func configureAudioGraph(context js.Value) {
	var oscillator, compressor js.Value = context.Call("createOscillator"), context.Call("createDynamicsCompressor")

	oscillator.Set("type", "triangle")
	oscillator.Get("frequency").Set("value", 10000)
	compressor.Get("threshold").Set("value", -50)
	compressor.Get("knee").Set("value", 40)
	compressor.Get("ratio").Set("value", 12)
	compressor.Get("attack").Set("value", 0)
	compressor.Get("release").Set("value", 0.25)
	oscillator.Call("connect", compressor)
	compressor.Call("connect", context.Get("destination"))
	oscillator.Call("start", 0)
}

// collectAudio renders a fixed offline graph without using microphone permissions.
func collectAudio(global js.Value) (signals AudioSignals, err error) {
	defer func() {
		var recovered any = recover()

		if recovered != nil {
			signals = AudioSignals{}
			err = fmt.Errorf("offline audio rendering is unavailable")
		}
	}()

	var constructor js.Value = audioConstructor(global)
	if constructor.Type() != js.TypeFunction {
		return
	}

	var (
		context          js.Value         = constructor.New(1, 5000, 44100)
		completed        chan audioResult = make(chan audioResult, 1)
		success, failure js.Func
		finished         bool
		result           audioResult
	)

	configureAudioGraph(context)
	success = js.FuncOf(func(this js.Value, args []js.Value) (o any) {
		if len(args) == 0 {
			completed <- audioResult{err: fmt.Errorf("offline audio rendering returned no buffer")}
		} else {
			completed <- audioResult{signals: AudioSignals{Supported: true, Hash: hashAudioBuffer(args[0])}}
		}

		return
	})

	failure = js.FuncOf(func(this js.Value, args []js.Value) (o any) {
		completed <- audioResult{signals: AudioSignals{Supported: true}, err: fmt.Errorf("offline audio rendering failed")}
		return
	})

	context.Call("startRendering").Call("then", success, failure)

	select {
	case result = <-completed:
		finished = true
	case <-time.After(3 * time.Second):
		result = audioResult{
			signals: AudioSignals{
				Supported: true,
			},
			err: fmt.Errorf("offline audio rendering timed out"),
		}
	}

	if finished {
		success.Release()
		failure.Release()
	}

	signals = result.signals
	err = result.err
	return
}
