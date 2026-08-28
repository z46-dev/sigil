#!/usr/bin/env bash

set -euo pipefail

GOOS=js GOARCH=wasm go build -o ./client/public/main.wasm ./client/src
