//go:build linux

package main

import "os"

// guiDisplayAvailable reports whether the environment is likely to support a
// GLFW/Fyne window. Headless servers and SSH sessions without display
// forwarding typically have neither variable set — same check as this
// project's sibling TaniumSensorExplorer (internal/../TaniumSensorExplorer/
// gui_display_linux.go), reused here for the same reason: without it, GLFW
// panics with a raw "NotInitialized" stack trace (confirmed on a real
// headless Ubuntu box) instead of a clean, actionable message.
func guiDisplayAvailable() bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	if os.Getenv("DISPLAY") != "" {
		return true
	}
	return false
}
