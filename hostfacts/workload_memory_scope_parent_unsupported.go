//go:build !linux

package hostfacts

import "context"

func observeWorkloadMemoryScopeParent(context.Context) (WorkloadMemoryHeadroom, error) {
	binding, err := unsupportedWorkloadMemoryLimit()
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return validateWorkloadMemoryHeadroom(WorkloadMemoryHeadroom{binding: binding})
}
