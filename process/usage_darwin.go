//go:build darwin

package process

import (
	"errors"
	"os"
	"syscall"

	"github.com/deliri/primitive/v2026/core"
)

func peakMemoryBytes(state *os.ProcessState) (core.ByteLength, error) {
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage.Maxrss < 0 {
		return core.ByteLength{}, errors.Join(core.ErrProcessContract, core.ErrNumericOverflow)
	}
	return core.NewByteLength(uint64(usage.Maxrss))
}
