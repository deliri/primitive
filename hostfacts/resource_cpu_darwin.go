//go:build darwin

package hostfacts

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Darwin's host_cpu_load_info is exactly four natural_t (uint32) counters:
// user, system, idle, nice. host_statistics returns their count separately.
// A regression, including a native counter wrap, is refused by resourceBusy.
const resourceMachCPULoadInfo = 3

type resourceMachStatus int32
type resourceMachCPUCounters [4]uint32

func (s resourceMachStatus) Error() string {
	return "Mach kernel status " + strconv.FormatInt(int64(s), 10)
}

func resourceMachResult(status uintptr, native syscall.Errno) error {
	if native != 0 {
		return errors.Join(core.ErrHostFactsObservation, native)
	}
	if status != 0 {
		return errors.Join(core.ErrHostFactsObservation, resourceMachStatus(status))
	}
	return nil
}

func resourceMachBusy(before, after resourceMachCPUCounters, cpus LogicalCPUCount) (uint64, error) {
	var first, last resourceCPUCounters
	for i := range before {
		if after[i] < before[i] {
			return 0, core.ErrHostFactsObservation
		}
		first.total += uint64(before[i])
		last.total += uint64(after[i])
	}
	first.idle, last.idle = uint64(before[2]), uint64(after[2])
	return resourceBusy(first, last, cpus)
}

func readResourceCPU(ctx context.Context) (resourceMachCPUCounters, error) {
	return readResourceMachCPU(ctx, resourceMachCPULoadInfo)
}

func readResourceMachCPU(ctx context.Context, flavor uintptr) (out resourceMachCPUCounters, resultErr error) {
	if err := contextstate.Validate(ctx); err != nil {
		return out, err
	}
	// The SDK macro borrows this cached task right. Calling the similarly
	// named trap instead would acquire another send right that needs release.
	task := uintptr(resourceMachTaskSelf())
	if task == 0 || task >= resourceMachDeadPort {
		return out, core.ErrHostFactsObservation
	}
	host, _, native := resourceMachCall(resourceMachHostSelfAddr, 0, 0, 0, 0, 0, 0)
	if native != 0 || host == 0 || host >= resourceMachDeadPort {
		return out, errors.Join(core.ErrHostFactsObservation, nonzeroMachErrno(native))
	}
	// mach_host_self supplies a send right. Every read relinquishes its own
	// right before the caller can wait, fail, cancel, or publish a sample.
	defer func() {
		status, _, native := resourceMachCall(resourceMachDeallocateAddr, task, host, 0, 0, 0, 0)
		resultErr = errors.Join(resultErr, resourceMachResult(status, native))
		if resultErr != nil {
			out = resourceMachCPUCounters{}
		}
	}()
	count := uint32(len(out))
	status, _, native := resourceMachCall(resourceMachStatisticsAddr, host, flavor, uintptr(unsafe.Pointer(&out[0])), uintptr(unsafe.Pointer(&count)), 0, 0)
	runtime.KeepAlive(&out)
	runtime.KeepAlive(&count)
	if err := resourceMachResult(status, native); err != nil {
		return out, err
	}
	if count != uint32(len(out)) {
		return out, core.ErrHostFactsObservation
	}
	var total uint64
	for _, tick := range out {
		total += uint64(tick)
	}
	if total == 0 {
		return resourceMachCPUCounters{}, core.ErrHostFactsObservation
	}
	return out, contextstate.Validate(ctx)
}

const resourceMachDeadPort = uintptr(1<<32 - 1) // MACH_PORT_DEAD; zero is MACH_PORT_NULL.

func nonzeroMachErrno(native syscall.Errno) error {
	if native != 0 {
		return native
	}
	return nil
}

// The same Go runtime/libSystem bridge is used by golang.org/x/sys/unix.
// These imports do not enable cgo or require a C compiler. syscall.syscall6
// supplies Go's native-call stack and scheduler boundary.
var resourceMachHostSelfAddr uintptr
var resourceMachStatisticsAddr, resourceMachDeallocateAddr uintptr

func resourceMachTaskSelf() uint32

//go:linkname resourceMachCall syscall.syscall6
//go:uintptrescapes
func resourceMachCall(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:cgo_import_dynamic resource_mach_host_self mach_host_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_mach_task_self_ mach_task_self_ "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_host_statistics host_statistics "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_mach_port_deallocate mach_port_deallocate "/usr/lib/libSystem.B.dylib"
