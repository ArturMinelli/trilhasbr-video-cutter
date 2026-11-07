# ONNX Runtime

Pinned to **1.31.0** (the C API version `github.com/yalue/onnxruntime_go` v1.38.0 is built against).
Layout: `resources/onnxruntime/<goos>-<goarch>/libonnxruntime.{dylib,so,dll}`. Only `darwin-arm64` is supported in v1.
Fetch with `scripts/fetch-onnxruntime.sh` (checksum-pinned; `./install.sh` runs it) (downloads the official release from github.com/microsoft/onnxruntime).
