package hostfacts

import (
	"runtime"

	"github.com/deliri/primitive/v2026/core"
)

// CurrentPlatform returns the compiler-owned platform represented by the Go
// runtime. A runtime target outside Primitive's closed platform domain is
// rejected instead of being projected as an informal string.
func CurrentPlatform() (core.Platform, error) {
	return platformFromRuntime(runtime.GOOS, runtime.GOARCH)
}

func platformFromRuntime(operatingSystem, architecture string) (core.Platform, error) {
	var platform core.Platform
	if err := platform.UnmarshalText([]byte(operatingSystem + "-" + architecture)); err != nil {
		return core.Platform{}, err
	}
	return platform, nil
}
