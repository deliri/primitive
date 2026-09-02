//go:build !darwin && !linux && !windows

package hostfacts

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
)

type platformRoot struct{}

func diskOpenIdentity() core.ErrorIdentity {
	return core.ErrDiskCapacityUnsupported
}

func openRoot(context.Context, core.AbsolutePath) (*platformRoot, error) {
	return nil, core.ErrHostFactsUnsupported
}

func (*platformRoot) close() error {
	return core.ErrHostFactsUnsupported
}

func (*platformRoot) diskCapacity() (DiskCapacity, error) {
	return DiskCapacity{}, core.ErrDiskCapacityUnsupported
}
