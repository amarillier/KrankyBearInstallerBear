//go:build !linux

package main

// guiDisplayAvailable is always true on macOS/Windows: neither has the
// "sitting on a headless server with no X11/Wayland session" scenario this
// check exists for on Linux (see gui_display_linux.go).
func guiDisplayAvailable() bool { return true }
