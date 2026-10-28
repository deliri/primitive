//go:build darwin

// Package machrefs is an independent native reference-count oracle for the
// owning Hostfacts tests. It is never imported by production code.
package machrefs

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type Reference struct{ task, host uintptr }

func Hold() (Reference, error) {
	task := uintptr(taskSelf())
	host, _, native := call(hostAddr, 0, 0, 0, 0, 0, 0)
	if native != 0 || task == 0 || host == 0 || host >= 1<<32-1 {
		return Reference{}, fmt.Errorf("native host reference: task=%d host=%d errno=%v", task, host, native)
	}
	return Reference{task, host}, nil
}

func (r Reference) Count() (uint32, error) {
	var count uint32
	// MACH_PORT_RIGHT_SEND is zero. The caller keeps one reference alive so
	// the kernel name remains valid throughout both observations.
	status, _, native := call(refsAddr, r.task, r.host, 0, uintptr(unsafe.Pointer(&count)), 0, 0)
	runtime.KeepAlive(&count)
	return count, checked(status, native)
}

func (r Reference) Close() error {
	status, _, native := call(deallocateAddr, r.task, r.host, 0, 0, 0, 0)
	return checked(status, native)
}

func checked(status uintptr, native syscall.Errno) error {
	if native != 0 {
		return fmt.Errorf("native reference observation: %w", native)
	}
	if status != 0 {
		return fmt.Errorf("native reference observation status %d", int32(status))
	}
	return nil
}

var hostAddr, refsAddr, deallocateAddr uintptr

func taskSelf() uint32

//go:linkname call syscall.syscall6
//go:uintptrescapes
func call(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

//go:cgo_import_dynamic refs_mach_host_self mach_host_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic refs_mach_task_self_ mach_task_self_ "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic refs_mach_port_get_refs mach_port_get_refs "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic refs_mach_port_deallocate mach_port_deallocate "/usr/lib/libSystem.B.dylib"
