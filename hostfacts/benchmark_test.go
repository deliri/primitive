package hostfacts

import (
	"bytes"
	"context"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkGoOOMBannerChunkBoundary(b *testing.B) {
	b.ReportAllocs()
	data := bytes.Repeat([]byte{'x'}, goOOMBufferBytes*2)
	copy(data[goOOMBufferBytes-1:], GoOOMPrefixedBanner)
	length := mustByteLength(b, uint64(len(data)))
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		reader := bytes.NewReader(data)
		got, err := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{Source: reader, Length: length})
		if err != nil || got.State() != GoOOMBannerPresent || got.BytesExamined() != length || reader.Len() != 0 {
			b.Fatalf("boundary banner = %+v/%v, unread %d, want exact presence and exhausted extent", got, err, reader.Len())
		}
	}
}

func BenchmarkMountInfoMatching(b *testing.B) {
	b.ReportAllocs()
	line := []byte("30 23 0:27 /tenant /sys/fs/cgroup rw - " + cgroupV2Filesystem + " cgroup rw")
	for b.Loop() {
		got, matched, err := parseMountInfoLine(line, WorkloadMemoryLimitSourceCgroupV2)
		if err != nil || !matched || got.root != "/tenant" || got.mountPoint.String() != "/sys/fs/cgroup" || got.source != WorkloadMemoryLimitSourceCgroupV2 {
			b.Fatalf("mount = %+v/%t/%v, want exact matched kernel facts", got, matched, err)
		}
	}
}

func BenchmarkCgroupMembershipMatching(b *testing.B) {
	b.ReportAllocs()
	line := []byte(cgroupV2HierarchyToken + "::/tenant/job")
	for b.Loop() {
		got, err := parseCgroupMembershipLine(line)
		if err != nil || got.path != "/tenant/job" || got.source != WorkloadMemoryLimitSourceCgroupV2 {
			b.Fatalf("membership = %+v/%v, want exact v2 membership", got, err)
		}
	}
}

func BenchmarkGoOOMStateJSON(b *testing.B) {
	b.ReportAllocs()
	data, err := GoOOMBannerPresent.MarshalJSON()
	if err != nil {
		b.Fatalf("state fixture error = %v, want nil", err)
	}
	for b.Loop() {
		var got GoOOMBannerState
		if err := got.UnmarshalJSON(data); err != nil || got != GoOOMBannerPresent {
			b.Fatalf("state = %v/%v, want present", got, err)
		}
	}
}

func BenchmarkGoOOMEvidenceJSON(b *testing.B) {
	b.ReportAllocs()
	want := GoOOMBannerEvidence{examined: mustByteLength(b, goOOMFixtureBytes), state: GoOOMBannerPresent}
	data, err := want.MarshalJSON()
	if err != nil {
		b.Fatalf("evidence fixture error = %v, want nil", err)
	}
	for b.Loop() {
		var got GoOOMBannerEvidence
		if err := got.UnmarshalJSON(data); err != nil || got != want {
			b.Fatalf("evidence = %+v/%v, want %+v", got, err, want)
		}
	}
}

func BenchmarkMemoryTriggerMaximumBatch(b *testing.B) {
	b.ReportAllocs()
	percent, err := NewPercent(100)
	if err != nil {
		b.Fatalf("percentage fixture error = %v, want nil", err)
	}
	snapshot, err := newGoMemorySnapshot(math.MaxInt64, 0, math.MaxInt64)
	if err != nil {
		b.Fatalf("snapshot fixture error = %v, want nil", err)
	}
	policy := GoMemoryPressurePolicy{TriggerPercent: percent}
	for b.Loop() {
		for range 64 {
			got, err := policy.TriggerBytes(snapshot.LimitBytes())
			if err != nil || got != snapshot.LimitBytes() {
				b.Fatalf("trigger = %v/%v, want exact maximum", got, err)
			}
		}
	}

	b.ReportMetric(64, "values/op")
}

// benchmarkCPUStat is a complete cgroup v2 cpu.stat file as the kernel
// publishes it for a busy systemd scope.
const benchmarkCPUStat = "usage_usec 81234567890\nuser_usec 61234567890\nsystem_usec 20000000000\n" +
	"core_sched.force_idle_usec 0\nnr_periods 0\nnr_throttled 0\nthrottled_usec 0\n" +
	"nr_bursts 0\nburst_usec 0\n"

// BenchmarkScopeCPUStatScan measures one scope sample after the file is
// opened: the bounded line scan of cpu.stat folded to usage_usec. Resource
// sampling performs this twice per requested scope per window.
func BenchmarkScopeCPUStatScan(b *testing.B) {
	data := []byte(benchmarkCPUStat)
	if bytes.Count(data, []byte{'\n'}) != 9 {
		b.Fatal("cpu.stat fixture is not the declared nine-line file")
	}
	reader := bytes.NewReader(data)
	var fold cpuStatFold
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(data)
		fold = cpuStatFold{}
		if err := (boundedLineScan{reader: reader, maximum: cgroupCPUStatBytes, visit: fold.visit}).run(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
	if !fold.seen || fold.usage != 81234567890 {
		b.Fatalf("fold = %+v, want usage_usec 81234567890", fold)
	}
}

// BenchmarkScopesBusy measures the pure pairing of eight scopes' readings
// into window-relative milli-CPU, including one absent and one replaced scope.
func BenchmarkScopesBusy(b *testing.B) {
	const scopes = 8
	cgroups := make([]scopeCgroup, scopes)
	before := make([]scopeSample, scopes)
	after := make([]scopeSample, scopes)
	for index := range scopes {
		unit, err := core.ParsePathComponent("bench-" + string(rune('a'+index)) + ".scope")
		if err != nil {
			b.Fatal(err)
		}
		cgroups[index] = scopeCgroup{scope: SystemdScopeRequest{Unit: unit}, present: true}
		before[index] = scopeSample{usage: 1_000_000, present: true}
		after[index] = scopeSample{usage: 1_000_000 + uint64(index+1)*250_000, present: true}
	}
	after[scopes-2].present = false
	after[scopes-1].usage = 0
	const window = int64(2_000_000_000)
	var got []ScopeCPU
	b.ReportAllocs()
	for b.Loop() {
		var err error
		got, err = scopesBusy(cgroups, before, after, window)
		if err != nil {
			b.Fatal(err)
		}
	}
	if len(got) != scopes || got[0].BusyCPUMilli != 125 || !got[0].Present || got[scopes-2].Present || got[scopes-1].Present {
		b.Fatalf("scopes = %+v, want exact window-relative milli-CPU with absent tails", got)
	}
}
