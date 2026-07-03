//go:build android
// +build android

// Android JNI stub — replaces the desktop main.go when the binary is
// cross-compiled for Android (e.g. `GOOS=android GOARCH=arm64 go build`).
//
// Android uses a different entry point (`Java_com_..._startZigLogic` in
// the Zig source).  The Go port of that JNI surface is Phase 2 work —
// until then, panicking is the safest behaviour: a developer who
// tries to boot the server binary on Android gets a clear "wrong
// binary" message instead of a confusing NDK linker error.
//
// See `docs/adr/` for the long-term plan (probably: a tiny NDK
// shim that forwards to this stub, OR a separate Android project that
// pulls in the same `internal/...` packages).
package main

import "os"

func main() {
	const msg = "little-timer: Android JNI is not implemented in the Go port — " +
		"use the separate Android project (see docs/adr/). " +
		"This binary is intended for desktop (Linux/Windows/macOS) only."
	_, _ = os.Stderr.WriteString(msg + "\n")
	panic(msg)
}