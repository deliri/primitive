package process

import "os"

// DiscardDeviceArgument returns the platform-owned null-device identity as a
// validated command argument. Command capabilities use it when a compiler or
// linker requires an output path but the artifact itself is not evidence.
func DiscardDeviceArgument() (Argument, error) {
	return NewArgument(os.DevNull)
}
