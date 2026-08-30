package wgsl

import _ "embed"

//go:embed webgpu_fingerprint.wgsl
var WebGPUFingerprint string

//go:embed webgpu_compute_fingerprint.wgsl
var WebGPUComputeFingerprint string
