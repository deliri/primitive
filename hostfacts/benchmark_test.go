package hostfacts

import (
	"bytes"
	"context"
	"math"
	"testing"
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
	want := GoOOMBannerEvidence{examined: mustByteLength(b, GoOOMMaximumEvidenceBytes), state: GoOOMBannerPresent}
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
