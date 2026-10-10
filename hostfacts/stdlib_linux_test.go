//go:build linux

package hostfacts

import (
	"errors"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"math"
	"math/big"
	"syscall"
	"testing"
)

func TestPhysicalMemoryScalingUsesIndependentUnboundedIntegerOracle(t *testing.T) {
	t.Parallel()
	totals := []uint64{0, 1, 2, math.MaxUint64, math.MaxUint64 / 2, math.MaxUint64/2 + 1, math.MaxUint64 / 3, math.MaxUint64/3 + 1, math.MaxUint64/4096 + 1}
	units := []uint64{0, 1, 2, 3, 4096, math.MaxUint32, math.MaxUint64}
	for _, total := range totals {
		for _, unit := range units {
			t.Run(fmt.Sprint(total)+"/"+fmt.Sprint(unit), func(t *testing.T) {
				t.Parallel()
				product := new(big.Int).Mul(new(big.Int).SetUint64(total), new(big.Int).SetUint64(unit))
				got, err := physicalMemoryBytes(total, unit)
				if unit == 0 || !product.IsUint64() {
					if got != 0 || !errors.Is(err, core.ErrHostFactsObservation) {
						t.Fatalf("scale = %d/%v, want typed numerical refusal", got, err)
					}
					return
				}
				if err != nil || got != product.Uint64() {
					t.Fatalf("scale = %d/%v, want independent product %v", got, err, product)
				}
			})
		}
	}
}

func TestLinuxPhysicalMemoryMatchesGoStandardSysinfo(t *testing.T) {
	t.Parallel()
	var native syscall.Sysinfo_t
	if err := syscall.Sysinfo(&native); err != nil {
		t.Fatalf("native Sysinfo = %v", err)
	}
	want := new(big.Int).Mul(new(big.Int).SetUint64(uint64(native.Totalram)), new(big.Int).SetUint64(uint64(native.Unit)))
	if !want.IsUint64() || want.Sign() <= 0 {
		t.Fatalf("native kernel total is unusable: %v", want)
	}
	got, err := ObservePhysicalMemory(t.Context())
	if err != nil || got.TotalBytes().Uint64() != want.Uint64() {
		t.Fatalf("physical memory = %v/%v, want actual kernel total %v", got, err, want)
	}
}

func TestLinuxHeldDirectoryCapacityMatchesGoStandardFstatfs(t *testing.T) {
	t.Parallel()
	path := mustAbsolutePathForHostfactsTest(t, t.TempDir())
	root, err := openRoot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.close(); err != nil {
			t.Fatalf("close held directory = %v", err)
		}
	})
	file, err := root.directory.File()
	if err != nil {
		t.Fatal(err)
	}
	var native syscall.Statfs_t
	if err := syscall.Fstatfs(int(file.Fd()), &native); err != nil {
		t.Fatalf("native Fstatfs = %v", err)
	}
	unit := native.Frsize
	if unit <= 0 {
		unit = native.Bsize
	}
	if unit <= 0 {
		t.Fatalf("native filesystem unit = %d", unit)
	}
	total := new(big.Int).Mul(new(big.Int).SetUint64(native.Blocks), new(big.Int).SetUint64(uint64(unit)))
	got, err := root.diskCapacity()
	if err != nil {
		t.Fatal(err)
	}
	gotTotal, totalErr := got.TotalBytes().Uint64()
	if totalErr != nil || !total.IsUint64() || gotTotal != total.Uint64() {
		t.Fatalf("capacity = %d/%v, want kernel total %v", gotTotal, totalErr, total)
	}
	// Available blocks may change during independent observations on a busy host.
	// Their unit and their containment in the same filesystem remain native invariants.
	if got.AvailableBytes().Uint64()%uint64(unit) != 0 || got.AvailableBytes().Uint64() > gotTotal {
		t.Fatalf("available capacity lost the native filesystem unit: %+v", got)
	}
}

func FuzzPhysicalMemoryKernelScalingMatchesIndependentIntegerOracle(f *testing.F) {
	for _, total := range []uint64{0, 1, math.MaxUint64 / 3, math.MaxUint64/3 + 1, math.MaxUint64} {
		for _, unit := range []uint64{0, 1, 3, 4096, math.MaxUint64} {
			f.Add(total, unit)
		}
	}
	f.Fuzz(func(t *testing.T, total, unit uint64) {
		product := new(big.Int).Mul(new(big.Int).SetUint64(total), new(big.Int).SetUint64(unit))
		got, err := physicalMemoryBytes(total, unit)
		if unit == 0 || !product.IsUint64() {
			if got != 0 || !errors.Is(err, core.ErrHostFactsObservation) {
				t.Fatalf("scale = %d/%v, want typed numerical refusal", got, err)
			}
			return
		}
		if err != nil || got != product.Uint64() {
			t.Fatalf("scale = %d/%v, want independent product %v", got, err, product)
		}
	})
}

func BenchmarkGoStandardLinuxPhysicalMemoryObservation(b *testing.B) {
	var native syscall.Sysinfo_t
	if err := syscall.Sysinfo(&native); err != nil {
		b.Fatal(err)
	}
	product := new(big.Int).Mul(new(big.Int).SetUint64(uint64(native.Totalram)), new(big.Int).SetUint64(uint64(native.Unit)))
	if !product.IsUint64() {
		b.Fatal("native kernel total is not representable")
	}
	want := product.Uint64()
	var got PhysicalMemory
	var observed uint64
	ctx := b.Context()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		got, err = ObservePhysicalMemory(ctx)
		if err != nil {
			b.Fatal(err)
		}
		observed++
	}
	b.StopTimer()
	if observed != uint64(b.N) || got.TotalBytes().Uint64() != want {
		b.Fatalf("observable kernel observations = %d, final total = %v, want %d", observed, got, want)
	}
}
