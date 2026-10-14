//go:build linux

package hostfacts

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

const (
	procSelfCgroupValue = "/proc/self/cgroup"
	procSelfMountsValue = "/proc/self/mountinfo"
)

func procSelfCgroupPath() (core.AbsolutePath, error) {
	return core.ParseAbsolutePath(procSelfCgroupValue)
}

func procSelfMountsPath() (core.AbsolutePath, error) {
	return core.ParseAbsolutePath(procSelfMountsValue)
}

func observeEffectiveWorkloadMemoryLimit(ctx context.Context) (WorkloadMemoryLimit, error) {
	procPath, err := procSelfCgroupPath()
	if err != nil {
		return WorkloadMemoryLimit{}, err
	}
	mountsPath, err := procSelfMountsPath()
	if err != nil {
		return WorkloadMemoryLimit{}, err
	}
	return observeWorkloadMemoryAt(ctx, procPath, mountsPath)
}
