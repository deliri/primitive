package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type workloadMemoryScopeParentFiles struct {
	membership core.AbsolutePath
	mounts     core.AbsolutePath
}

func (f workloadMemoryScopeParentFiles) Validate() error {
	return errors.Join(f.membership.Validate(), f.mounts.Validate())
}

// The paths name native proc files. Only the control-group record is lent;
// tests use owned files, not a fabricated observation or execution callback.
func observeWorkloadMemoryScopeParentAt(ctx context.Context, files workloadMemoryScopeParentFiles, data []byte) (WorkloadMemoryHeadroom, error) {
	if err := errors.Join(contextstate.Validate(ctx), files.Validate()); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	target, present, err := parseWorkloadMemoryScopeParent(data)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if !present {
		return unavailableWorkloadMemoryHeadroom()
	}
	current, found, err := observeCgroupMembership(ctx, files.membership)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if !found {
		return unavailableWorkloadMemoryHeadroom()
	}
	future := cgroupMembership{path: target, source: current.source}
	mount, err := selectCgroupMount(ctx, files.mounts, future)
	if errors.Is(err, core.ErrCgroupMountMissing) {
		return unavailableWorkloadMemoryHeadroom()
	}
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if err := admitWorkloadMemoryScopeParentDirectory(ctx, future, mount); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return unavailableWorkloadMemoryHeadroom()
		}
		return WorkloadMemoryHeadroom{}, err
	}
	result, err := foldWorkloadMemoryHeadroom(ctx, future, mount)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if err := recheckCgroupMembership(ctx, files.membership, current); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return validateWorkloadMemoryHeadroom(result)
}

func unavailableWorkloadMemoryHeadroom() (WorkloadMemoryHeadroom, error) {
	binding, err := unavailableWorkloadMemoryLimit()
	return WorkloadMemoryHeadroom{binding: binding}, err
}

func admitWorkloadMemoryScopeParentDirectory(ctx context.Context, membership cgroupMembership, mount cgroupMount) error {
	directory, err := resolveCgroupDirectory(membership.path, mount)
	if err != nil {
		return err
	}
	root, err := filestore.OpenRoot(ctx, directory)
	if err != nil {
		return err
	}
	return filestore.CloseRoot(filestore.RootHandleRequest{Root: root})
}

func parseWorkloadMemoryScopeParent(data []byte) (string, bool, error) {
	if len(data) > procLineMaximumBytes {
		return "", false, core.ErrHostFactsObservation
	}
	value := strings.TrimSuffix(string(data), "\n")
	if value == "" {
		return "", false, nil
	}
	if strings.ContainsAny(value, "\r\n") || !validCgroupPath(value) {
		return "", false, core.ErrHostFactsObservation
	}
	return value, true, nil
}

func sameWorkloadMemoryScopeParent(before, after []byte) error {
	a, ap, ae := parseWorkloadMemoryScopeParent(before)
	b, bp, be := parseWorkloadMemoryScopeParent(after)
	if err := errors.Join(ae, be); err != nil {
		return err
	}
	if a != b || ap != bp {
		return core.ErrHostFactsObservation
	}
	return nil
}
