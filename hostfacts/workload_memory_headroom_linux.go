//go:build linux

package hostfacts

import "context"

func observeWorkloadMemoryHeadroom(ctx context.Context) (WorkloadMemoryHeadroom, error) {
	proc, err := procSelfCgroupPath()
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	mounts, err := procSelfMountsPath()
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return observeWorkloadMemoryHeadroomAt(ctx, proc, mounts)
}
