# Named pipes and blocking duplicates

`CreateNamedPipe` accepts `NamedPipeRequest{Path core.AbsolutePath, Mode fs.FileMode}`
and calls native `mkfifo` on Darwin and Linux. The name is exclusive and native
umask applies, including to legitimate mode `0000`. This absolute-path operation
does not promise `os.Root` confinement, reserve the parent against rename, or
acknowledge durable storage. The caller owns the parent and namespace.

`OpenNamedPipeReadWrite(ctx, Location)` opens through the supplied `os.Root` with
`O_RDWR`, without creation or truncation, then checks the held file's FIFO kind.
It returns a caller-owned ordinary Go file and reads no content. Existing rooted
symlink confinement remains Go's responsibility. The caller coordinates namespace
replacement: a racing different entry can be opened before the kind refusal.
FIFO `O_RDWR` is native Darwin/Linux behavior; POSIX leaves it undefined. The
context is admitted before synchronous native entry and cannot interrupt later
blocking I/O.

`DuplicateBlockingFile(ctx, FileHandleRequest)` deliberately performs Go's `Fd`
blocking transition before duplicating the descriptor held by `SyscallConn.Control`.
The earlier numeric `Fd` result is never used for duplication. Source and duplicate
share native open-description flags and file offset. This changes the borrowed
source's deadline and blocking behavior. The caller coordinates concurrent use;
the source is never closed, and closing either Go file leaves the other owned
descriptor usable. No buffering, task, retry, or cancellation-after-entry promise
is added. Native `Dup` does not copy the source's `FD_CLOEXEC`; this operation does
not add close-on-exec to the duplicate. Other platforms return `errors.ErrUnsupported` through Filestore's
existing error categories. Native syscall and cancellation causes remain visible
through `errors.Is` and `errors.As`.

Owning native references: [Go File.Fd and SyscallConn](https://pkg.go.dev/os#File.Fd),
[Go Root.OpenFile](https://pkg.go.dev/os#Root.OpenFile),
[Linux mkfifo](https://man7.org/linux/man-pages/man3/mkfifo.3.html),
[Linux FIFO O_RDWR](https://man7.org/linux/man-pages/man7/fifo.7.html),
and [BSD mkfifo](https://man.freebsd.org/cgi/man.cgi?query=mkfifo&sektion=2).
The pinned native Go source is the implementation reference; portable guarantees
are limited to the explicitly documented platforms and behaviors.
