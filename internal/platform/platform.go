package platform

import "runtime"

// Detect returns the current OS identifier (runtime.GOOS).
func Detect() string {
	return runtime.GOOS
}

// Get returns the platform override if non-empty, otherwise the detected OS.
func Get(override string) string {
	if override != "" {
		return override
	}
	return Detect()
}
