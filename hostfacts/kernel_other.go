//go:build !linux && !darwin

package hostfacts

import "github.com/deliri/primitive/v2026/core"

func observeKernel() (KernelObservation, error) {
	return KernelObservation{}, core.ErrHostFactsUnsupported
}
