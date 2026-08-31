//go:build !linux && !darwin

package process

import (
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
)

func peakMemoryBytes(*os.ProcessState) (core.ByteLength, error) {
	return core.ByteLength{}, errors.Join(core.ErrProcessContract, errors.New("peak resident memory observation is unsupported on this host"))
}
