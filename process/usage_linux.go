//go:build linux

package process

import (
	"errors"
	"math"
	"os"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

func peakMemoryBytes(state *os.ProcessState) (core.ByteLength, error) {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage.Maxrss < 0 || uint64(usage.Maxrss) > math.MaxUint64/1024 {
		return core.ByteLength{}, errors.Join(core.ErrProcessContract, core.ErrNumericOverflow)
	}
	return core.NewByteLength(uint64(usage.Maxrss) * 1024)
}
