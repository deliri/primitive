//go:build linux

package hostfacts

import (
	"fmt"
	"math"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinuxDeviceProjectionPreservesEveryKernelIdentityBit(t *testing.T) {
	t.Parallel()
	// Each bit has a distinct destination across four disjoint ABI fields.
	// x/sys remains an independent test oracle, outside the production leaf.
	for bit := range 64 {
		t.Run(fmt.Sprint(bit), func(t *testing.T) {
			t.Parallel()
			device := uint64(1) << bit
			major, minor := linuxDeviceMajor(device), linuxDeviceMinor(device)
			if major != unix.Major(device) || minor != unix.Minor(device) || unix.Mkdev(major, minor) != device {
				t.Fatalf("device bit %d projected to %x:%x, want %x:%x and exact native reconstruction", bit, major, minor, unix.Major(device), unix.Minor(device))
			}
		})
	}
}

func TestLinuxHeldDirectoryDeviceMatchesNativeKernelIdentity(t *testing.T) {
	t.Parallel()
	path := mustAbsolutePathForHostfactsTest(t, t.TempDir())
	root, err := openRoot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.close(); err != nil {
			t.Fatal(err)
		}
	})
	file, err := root.directory.File()
	if err != nil {
		t.Fatal(err)
	}
	var native syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &native); err != nil {
		t.Fatal(err)
	}
	if root.dev != uint64(native.Dev) || linuxDeviceMajor(root.dev) != unix.Major(uint64(native.Dev)) || linuxDeviceMinor(root.dev) != unix.Minor(uint64(native.Dev)) {
		t.Fatalf("held directory device %x lost actual kernel identity %x", root.dev, native.Dev)
	}
}

func FuzzLinuxDeviceProjectionMatchesNativeEncoding(f *testing.F) {
	for bit := range 64 {
		f.Add(uint64(1) << bit)
	}
	for _, device := range []uint64{0, math.MaxUint64, 0xaaaaaaaaaaaaaaaa, 0x5555555555555555} {
		f.Add(device)
	}
	f.Fuzz(func(t *testing.T, device uint64) {
		major, minor := linuxDeviceMajor(device), linuxDeviceMinor(device)
		if major != unix.Major(device) || minor != unix.Minor(device) || unix.Mkdev(major, minor) != device {
			t.Fatalf("device %x projected to %x:%x, want %x:%x and native reconstruction", device, major, minor, unix.Major(device), unix.Minor(device))
		}
	})
}

func BenchmarkLinuxDeviceProjection(b *testing.B) {
	var checksum, want uint64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		device := uint64(i)
		checksum += uint64(linuxDeviceMajor(device)) + uint64(linuxDeviceMinor(device))
	}
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		want += uint64(unix.Major(uint64(i))) + uint64(unix.Minor(uint64(i)))
	}
	if checksum != want {
		b.Fatalf("observed checksum = %x, want native %x", checksum, want)
	}
}
