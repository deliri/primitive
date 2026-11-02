//go:build darwin

#include "textflag.h"

// libSystem trampolines follow the x/sys/unix native-call pattern.
TEXT resource_host_self_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_mach_host_self(SB)
GLOBL ·resourceMachHostSelfAddr(SB),RODATA,$8
DATA ·resourceMachHostSelfAddr(SB)/8,$resource_host_self_trampoline<>(SB)

TEXT resource_statistics_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_host_statistics(SB)
GLOBL ·resourceMachStatisticsAddr(SB),RODATA,$8
DATA ·resourceMachStatisticsAddr(SB)/8,$resource_statistics_trampoline<>(SB)

TEXT resource_deallocate_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_mach_port_deallocate(SB)
GLOBL ·resourceMachDeallocateAddr(SB),RODATA,$8
DATA ·resourceMachDeallocateAddr(SB)/8,$resource_deallocate_trampoline<>(SB)

// Borrow the cached task port exactly as Go's Darwin runtime does.
TEXT ·resourceMachTaskSelf(SB),NOSPLIT,$0-4
	MOVQ $resource_mach_task_self_(SB), AX
	MOVL (AX), AX
	MOVL AX, ret+0(FP)
	RET
