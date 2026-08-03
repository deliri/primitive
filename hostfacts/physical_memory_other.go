//go:build !darwin && !linux

package hostfacts

import "github.com/deliri/primitive/v2026/core"

func observePhysicalMemory() (uint64, error) {
	return 0, core.ErrHostFactsUnsupported
}
