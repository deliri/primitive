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
	selection, present, err := selectWorkloadMemoryScopeParent(ctx, files, data)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if !present {
		return unavailableWorkloadMemoryHeadroom()
	}
	if err := admitWorkloadMemoryScopeParentDirectory(ctx, selection.prospective, selection.mount); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return unavailableWorkloadMemoryHeadroom()
		}
		return WorkloadMemoryHeadroom{}, err
	}
	result, err := foldWorkloadMemoryHeadroom(ctx, selection.prospective, selection.mount)
	if err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	if err := recheckCgroupMembership(ctx, files.membership, selection.current); err != nil {
		return WorkloadMemoryHeadroom{}, err
	}
	return validateWorkloadMemoryHeadroom(result)
}

// These three native facts are lent only for this observation; there is no
// retained process population or prospective-job registry.
type workloadMemoryScopeParentSelection struct {
	current     cgroupMembership
	prospective cgroupMembership
	mount       cgroupMount
}

func (s workloadMemoryScopeParentSelection) Validate() error {
	return errors.Join(s.current.Validate(), s.prospective.Validate(), s.mount.Validate())
}

func selectWorkloadMemoryScopeParent(ctx context.Context, files workloadMemoryScopeParentFiles, data []byte) (workloadMemoryScopeParentSelection, bool, error) {
	if err := errors.Join(contextstate.Validate(ctx), files.Validate()); err != nil {
		return workloadMemoryScopeParentSelection{}, false, err
	}
	target, present, err := parseWorkloadMemoryScopeParent(data)
	if err != nil || !present {
		return workloadMemoryScopeParentSelection{}, false, err
	}
	current, found, err := observeCgroupMembership(ctx, files.membership)
	if err != nil || !found {
		return workloadMemoryScopeParentSelection{}, false, err
	}
	prospective := cgroupMembership{path: target, source: current.source}
	mount, err := selectCgroupMount(ctx, files.mounts, prospective)
	if errors.Is(err, core.ErrCgroupMountMissing) {
		return workloadMemoryScopeParentSelection{}, false, nil
	}
	if err != nil {
		return workloadMemoryScopeParentSelection{}, false, err
	}
	selection := workloadMemoryScopeParentSelection{current: current, prospective: prospective, mount: mount}
	if err := selection.Validate(); err != nil {
		return workloadMemoryScopeParentSelection{}, false, err
	}
	return selection, true, nil
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
