package hostfacts

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func recheckCgroupMembership(ctx context.Context, procPath core.AbsolutePath, membership cgroupMembership) error {
	after, found, err := observeCgroupMembership(ctx, procPath)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrCgroupMembershipDisappeared
	}
	if after != membership {
		return core.ErrCgroupMembershipChanged
	}
	return nil
}

func selectCgroupMount(ctx context.Context, mountsPath core.AbsolutePath, membership cgroupMembership) (cgroupMount, error) {
	var selection cgroupMountSelection
	err := scanVirtualLines(ctx, virtualFileRequest{Path: mountsPath, MaximumBytes: virtualFileMaximumBytes}, func(line []byte) error {
		mount, matches, parseErr := parseMountInfoLine(line, membership.source)
		if parseErr != nil {
			return parseErr
		}
		if matches {
			return selection.consider(mount, membership)
		}
		return nil
	})
	if err != nil {
		return cgroupMount{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	if selection.count == 0 {
		return cgroupMount{}, core.ErrCgroupMountMissing
	}
	if selection.count != 1 {
		return cgroupMount{}, core.ErrCgroupMountAmbiguous
	}
	return selection.selected, selection.selected.Validate()
}
