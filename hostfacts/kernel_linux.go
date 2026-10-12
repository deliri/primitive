//go:build linux

package hostfacts

import (
	"errors"
	"syscall"
)

func observeKernel() (KernelObservation, error) {
	var native syscall.Utsname
	if err := syscall.Uname(&native); err != nil {
		return KernelObservation{}, err
	}
	var fields [3][65]byte
	for i := range native.Sysname {
		fields[0][i] = byte(native.Sysname[i])
		fields[1][i] = byte(native.Release[i])
		fields[2][i] = byte(native.Machine[i])
	}
	system, systemErr := admitKernelField(fields[0][:])
	release, releaseErr := admitKernelField(fields[1][:])
	machine, machineErr := admitKernelField(fields[2][:])
	if err := errors.Join(systemErr, releaseErr, machineErr); err != nil {
		return KernelObservation{}, err
	}
	return KernelObservation{System: system, Release: release, Machine: machine}, nil
}
