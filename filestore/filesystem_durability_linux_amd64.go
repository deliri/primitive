package filestore

// Linux arch/x86/entry/syscalls/syscall_64.tbl assigns syncfs number 306.
// The frozen Go syscall package does not publish SYS_SYNCFS on amd64.
const filesystemSyncTrap = 306
