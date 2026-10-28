//go:build darwin

package hostfacts

import (
	"context"
	"errors"
	"math"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// host_processor_info supplies four uint32 ticks per processor without the
// third-party host_statistics cache. Keep each processor until comparison so
// a wrapped counter cannot hide behind another processor's advancing ticks.
const resourceMachProcessorCPULoadInfo = 2
const resourceMachCPUStates = 4

type resourceMachStatus int32
type resourceMachCPUState [resourceMachCPUStates]uint32
type resourceMachCPUCounters []resourceMachCPUState

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
	if cpus.Validate() != nil || len(before) != cpus.Int() || len(after) != len(before) {
		return 0, core.ErrHostFactsObservation
	}
	var first, last resourceCPUCounters
	for cpu := range before {
		for state, tick := range before[cpu] {
			next := after[cpu][state]
			if next < tick || math.MaxUint64-first.total < uint64(tick) || math.MaxUint64-last.total < uint64(next) {
				return 0, core.ErrHostFactsObservation
			}
			first.total += uint64(tick)
			last.total += uint64(next)
			if state == 2 {
				first.idle += uint64(tick)
				last.idle += uint64(next)
			}
		}
	}
	return resourceBusy(first, last, cpus)
}

// resourceMachCPUBuffer owns one returned OOL VM region and one host send
// right. The cached task port is borrowed. Neither resource survives a read.
type resourceMachCPUBuffer struct {
	task, host        uintptr
	data              unsafe.Pointer
	processors, count uint32
}

func (b *resourceMachCPUBuffer) close() error {
	var vmErr, hostErr error
	if b.data != nil {
		// count is the returned number of integer_t values, not processors.
		// Widen before multiplying; uint32*4 fits both supported 64-bit ABIs.
		extent := uint64(b.count) * uint64(unsafe.Sizeof(uint32(0)))
		status, _, native := resourceMachCall(resourceMachVMDeallocateAddr, b.task, uintptr(b.data), uintptr(extent), 0, 0, 0)
		vmErr = resourceMachResult(status, native)
		if vmErr == nil {
			b.data = nil
		}
	}
	if b.host != 0 {
		status, _, native := resourceMachCall(resourceMachDeallocateAddr, b.task, b.host, 0, 0, 0, 0)
		hostErr = resourceMachResult(status, native)
		if hostErr == nil {
			b.host = 0
		}
	}
	return errors.Join(vmErr, hostErr)
}

func (b resourceMachCPUBuffer) snapshot() (resourceMachCPUCounters, error) {
	if b.data == nil || uintptr(b.data)%unsafe.Alignof(resourceMachCPUState{}) != 0 || b.processors == 0 || uint64(b.processors) != uint64(runtime.NumCPU()) || uint64(b.count) != uint64(b.processors)*resourceMachCPUStates {
		return nil, core.ErrHostFactsObservation
	}
	// The validated native extent is exactly this many fixed processor rows.
	rows := unsafe.Slice((*resourceMachCPUState)(b.data), int(b.processors))
	out := make(resourceMachCPUCounters, len(rows))
	copy(out, rows)
	haveTicks := false
	for _, row := range out {
		for _, tick := range row {
			haveTicks = haveTicks || tick != 0
		}
	}
	if !haveTicks {
		return nil, core.ErrHostFactsObservation
	}
	return out, nil
}

func openResourceMachCPU(ctx context.Context, flavor uintptr) (out resourceMachCPUBuffer, resultErr error) {
	if err := contextstate.Validate(ctx); err != nil {
		return out, err
	}
	out.task = uintptr(resourceMachTaskSelf())
	if out.task == 0 || out.task >= resourceMachDeadPort {
		return out, core.ErrHostFactsObservation
	}
	host, _, native := resourceMachCall(resourceMachHostSelfAddr, 0, 0, 0, 0, 0, 0)
	if host != 0 && host < resourceMachDeadPort {
		out.host = host
	}
	// Once native resources arrive, cleanup precedes every validation/cancel
	// return. It always attempts VM and port release independently.
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, out.close())
			out = resourceMachCPUBuffer{}
		}
	}()
	if native != 0 || out.host == 0 {
		return out, errors.Join(core.ErrHostFactsObservation, nonzeroMachErrno(native))
	}
	status, _, native := resourceMachCall(resourceMachProcessorInfoAddr, out.host, flavor, uintptr(unsafe.Pointer(&out.processors)), uintptr(unsafe.Pointer(&out.data)), uintptr(unsafe.Pointer(&out.count)), 0)
	runtime.KeepAlive(&out)
	if err := resourceMachResult(status, native); err != nil {
		return out, err
	}
	if out.data == nil || out.processors == 0 || uint64(out.count) != uint64(out.processors)*resourceMachCPUStates {
		return out, core.ErrHostFactsObservation
	}
	return out, contextstate.Validate(ctx)
}

func readResourceCPU(ctx context.Context) (resourceMachCPUCounters, error) {
	return readResourceMachCPU(ctx, resourceMachProcessorCPULoadInfo)
}

func readResourceMachCPU(ctx context.Context, flavor uintptr) (out resourceMachCPUCounters, resultErr error) {
	buffer, err := openResourceMachCPU(ctx, flavor)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, buffer.close())
		if resultErr != nil {
			out = nil
		}
	}()
	out, resultErr = readCachedResourceCPU(buffer)
	if resultErr == nil {
		resultErr = contextstate.Validate(ctx)
	}
	return out, resultErr
}

const resourceMachDeadPort = uintptr(1<<32 - 1)

func nonzeroMachErrno(native syscall.Errno) error {
	if native != 0 {
		return native
	}
	return nil
}

// Existing Go runtime/libSystem bridge; no cgo or C compiler is required.
var resourceMachHostSelfAddr uintptr
var resourceMachProcessorInfoAddr, resourceMachDeallocateAddr, resourceMachVMDeallocateAddr uintptr

func resourceMachTaskSelf() uint32

//go:linkname resourceMachCall syscall.syscall6
//go:uintptrescapes
func resourceMachCall(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:cgo_import_dynamic resource_mach_host_self mach_host_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_mach_task_self_ mach_task_self_ "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_host_processor_info host_processor_info "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_mach_port_deallocate mach_port_deallocate "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic resource_mach_vm_deallocate mach_vm_deallocate "/usr/lib/libSystem.B.dylib"
