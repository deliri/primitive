//go:build darwin

#include "textflag.h"

// libSystem trampolines follow the x/sys/unix native-call pattern.
TEXT resource_host_self_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_mach_host_self(SB)
GLOBL ·resourceMachHostSelfAddr(SB),RODATA,$8
DATA ·resourceMachHostSelfAddr(SB)/8,$resource_host_self_trampoline<>(SB)

TEXT resource_processor_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_host_processor_info(SB)
GLOBL ·resourceMachProcessorInfoAddr(SB),RODATA,$8
DATA ·resourceMachProcessorInfoAddr(SB)/8,$resource_processor_trampoline<>(SB)

TEXT resource_deallocate_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_mach_port_deallocate(SB)
GLOBL ·resourceMachDeallocateAddr(SB),RODATA,$8
DATA ·resourceMachDeallocateAddr(SB)/8,$resource_deallocate_trampoline<>(SB)

TEXT resource_vm_deallocate_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_mach_vm_deallocate(SB)
GLOBL ·resourceMachVMDeallocateAddr(SB),RODATA,$8
DATA ·resourceMachVMDeallocateAddr(SB)/8,$resource_vm_deallocate_trampoline<>(SB)

// Borrow the cached task port exactly as Go's Darwin runtime does.
TEXT ·resourceMachTaskSelf(SB),NOSPLIT,$0-4
	MOVD $resource_mach_task_self_(SB), R0
	MOVWU (R0), R0
	MOVW R0, ret+0(FP)
	RET
