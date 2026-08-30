@group(0) @binding(0) var<storage, read_write> output: array<u32>;

@compute @workgroup_size(64)
fn compute(@builtin(global_invocation_id) invocation: vec3u) {
    let index = invocation.x;
    let input = f32(index + 1u) / 1024.0;
    let trigonometric = sin(input * 91.7) * cos(input * 47.3);
    let exponential = exp2(fract(input * 13.1)) + log2(input + 1.0);
    let inverse = inverseSqrt(input + 0.03125);
    output[index] = bitcast<u32>(trigonometric * exponential + inverse);
}
