//go:build darwin

#include "textflag.h"

TEXT resource_control_statistics_trampoline<>(SB),NOSPLIT,$0-0
	JMP resource_control_host_statistics(SB)
GLOBL ·resourceMachCachedStatisticsAddr(SB),RODATA,$8
DATA ·resourceMachCachedStatisticsAddr(SB)/8,$resource_control_statistics_trampoline<>(SB)
