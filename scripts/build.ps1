$env:GOOS = "js"
$env:GOARCH = "wasm"

try {
    go build -o ./client/public/main.wasm ./client/src

    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
} finally {
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
}

