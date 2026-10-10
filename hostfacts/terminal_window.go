package hostfacts

// terminalWindow is the eight-byte winsize ABI used by the Linux leaf. The
// kernel fills one borrowed record while SyscallConn.Control holds its file.
type terminalWindow struct {
	Rows, Columns, XPixels, YPixels uint16
}
