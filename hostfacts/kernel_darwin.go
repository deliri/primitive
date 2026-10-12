//go:build darwin

package hostfacts

import (
	"errors"
	"syscall"
)

func observeKernel() (KernelObservation, error) {
	system, systemErr := syscall.Sysctl("kern.ostype")
	release, releaseErr := syscall.Sysctl("kern.osrelease")
	machine, machineErr := syscall.Sysctl("hw.machine")
	if err := errors.Join(systemErr, releaseErr, machineErr); err != nil {
		return KernelObservation{}, err
	}
	return KernelObservation{System: KernelText{value: system}, Release: KernelText{value: release}, Machine: KernelText{value: machine}}, nil
}
