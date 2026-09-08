//go:build !linux && !darwin

package process

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

func peakMemoryBytes(*os.ProcessState) (core.ByteLength, error) {
	return core.ByteLength{}, core.ErrProcessUnsupported
}
