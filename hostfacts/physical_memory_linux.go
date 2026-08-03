//go:build linux

package hostfacts

import (
	"errors"
	"math"

	"golang.org/x/sys/unix"
)

func observePhysicalMemory() (uint64, error) {
	var information unix.Sysinfo_t
	if err := unix.Sysinfo(&information); err != nil {
		return 0, err
	}
	total := uint64(information.Totalram)
	unit := uint64(information.Unit)
	if unit != 0 && total > math.MaxUint64/unit {
		return 0, errors.New("physical memory total overflows uint64")
	}
	return total * unit, nil
}
