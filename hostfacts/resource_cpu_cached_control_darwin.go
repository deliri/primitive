//go:build darwin

package hostfacts

import (
	"runtime"
	"unsafe"

	"github.com/deliri/primitive/v2026/core"
)

// CONTROL ONLY: restore the old cached HOST_CPU_LOAD_INFO route while keeping
// the identical accepted snapshot shape, comparison, and OOL/host cleanup.
// Aggregate ticks occupy one row; the remaining rows contribute zero. These
// are an old-route projection, never claimed as real per-CPU observations.
func readCachedResourceCPU(buffer resourceMachCPUBuffer) (resourceMachCPUCounters, error) {
	out, err := buffer.snapshot()
	if err != nil {
		return nil, err
	}
	var cached resourceMachCPUState
	count := uint32(resourceMachCPUStates)
	status, _, native := resourceMachCall(resourceMachCachedStatisticsAddr, buffer.host, 3, uintptr(unsafe.Pointer(&cached[0])), uintptr(unsafe.Pointer(&count)), 0, 0)
	runtime.KeepAlive(&cached)
	runtime.KeepAlive(&count)
	if err := resourceMachResult(status, native); err != nil {
		return nil, err
	}
	if count != resourceMachCPUStates || cached == (resourceMachCPUState{}) {
		return nil, core.ErrHostFactsObservation
	}
	for i := range out {
		out[i] = resourceMachCPUState{}
	}
	out[0] = cached
	return out, nil
}

var resourceMachCachedStatisticsAddr uintptr

//go:cgo_import_dynamic resource_control_host_statistics host_statistics "/usr/lib/libSystem.B.dylib"
