# Calling-process output half-close

Disproved assumption: closing fd1 signals EOF when fd0 aliases the same socket.
The native rsync hang retains fd0 after fd1 closed. Socket write shutdown affects
the shared socket while keeping the read direction usable. Ordinary pipe/file
close remains the native EOF operation. No arbitrary descriptor API is added.

Authority read before implementation:
- Go1.27.3 net/file.go, file_unix.go, unixsock.go and fd_posix.go: FileConn
  duplicates and makes the shared description nonblocking; CloseWrite delegates
  to shutdown(SHUT_WR). Do not mutate aliases' blocking policy.
- Go1.27.3 os/file.go and rawconn.go: SyscallConn.Control pins the existing file.
- https://www.man7.org/linux/man-pages/man2/shutdown.2.html
- https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/shutdown.2.html

StandardOutput returns a sealed OutputStream. It reuses the existing scoped
pipeErrorDestination authority for error-returning writes (fd1 EPIPE must not
terminate the caller), and owns that writer plus original stdout until Close.
Close pins stdout, shuts down only socket writes, and closes both owned handles
on every native outcome. ENOTSOCK alone means ordinary pipe/file close; other
native causes stay reachable beneath Core.ErrProcessStream. Zero is refused.

Native aliased socket row runs an owned child with one socket endpoint as both
stdin and stdout, and fd3/fd4 as its inherited far streams. The peer waits for
EOF before sending the upward reply. Exact data, unchanged shared flags and a
clean joined child prove half-close plus continued reading. A cancellation row
kills/reaps the blocked native child. Other rows prove pipe/file EOF, nil and
closed constructor refusal, single owned close, repeat-close/native write
causes, unconnected-socket shutdown failure plus cleanup, and EPIPE conservation.
Fuzz writes arbitrary bytes through the public output in the same isolated
native pipe child and requires byte equality, closed ownership and joined exit.
mutants.json names compiling behavioral mutations; build failure is not proof.

Catalog authority remains Primitive capabilities: Process owns the returned
calling-process capability; raw syscall.Shutdown remains a native transport
effect unavailable to application code. No competing operation list is created.
