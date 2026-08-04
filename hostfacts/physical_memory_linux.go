//go:build linux

package hostfacts

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

func observePhysicalMemory() (uint64, error) {
	var information unix.Sysinfo_t
	if err := unix.Sysinfo(&information); err != nil {
		return 0, err
	}
	return physicalMemoryBytes(
		uint64(information.Totalram),
		uint64(information.Unit),
	)
}

const (
	physicalMemoryUnitZeroErrorText = "physical memory unit is zero"
	physicalMemoryOverflowErrorText = "physical memory total overflows uint64"
)

// physicalMemoryBytes scales the kernel's unit-relative total into bytes. The
// unit is a divisor in the overflow guard, so a zero unit is rejected before
// the guard rather than allowed to skip it.
func physicalMemoryBytes(total, unit uint64) (uint64, error) {
	if unit == 0 {
		return 0, errors.Join(
			core.ErrHostFactsObservation,
			errors.New(physicalMemoryUnitZeroErrorText),
		)
	}
	if total > math.MaxUint64/unit {
		return 0, errors.Join(
			core.ErrHostFactsObservation,
			errors.New(physicalMemoryOverflowErrorText),
		)
	}
	return total * unit, nil
}
