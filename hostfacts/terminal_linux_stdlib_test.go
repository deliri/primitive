//go:build linux

package hostfacts

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/sys/unix"
)

func TestLinuxStandardSyscallPreservesNativeTerminalWindowFields(t *testing.T) {
	t.Parallel()
	for _, rows := range []uint16{0, 1, 32767, 65535} {
		for _, columns := range []uint16{0, 1, 2, 15, 16, 79, 80, 81, 120, 121, 122, 255, 256, 4096, 32768, 65535} {
			t.Run(fmt.Sprintf("rows_%d_columns_%d", rows, columns), func(t *testing.T) {
				t.Parallel()
				slave := openPseudoTerminalSlave(t)
				window := unix.Winsize{Row: rows, Col: columns, Xpixel: ^rows, Ypixel: ^columns}
				if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &window); err != nil {
					t.Fatal(err)
				}
				got, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
				if err != nil {
					t.Fatal(err)
				}
				attachment, err := got.Attachment()
				wantAttachment, wantErr := TerminalAttachmentTerminal, error(nil)
				if columns == 0 {
					wantAttachment, wantErr = TerminalAttachmentTerminalWithoutGeometry, core.ErrHostFactsContract
				}
				if err != nil || attachment != wantAttachment {
					t.Fatalf("attachment=%v/%v, want %v", attachment, err, wantAttachment)
				}
				observed, err := got.Columns()
				if !errors.Is(err, wantErr) || uint16(observed) != columns {
					t.Fatalf("columns=%d/%v, want exact %d/%v", observed, err, columns, wantErr)
				}
				native, err := unix.IoctlGetWinsize(int(slave.Fd()), unix.TIOCGWINSZ)
				if err != nil || native == nil || *native != window {
					t.Fatalf("native window=%+v/%v, want every untouched kernel field %+v", native, err, window)
				}
			})
		}
	}
}

func FuzzLinuxStandardTerminalSyscallMatchesKernelGeometry(f *testing.F) {
	for _, columns := range []uint16{0, 1, 16, 80, 121, 256, 32768, 65535} {
		for _, rows := range []uint16{0, 1, 32767, 65535} {
			f.Add(rows, columns)
		}
	}
	f.Fuzz(func(t *testing.T, rows, columns uint16) {
		slave := openPseudoTerminalSlave(t)
		window := unix.Winsize{Row: rows, Col: columns, Xpixel: ^rows, Ypixel: ^columns}
		if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &window); err != nil {
			t.Fatal(err)
		}
		got, err := ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
		if err != nil {
			t.Fatal(err)
		}
		attachment, err := got.Attachment()
		wantAttachment, wantErr := TerminalAttachmentTerminal, error(nil)
		if columns == 0 {
			wantAttachment, wantErr = TerminalAttachmentTerminalWithoutGeometry, core.ErrHostFactsContract
		}
		if err != nil || attachment != wantAttachment {
			t.Fatalf("attachment=%v/%v, want %v", attachment, err, wantAttachment)
		}
		observed, err := got.Columns()
		if !errors.Is(err, wantErr) || uint16(observed) != columns {
			t.Fatalf("columns=%d/%v, want native %d/%v", observed, err, columns, wantErr)
		}
		native, err := unix.IoctlGetWinsize(int(slave.Fd()), unix.TIOCGWINSZ)
		if err != nil || native == nil || *native != window {
			t.Fatalf("native window=%+v/%v, want untouched %+v", native, err, window)
		}
	})
}

func BenchmarkLinuxStandardTerminalSyscallObservation(b *testing.B) {
	slave := openPseudoTerminalSlave(b)
	const columns = 121
	window := unix.Winsize{Row: 40, Col: columns, Xpixel: 999, Ypixel: 1234}
	if err := unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &window); err != nil {
		b.Fatal(err)
	}
	var got TerminalGeometry
	var observed uint64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		got, err = ObserveTerminalGeometry(TerminalGeometryRequest{File: slave})
		if err != nil {
			b.Fatal(err)
		}
		observed++
	}
	b.StopTimer()
	width, err := got.Columns()
	if err != nil || width != columns || observed != uint64(b.N) {
		b.Fatalf("observations=%d final columns=%d/%v, want exact %d native observations of %d", observed, width, err, b.N, columns)
	}
	native, err := unix.IoctlGetWinsize(int(slave.Fd()), unix.TIOCGWINSZ)
	if err != nil || native == nil || *native != window {
		b.Fatalf("native window=%+v/%v, want untouched %+v", native, err, window)
	}
}
