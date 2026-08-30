//go:build js && wasm

package service

import (
	"fmt"
	"sort"
	"strings"
	"syscall/js"
	"time"

	"github.com/z46-dev/sigil/service/wgsl"
)

type webGPUPromiseResult struct {
	value js.Value
	err   error
}

func awaitWebGPUPromise(promise js.Value) (value js.Value, err error) {
	var (
		completed        chan webGPUPromiseResult = make(chan webGPUPromiseResult, 1)
		success, failure js.Func
		finished         bool
		result           webGPUPromiseResult
	)

	success = js.FuncOf(func(this js.Value, args []js.Value) (output any) {
		if len(args) == 0 {
			completed <- webGPUPromiseResult{value: js.Undefined()}
		} else {
			completed <- webGPUPromiseResult{value: args[0]}
		}

		return
	})
	failure = js.FuncOf(func(this js.Value, args []js.Value) (output any) {
		completed <- webGPUPromiseResult{err: fmt.Errorf("WebGPU promise was rejected")}
		return
	})
	promise.Call("then", success, failure)

	select {
	case result = <-completed:
		finished = true
	case <-time.After(3 * time.Second):
		result.err = fmt.Errorf("WebGPU operation timed out")
	}

	if finished {
		success.Release()
		failure.Release()
	}

	value = result.value
	err = result.err
	return
}

func webGPUAdapterInfo(adapter js.Value) (vendor, architecture, device, description string) {
	var info js.Value = adapter.Get("info")

	if info.Type() != js.TypeObject {
		return
	}

	vendor = stringProperty(info, "vendor")
	architecture = stringProperty(info, "architecture")
	device = stringProperty(info, "device")
	description = stringProperty(info, "description")
	return
}

func webGPUFeaturesHash(adapter js.Value) (hash string) {
	var (
		values   js.Value = js.Global().Get("Array").Call("from", adapter.Get("features"))
		features []string = stringArray(values)
	)

	sort.Strings(features)
	if len(features) > 0 {
		hash = digestSignal(strings.Join(features, "\n"))
	}

	return
}

func webGPULimitsHash(adapter js.Value) (hash string) {
	var (
		limits js.Value = adapter.Get("limits")
		keys   []string = []string{
			"maxTextureDimension1D", "maxTextureDimension2D", "maxTextureDimension3D",
			"maxTextureArrayLayers", "maxBindGroups", "maxBindGroupsPlusVertexBuffers",
			"maxBindingsPerBindGroup", "maxDynamicUniformBuffersPerPipelineLayout",
			"maxDynamicStorageBuffersPerPipelineLayout", "maxSampledTexturesPerShaderStage",
			"maxSamplersPerShaderStage", "maxStorageBuffersPerShaderStage",
			"maxStorageTexturesPerShaderStage", "maxUniformBuffersPerShaderStage",
			"maxUniformBufferBindingSize", "maxStorageBufferBindingSize",
			"minUniformBufferOffsetAlignment", "minStorageBufferOffsetAlignment",
			"maxVertexBuffers", "maxBufferSize", "maxVertexAttributes",
			"maxVertexBufferArrayStride", "maxInterStageShaderVariables",
			"maxColorAttachments", "maxColorAttachmentBytesPerSample",
			"maxComputeWorkgroupStorageSize", "maxComputeInvocationsPerWorkgroup",
			"maxComputeWorkgroupSizeX", "maxComputeWorkgroupSizeY", "maxComputeWorkgroupSizeZ",
			"maxComputeWorkgroupsPerDimension",
		}
		parts []string = make([]string, 0, len(keys))
	)

	for _, key := range keys {
		if value := limits.Get(key); value.Type() == js.TypeNumber {
			parts = append(parts, fmt.Sprintf("%s=%.0f", key, value.Float()))
		}
	}

	if len(parts) > 0 {
		hash = digestSignal(strings.Join(parts, "\n"))
	}

	return
}

