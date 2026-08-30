@vertex fn vertex(@builtin(vertex_index) index: u32) -> @builtin(position) vec4f {
    var positions = array<vec2f, 3>(vec2f(-1.0, -1.0), vec2f(3.0, -1.0), vec2f(-1.0, 3.0));
    return vec4f(positions[index], 0.0, 1.0);
}

@fragment fn fragment(@builtin(position) point: vec4f) -> @location(0) vec4f {
    let uv = point.xy / vec2f(64.0, 64.0);
    let wave = sin(uv.x * 31.0) * cos(uv.y * 29.0);
    return vec4f(fract(uv.x + wave), fract(uv.y - wave), fract(wave * 0.5 + 0.5), 1.0);
}