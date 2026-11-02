//go:build darwin

#include "textflag.h"

TEXT refs_host_trampoline<>(SB),NOSPLIT,$0-0
	JMP refs_mach_host_self(SB)
GLOBL ·hostAddr(SB),RODATA,$8
DATA ·hostAddr(SB)/8,$refs_host_trampoline<>(SB)

TEXT refs_count_trampoline<>(SB),NOSPLIT,$0-0
	JMP refs_mach_port_get_refs(SB)
GLOBL ·refsAddr(SB),RODATA,$8
DATA ·refsAddr(SB)/8,$refs_count_trampoline<>(SB)

TEXT refs_deallocate_trampoline<>(SB),NOSPLIT,$0-0
	JMP refs_mach_port_deallocate(SB)
GLOBL ·deallocateAddr(SB),RODATA,$8
DATA ·deallocateAddr(SB)/8,$refs_deallocate_trampoline<>(SB)

TEXT ·taskSelf(SB),NOSPLIT,$0-4
	MOVD $refs_mach_task_self_(SB), R0
	MOVWU (R0), R0
	MOVW R0, ret+0(FP)
	RET
