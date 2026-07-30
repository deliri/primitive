//go:build !darwin && !linux && !windows

package hostfacts

import (
	"os"

	"github.com/deliri/primitive/v2026/core"
)

type platformRoot struct{}

func diskOpenIdentity() core.ErrorIdentity {
	return core.ErrDiskCapacityUnsupported
}

func treeOpenIdentity() core.ErrorIdentity {
	return core.ErrTreeMeasurementUnsupported
}

func openRoot(string) (*platformRoot, error) {
	return nil, core.ErrHostFactsUnsupported
}

func (*platformRoot) close() error {
	return core.ErrHostFactsUnsupported
}

func (*platformRoot) diskCapacity() (DiskCapacity, error) {
	return DiskCapacity{}, core.ErrDiskCapacityUnsupported
}

func (*platformRoot) openDirectory(string) (*os.File, error) {
	return nil, core.ErrTreeMeasurementUnsupported
}

func (*platformRoot) inspectEntry(*os.File, string, string) (treeEntry, error) {
	return treeEntry{}, core.ErrTreeMeasurementUnsupported
}