func webGPURender(device js.Value) (hash string, err error) {
	var (
		texture js.Value = device.Call("createTexture", map[string]any{
			"size":   []any{64, 64, 1},
			"format": "rgba8unorm",
			"usage":  1 | 16,
		})
		buffer js.Value = device.Call("createBuffer", map[string]any{
			"size":  64 * 64 * 4,
			"usage": 1 | 8,
		})
		module js.Value = device.Call("createShaderModule", map[string]any{"code": wgsl.WebGPUFingerprint})
		pipe   js.Value = device.Call("createRenderPipeline", map[string]any{
			"layout": "auto",
			"vertex": map[string]any{"module": module, "entryPoint": "vertex"},
			"fragment": map[string]any{
				"module": module, "entryPoint": "fragment",
				"targets": []any{map[string]any{"format": "rgba8unorm"}},
			},
			"primitive": map[string]any{"topology": "triangle-list"},
		})
		encoder js.Value = device.Call("createCommandEncoder")
		pass    js.Value = encoder.Call("beginRenderPass", map[string]any{
			"colorAttachments": []any{map[string]any{
				"view": texture.Call("createView"), "loadOp": "clear", "storeOp": "store",
				"clearValue": map[string]any{"r": 0, "g": 0, "b": 0, "a": 1},
			}},
		})
		mapped js.Value
		pixels []byte = make([]byte, 64*64*4)
	)

	pass.Call("setPipeline", pipe)
	pass.Call("draw", 3)
	pass.Call("end")
	encoder.Call("copyTextureToBuffer", map[string]any{"texture": texture}, map[string]any{
		"buffer": buffer, "bytesPerRow": 256, "rowsPerImage": 64,
	}, []any{64, 64, 1})
	device.Get("queue").Call("submit", []any{encoder.Call("finish")})

	if _, err = awaitWebGPUPromise(buffer.Call("mapAsync", 1)); err != nil {
		return
	}

	mapped = js.Global().Get("Uint8Array").New(buffer.Call("getMappedRange"))
	js.CopyBytesToGo(pixels, mapped)
	buffer.Call("unmap")
	buffer.Call("destroy")
	texture.Call("destroy")
	hash = digestBytes(pixels)
	return
}

// webGPUCompute hashes raw results from a deterministic floating-point compute workload.
func webGPUCompute(device js.Value) (hash string, err error) {
	const bufferSize int = 1024 * 4

	var (
		storage js.Value = device.Call("createBuffer", map[string]any{
			"size":  bufferSize,
			"usage": 128 | 4,
		})
		readback js.Value = device.Call("createBuffer", map[string]any{
			"size":  bufferSize,
			"usage": 1 | 8,
		})
		module js.Value = device.Call("createShaderModule", map[string]any{"code": wgsl.WebGPUComputeFingerprint})
		pipe   js.Value = device.Call("createComputePipeline", map[string]any{
			"layout":  "auto",
			"compute": map[string]any{"module": module, "entryPoint": "compute"},
		})
		bindGroup js.Value = device.Call("createBindGroup", map[string]any{
			"layout": pipe.Call("getBindGroupLayout", 0),
			"entries": []any{map[string]any{
				"binding":  0,
				"resource": map[string]any{"buffer": storage},
			}},
		})
		encoder js.Value = device.Call("createCommandEncoder")
		pass    js.Value = encoder.Call("beginComputePass")
		mapped  js.Value
		output  []byte = make([]byte, bufferSize)
	)

	pass.Call("setPipeline", pipe)
	pass.Call("setBindGroup", 0, bindGroup)
	pass.Call("dispatchWorkgroups", 16)
	pass.Call("end")
	encoder.Call("copyBufferToBuffer", storage, 0, readback, 0, bufferSize)
	device.Get("queue").Call("submit", []any{encoder.Call("finish")})

	if _, err = awaitWebGPUPromise(readback.Call("mapAsync", 1)); err != nil {
		return
	}

	mapped = js.Global().Get("Uint8Array").New(readback.Call("getMappedRange"))
	js.CopyBytesToGo(output, mapped)
	readback.Call("unmap")
	readback.Call("destroy")
	storage.Call("destroy")
	hash = digestBytes(output)
	return
}

// collectWebGPU gathers adapter capabilities and hashes a deterministic offscreen render.
func collectWebGPU(global js.Value) (signals RenderingSignals, err error) {
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("WebGPU collection failed")
		}
	}()

	var (
		gpu     js.Value = global.Get("navigator").Get("gpu")
		adapter js.Value
		device  js.Value
	)

	if gpu.Type() != js.TypeObject {
		return
	}

	if adapter, err = awaitWebGPUPromise(gpu.Call("requestAdapter")); err != nil || adapter.IsNull() || adapter.IsUndefined() {
		return
	}

	signals.WebGPUSupported = true
	signals.WebGPUVendor, signals.WebGPUArchitecture, signals.WebGPUDevice, signals.WebGPUDescription = webGPUAdapterInfo(adapter)
	signals.WebGPUFeaturesHash = webGPUFeaturesHash(adapter)
	signals.WebGPULimitsHash = webGPULimitsHash(adapter)

	if device, err = awaitWebGPUPromise(adapter.Call("requestDevice")); err != nil {
		return
	}

	signals.WebGPURenderHash, err = webGPURender(device)
	if err == nil {
		signals.WebGPUComputeHash, err = webGPUCompute(device)
	}

	device.Call("destroy")
	return
}
