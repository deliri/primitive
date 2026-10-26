//go:build !linux

package process

import "github.com/deliri/primitive/v2026/core"

// cappedCommand runs path as it is; this platform has no per-tree memory
// limit, so a nonzero max is refused rather than silently ignored.
func cappedCommand(max core.ByteLength, path string, arguments []string) (string, []string, error) {
	if max.Uint64() != 0 {
		return "", nil, core.ErrProcessUnsupported
	}
	return path, arguments, nil
}
