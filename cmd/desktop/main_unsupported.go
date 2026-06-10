//go:build !windows && !darwin

package main

// This stub keeps package main compiling on platforms without a supported
// webview runtime (notably the Linux CI runner), so `go test ./...` and
// `go build ./...` stay green without a Wails/CGO toolchain. The real desktop
// app (main.go) builds only on Windows and macOS — see ADR-001.

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "kaimi-desktop builds and runs on Windows and macOS only (see docs/desktop/).")
	os.Exit(1)
}
