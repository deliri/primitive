package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCgroupMembershipParserHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		line       string
		wantPath   string
		wantSource WorkloadMemoryLimitSource
		wantErr    bool
	}{
		{name: "v2 root membership", line: "0::/", wantPath: "/", wantSource: WorkloadMemoryLimitSourceCgroupV2},
		{name: "v2 nested membership", line: "0::/system.slice/app.service", wantPath: "/system.slice/app.service", wantSource: WorkloadMemoryLimitSourceCgroupV2},
		{name: "v1 memory-only membership", line: "5:memory:/job", wantPath: "/job", wantSource: WorkloadMemoryLimitSourceCgroupV1},
		{name: "v1 memory first controller", line: "5:memory,cpu:/job", wantPath: "/job", wantSource: WorkloadMemoryLimitSourceCgroupV1},
		{name: "v1 memory last controller", line: "5:cpu,memory:/job", wantPath: "/job", wantSource: WorkloadMemoryLimitSourceCgroupV1},
		{name: "irrelevant v1 controller is neutral", line: "2:cpu:/job"},
		{name: "named hierarchy without memory is neutral", line: "1:name=systemd:/job"},
		{name: "memory substring is neutral", line: "5:notmemory:/job"},
		{name: "empty controller token is neutral outside v2 hierarchy", line: "1::/job"},
		{name: "empty line is malformed", line: "", wantErr: true},
		{name: "missing separators is malformed", line: "0", wantErr: true},
		{name: "one separator is malformed", line: "0:", wantErr: true},
		{name: "empty path is malformed", line: "0::", wantErr: true},
		{name: "relative v2 path is malformed", line: "0::relative", wantErr: true},
		{name: "relative v1 path is malformed", line: "5:memory:relative", wantErr: true},
		{name: "noncanonical parent traversal is malformed", line: "0::/a/../b", wantErr: true},
		{name: "noncanonical doubled separator is malformed", line: "0::/a//b", wantErr: true},
		{name: "deleted membership suffix is malformed", line: "0::/job (deleted)", wantErr: true},
		{name: "NUL membership is malformed", line: "0::/job\x00tail", wantErr: true},
		{name: "extra separators remain a valid cgroup name", line: "0::/job:child", wantPath: "/job:child", wantSource: WorkloadMemoryLimitSourceCgroupV2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseCgroupMembershipLine([]byte(tc.line))
			if tc.wantErr {
				if gotErr == nil {
					t.Fatalf("parseCgroupMembershipLine(%q) error = nil, want refusal", tc.line)
				}
				return
			}
			if gotErr != nil || got.path != tc.wantPath || got.source != tc.wantSource {
				t.Fatalf(
					"parseCgroupMembershipLine(%q) = (%v, %v), want path %q source %v",
					tc.line,
					got,
					gotErr,
					tc.wantPath,
					tc.wantSource,
				)
			}
		})
	}
}

func TestMountInfoParserBindsVersionRootAndMountPoint(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		line      string
		wantRoot  string
		wantMount string
		source    WorkloadMemoryLimitSource
		wantMatch bool
		wantErr   bool
	}{
		{name: "v2 root mount", line: "30 23 0:27 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantMatch: true, wantRoot: "/", wantMount: "/sys/fs/cgroup"},
		{name: "v2 namespaced root", line: "30 23 0:27 /tenant /sys/fs/cgroup rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantMatch: true, wantRoot: "/tenant", wantMount: "/sys/fs/cgroup"},
		{name: "v1 memory super option", line: "31 23 0:28 / /sys/fs/cgroup/memory rw - cgroup cgroup rw,memory", source: WorkloadMemoryLimitSourceCgroupV1, wantMatch: true, wantRoot: "/", wantMount: "/sys/fs/cgroup/memory"},
		{name: "v1 memory among super options", line: "31 23 0:28 /root /sys/fs/cgroup/memory rw - cgroup cgroup rw,cpu,memory,relatime", source: WorkloadMemoryLimitSourceCgroupV1, wantMatch: true, wantRoot: "/root", wantMount: "/sys/fs/cgroup/memory"},
		{name: "v2 is neutral for v1 request", line: "30 23 0:27 / /sys/fs/cgroup rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV1},
		{name: "v1 is neutral for v2 request", line: "31 23 0:28 / /sys/fs/cgroup/memory rw - cgroup cgroup rw,memory", source: WorkloadMemoryLimitSourceCgroupV2},
		{name: "v1 without memory is neutral", line: "31 23 0:28 / /sys/fs/cgroup/cpu rw - cgroup cgroup rw,cpu", source: WorkloadMemoryLimitSourceCgroupV1},
		{name: "escaped space in mount point decodes", line: "30 23 0:27 / /sys/fs/cgroup\\040space rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantMatch: true, wantRoot: "/", wantMount: "/sys/fs/cgroup space"},
		{name: "missing separator is malformed", line: "30 23 0:27 / /sys/fs/cgroup rw cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "truncated post separator fields are malformed", line: "30 23 0:27 / /sys/fs/cgroup rw - cgroup2", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "relative mount point is malformed", line: "30 23 0:27 / relative rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "noncanonical root is malformed", line: "30 23 0:27 /a/../b /sys/fs/cgroup rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "truncated escape is malformed", line: "30 23 0:27 / /sys/fs/cgroup\\04 rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "nonoctal escape is malformed", line: "30 23 0:27 / /sys/fs/cgroup\\xyz rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "noncanonical octal escape is refused", line: "30 23 0:27 / /sys/fs/cgroup\\141 rw - cgroup2 cgroup rw", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotMatch, gotErr := parseMountInfoLine([]byte(tc.line), tc.source)
			if tc.wantErr {
				if gotErr == nil {
					t.Fatalf("parseMountInfoLine(%q) error = nil, want refusal", tc.line)
				}
				return
			}
			if gotErr != nil || gotMatch != tc.wantMatch {
				t.Fatalf("parseMountInfoLine(%q) = (%v, %t, %v), want match %t", tc.line, got, gotMatch, gotErr, tc.wantMatch)
			}
			if gotMatch && (got.root != tc.wantRoot || got.mountPoint.String() != tc.wantMount || got.source != tc.source) {
				t.Fatalf("parseMountInfoLine(%q) = %+v, want root %q mount %q source %v", tc.line, got, tc.wantRoot, tc.wantMount, tc.source)
			}
		})
	}
}

func TestCgroupMountSelectionUsesMostSpecificVisibleRootAndRejectsAmbiguity(t *testing.T) {
	t.Parallel()

	membership := cgroupMembership{
		path: "/tenant/team/job", source: WorkloadMemoryLimitSourceCgroupV2,
	}
	rootMount := cgroupMount{
		root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, "/cgroup-root"),
		source: WorkloadMemoryLimitSourceCgroupV2,
	}
	tenantMount := cgroupMount{
		root: "/tenant", mountPoint: mustAbsolutePathForHostfactsTest(t, "/cgroup-tenant"),
		source: WorkloadMemoryLimitSourceCgroupV2,
	}
	unrelatedMount := cgroupMount{
		root: "/other", mountPoint: mustAbsolutePathForHostfactsTest(t, "/cgroup-other"),
		source: WorkloadMemoryLimitSourceCgroupV2,
	}
	var selection cgroupMountSelection
	for _, candidate := range []cgroupMount{unrelatedMount, rootMount, tenantMount} {
		if err := selection.consider(candidate, membership); err != nil {
			t.Fatalf("cgroupMountSelection.consider(%v) error = %v, want nil", candidate, err)
		}
	}
	if selection.count != 1 || selection.selected != tenantMount {
		t.Fatalf("cgroup mount selection = %+v, want unique most-specific %+v", selection, tenantMount)
	}

	duplicate := tenantMount
	duplicate.mountPoint = mustAbsolutePathForHostfactsTest(t, "/duplicate-tenant")
	if err := selection.consider(duplicate, membership); err != nil {
		t.Fatalf("cgroupMountSelection.consider(duplicate) error = %v, want nil", err)
	}
	if selection.count != 2 {
		t.Fatalf("cgroup mount selection duplicate count = %d, want 2 ambiguity", selection.count)
	}
}

func TestCgroupLimitTokenHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		token         string
		wantValue     uint64
		source        WorkloadMemoryLimitSource
		wantUnlimited bool
		wantErr       bool
	}{
		{name: "v2 zero is a finite exhausted limit", token: "0", source: WorkloadMemoryLimitSourceCgroupV2, wantValue: 0},
		{name: "v2 zero newline is finite", token: "0\n", source: WorkloadMemoryLimitSourceCgroupV2, wantValue: 0},
		{name: "v2 one is finite", token: "1", source: WorkloadMemoryLimitSourceCgroupV2, wantValue: 1},
		{name: "v2 maximum uint is finite", token: strconv.FormatUint(math.MaxUint64, 10), source: WorkloadMemoryLimitSourceCgroupV2, wantValue: math.MaxUint64},
		{name: "v2 max is unlimited", token: "max", source: WorkloadMemoryLimitSourceCgroupV2, wantUnlimited: true},
		{name: "v2 max newline is unlimited", token: "max\n", source: WorkloadMemoryLimitSourceCgroupV2, wantUnlimited: true},
		{name: "v1 zero is finite", token: "0", source: WorkloadMemoryLimitSourceCgroupV1, wantValue: 0},
		{name: "v1 one below unlimited sentinel is finite", token: strconv.FormatUint(cgroupV1UnlimitedMin-1, 10), source: WorkloadMemoryLimitSourceCgroupV1, wantValue: cgroupV1UnlimitedMin - 1},
		{name: "v1 sentinel is unlimited", token: strconv.FormatUint(cgroupV1UnlimitedMin, 10), source: WorkloadMemoryLimitSourceCgroupV1, wantUnlimited: true},
		{name: "v1 maximum uint is unlimited", token: strconv.FormatUint(math.MaxUint64, 10), source: WorkloadMemoryLimitSourceCgroupV1, wantUnlimited: true},
		{name: "empty token is malformed", token: "", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "newline-only token is malformed", token: "\n", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "leading zero is noncanonical", token: "00", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "leading plus is noncanonical", token: "+1", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "negative value is malformed", token: "-1", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "leading space is malformed", token: " 1", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "trailing space is malformed", token: "1 ", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "double newline is malformed", token: "1\n\n", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
		{name: "v1 max word is malformed", token: "max", source: WorkloadMemoryLimitSourceCgroupV1, wantErr: true},
		{name: "uint overflow is malformed", token: "18446744073709551616", source: WorkloadMemoryLimitSourceCgroupV2, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			valuePath := filepath.Join(root, "limit")
			if err := os.WriteFile(valuePath, []byte(tc.token), 0o600); err != nil {
				t.Fatalf("os.WriteFile(limit) error = %v", err)
			}
			gotValue, gotUnlimited, gotErr := readCgroupLimit(
				context.Background(),
				mustAbsolutePathForHostfactsTest(t, valuePath),
				tc.source,
			)
			if tc.wantErr {
				if gotErr == nil {
					t.Fatalf("readCgroupLimit(%q) error = nil, want refusal", tc.token)
				}
				return
			}
			if gotErr != nil || gotValue != tc.wantValue || gotUnlimited != tc.wantUnlimited {
				t.Fatalf(
					"readCgroupLimit(%q) = (%d, %t, %v), want (%d, %t, nil)",
					tc.token,
					gotValue,
					gotUnlimited,
					gotErr,
					tc.wantValue,
					tc.wantUnlimited,
				)
			}
		})
	}
}

func TestCgroupLevelLimitExhaustsDeclarationCombinations(t *testing.T) {
	t.Parallel()
	unknown := cgroupLevelLimitStateUnknown.String()
	for raw := 0; raw <= math.MaxUint8; raw++ {
		state := cgroupLevelLimitState(raw)
		admitted := state == cgroupLevelLimitAbsent ||
			state == cgroupLevelLimitFinite ||
			state == cgroupLevelLimitUnlimited
		if state.IsValid() != admitted || (state.Validate() == nil) != admitted {
			t.Errorf("cgroupLevelLimitState(%d) validity disagrees with admitted=%t", raw, admitted)
		}
		if label := state.String(); label == "" || (!admitted && label != unknown) {
			t.Errorf("cgroupLevelLimitState(%d).String() = %q, want safe closed-domain label", raw, label)
		}
	}

	cases := []struct {
		name    string
		level   cgroupLevelLimit
		wantErr bool
	}{
		{name: "unset zero state is rejected", level: cgroupLevelLimit{}, wantErr: true},
		{name: "observed absence is valid", level: cgroupLevelLimit{state: cgroupLevelLimitAbsent}},
		{name: "finite zero is valid", level: cgroupLevelLimit{state: cgroupLevelLimitFinite}},
		{name: "finite one is valid", level: cgroupLevelLimit{state: cgroupLevelLimitFinite, value: 1}},
		{name: "finite maximum is valid", level: cgroupLevelLimit{state: cgroupLevelLimitFinite, value: math.MaxUint64}},
		{name: "unlimited zero is valid", level: cgroupLevelLimit{state: cgroupLevelLimitUnlimited}},
		{name: "absent with value is contradictory", level: cgroupLevelLimit{state: cgroupLevelLimitAbsent, value: 1}, wantErr: true},
		{name: "unlimited with value is contradictory", level: cgroupLevelLimit{state: cgroupLevelLimitUnlimited, value: 1}, wantErr: true},
		{name: "state limit is refused", level: cgroupLevelLimit{state: cgroupLevelLimitStateLimit}, wantErr: true},
		{name: "future state is refused", level: cgroupLevelLimit{state: cgroupLevelLimitState(255)}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.level.Validate()
			if gotFailure := gotErr != nil; gotFailure != tc.wantErr {
				t.Fatalf("cgroupLevelLimit%+v.Validate() error = %v, want failure %t", tc.level, gotErr, tc.wantErr)
			}
		})
	}
}

func TestCgroupAncestorFoldRealFilesystemLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive smallest finite ancestor is selected", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName,
			teamValue: "400", jobValue: "700",
		})
		mount := cgroupMount{root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: WorkloadMemoryLimitSourceCgroupV2}
		membership := cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}

		got, gotErr := foldCgroupLimits(context.Background(), membership, mount)
		if gotErr != nil {
			t.Fatalf("foldCgroupLimits() error = %v, want nil", gotErr)
		}
		limit, present := got.LimitBytes()
		path, pathPresent := got.InterfacePath()
		wantPath := filepath.Join(root, "team", cgroupV2LimitName)
		if !present || limit.Uint64() != 400 || !pathPresent || path.String() != wantPath ||
			got.State() != WorkloadMemoryLimitLimited {
			t.Fatalf("foldCgroupLimits() = %v limit=%d/%t path=%s/%t, want finite 400 at %s", got, limit.Uint64(), present, path.String(), pathPresent, wantPath)
		}
	})

	t.Run("negative malformed ancestor fails instead of falling back", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName,
			teamValue: "broken", jobValue: "700",
		})
		mount := cgroupMount{root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: WorkloadMemoryLimitSourceCgroupV2}
		membership := cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}

		got, gotErr := foldCgroupLimits(context.Background(), membership, mount)
		if got != (WorkloadMemoryLimit{}) || gotErr == nil {
			t.Fatalf("foldCgroupLimits(malformed ancestor) = (%v, %v), want zero refusal", got, gotErr)
		}
	})

	t.Run("neutral all unlimited ancestors remain unlimited", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName,
			teamValue: "max", jobValue: "max",
		})
		mount := cgroupMount{root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: WorkloadMemoryLimitSourceCgroupV2}
		membership := cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}

		got, gotErr := foldCgroupLimits(context.Background(), membership, mount)
		if gotErr != nil || got.State() != WorkloadMemoryLimitUnlimited || got.Validate() != nil {
			t.Fatalf("foldCgroupLimits(all unlimited) = (%v, %v), want valid unlimited", got, gotErr)
		}
	})

	t.Run("zero current limit survives as exact finite evidence", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName,
			teamValue: "max", jobValue: "0",
		})
		mount := cgroupMount{root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: WorkloadMemoryLimitSourceCgroupV2}
		membership := cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}

		got, gotErr := foldCgroupLimits(context.Background(), membership, mount)
		limit, present := got.LimitBytes()
		if gotErr != nil || got.State() != WorkloadMemoryLimitLimited || !present || limit.Uint64() != 0 {
			t.Fatalf("foldCgroupLimits(zero) = (%v, %v) limit=%d/%t, want finite zero", got, gotErr, limit.Uint64(), present)
		}
	})
}

func TestBoundedVirtualReadersRefuseOversizeNoProgressAndInvalidContracts(t *testing.T) {
	t.Parallel()

	if gotErr := (virtualFileRequest{
		Path:         mustAbsolutePathForHostfactsTest(t, t.TempDir()),
		MaximumBytes: virtualFileMaximumBytes + 1,
	}).Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("virtualFileRequest(over package maximum).Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}
	if _, gotErr := readBoundedValue(context.Background(), bytes.NewReader(bytes.Repeat([]byte{'x'}, 129)), 128); !errors.Is(gotErr, core.ErrHostFactsObservation) {
		t.Fatalf("readBoundedValue(over maximum) error = %v, want %v", gotErr, core.ErrHostFactsObservation)
	}
	if _, gotErr := readBoundedValue(context.Background(), zeroReader{}, 1); !errors.Is(gotErr, io.ErrNoProgress) {
		t.Fatalf("readBoundedValue(no progress) error = %v, want %v", gotErr, io.ErrNoProgress)
	}
	native := errors.New("virtual file failed")
	if _, gotErr := readBoundedValue(context.Background(), errorReader{err: native}, 1); !errors.Is(gotErr, native) {
		t.Fatalf("readBoundedValue(native failure) error = %v, want %v", gotErr, native)
	}
}

func TestBoundedLineScannerHostileStreamTable(t *testing.T) {
	t.Parallel()

	native := errors.New("line source failed")
	visitorFailure := errors.New("line visitor failed")
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		ctx       context.Context
		reader    io.Reader
		visitErr  error
		wantErr   error
		name      string
		wantText  string
		maximum   uint64
		wantLines int
	}{
		{name: "one terminated line", ctx: context.Background(), reader: bytes.NewBufferString("one\n"), maximum: 4, wantLines: 1, wantText: "one"},
		{name: "one unterminated final line", ctx: context.Background(), reader: bytes.NewBufferString("one"), maximum: 3, wantLines: 1, wantText: "one"},
		{name: "three terminated lines", ctx: context.Background(), reader: bytes.NewBufferString("one\ntwo\nthree\n"), maximum: 14, wantLines: 3, wantText: "onetwothree"},
		{name: "line split every byte", ctx: context.Background(), reader: &chunkReader{data: []byte("one\ntwo\n"), maximum: 1}, maximum: 8, wantLines: 2, wantText: "onetwo"},
		{name: "empty stream is neutral", ctx: context.Background(), reader: bytes.NewReader(nil), maximum: 1, wantLines: 0},
		{name: "blank terminated line is refused", ctx: context.Background(), reader: bytes.NewBufferString("\n"), maximum: 1, wantErr: core.ErrHostFactsObservation},
		{name: "blank line between values is refused", ctx: context.Background(), reader: bytes.NewBufferString("one\n\ntwo\n"), maximum: 9, wantErr: core.ErrHostFactsObservation},
		{name: "document one byte over maximum is refused", ctx: context.Background(), reader: bytes.NewBufferString("1234"), maximum: 3, wantErr: core.ErrHostFactsObservation},
		{name: "line one byte over maximum is refused", ctx: context.Background(), reader: bytes.NewReader(bytes.Repeat([]byte{'x'}, procLineMaximumBytes+1)), maximum: procLineMaximumBytes + 1, wantErr: core.ErrHostFactsObservation},
		{name: "visitor failure is preserved", ctx: context.Background(), reader: bytes.NewBufferString("one\n"), maximum: 4, visitErr: visitorFailure, wantErr: visitorFailure},
		{name: "native reader failure is preserved", ctx: context.Background(), reader: errorReader{err: native}, maximum: 1, wantErr: native},
		{name: "zero progress is refused", ctx: context.Background(), reader: zeroReader{}, maximum: 1, wantErr: io.ErrNoProgress},
		{name: "cancelled context is observed before reading", ctx: cancelled, reader: bytes.NewBufferString("one\n"), maximum: 4, wantErr: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			lines := 0
			var text bytes.Buffer
			gotErr := (boundedLineScan{
				reader: tc.reader, maximum: tc.maximum,
				visit: func(line []byte) error {
					if tc.visitErr != nil {
						return tc.visitErr
					}
					lines++
					_, err := text.Write(line)
					return err
				},
			}).run(tc.ctx)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("boundedLineScan.run() error = %v, want %v", gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || lines != tc.wantLines || text.String() != tc.wantText {
				t.Fatalf(
					"boundedLineScan.run() = lines %d text %q error %v, want lines %d text %q nil",
					lines,
					text.String(),
					gotErr,
					tc.wantLines,
					tc.wantText,
				)
			}
		})
	}
}

func BenchmarkCgroupMountInfoStreaming1KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkCgroupMountInfoStreaming(b, 1<<10)
}

func BenchmarkCgroupMountInfoStreaming1MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkCgroupMountInfoStreaming(b, 1<<20)
}

func benchmarkCgroupMountInfoStreaming(b *testing.B, size int) {
	b.Helper()
	line := []byte("30 23 0:27 / /sys/fs/cgroup rw,nosuid,nodev,noexec,relatime - ext4 /dev/disk rw\n")
	data := bytes.Repeat(line, size/len(line))
	b.ResetTimer()

	for b.Loop() {
		lines := 0
		err := (boundedLineScan{
			reader: bytes.NewReader(data), maximum: uint64(size),
			visit: func(line []byte) error {
				_, matched, err := parseMountInfoLine(line, WorkloadMemoryLimitSourceCgroupV2)
				if err != nil || matched {
					return errors.Join(core.ErrHostFactsObservation, err)
				}
				lines++
				return nil
			},
		}).run(context.Background())
		if err != nil || lines == 0 {
			b.Fatalf("boundedLineScan(%d bytes) = %d lines, %v; want positive lines and nil", size, lines, err)
		}
	}
}

type cgroupHierarchyFixture struct {
	root      string
	limitName string
	rootValue string
	teamValue string
	jobValue  string
}

type cgroupFileFixture struct {
	path  string
	value string
}

type cgroupFixtureLevel uint8

const (
	cgroupFixtureLevelRoot cgroupFixtureLevel = iota + 1
	cgroupFixtureLevelTeam
	cgroupFixtureLevelJob
)

func writeCgroupHierarchyForTest(t *testing.T, fixture cgroupHierarchyFixture) {
	t.Helper()
	team := filepath.Join(fixture.root, "team")
	job := filepath.Join(team, "job")
	if err := os.MkdirAll(job, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", job, err)
	}
	files := []cgroupFileFixture{
		{path: filepath.Join(fixture.root, fixture.limitName), value: fixture.rootValue},
		{path: filepath.Join(team, fixture.limitName), value: fixture.teamValue},
		{path: filepath.Join(job, fixture.limitName), value: fixture.jobValue},
	}
	for _, file := range files {
		// An empty fixture value means the real interface file is absent, not
		// present with malformed empty content.
		if file.value == "" {
			continue
		}
		if err := os.WriteFile(file.path, []byte(file.value+"\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v", file.path, err)
		}
	}
}

func TestCgroupFoldTreatsAnAbsentInterfaceAsNoDeclarationLayerTriad(t *testing.T) {
	t.Parallel()

	// The kernel exposes memory.max only on non-root cgroups, so the mount root
	// this fold always walks to declares nothing on every real cgroup v2 host.
	// Absence has to fold in as "this level sets no ceiling" or the observation
	// fails on the ordinary shape rather than the exotic one.
	t.Run("positive finite descendant survives an absent root declaration", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName,
			teamValue: "400", jobValue: "700",
		})
		got, gotErr := foldCgroupLimits(context.Background(), cgroupV2MembershipForTest(), cgroupV2MountForTest(t, root))
		limit, present := got.LimitBytes()
		path, pathPresent := got.InterfacePath()
		wantPath := filepath.Join(root, "team", cgroupV2LimitName)
		if gotErr != nil || got.State() != WorkloadMemoryLimitLimited || !present ||
			limit.Uint64() != 400 || !pathPresent || path.String() != wantPath {
			t.Fatalf(
				"foldCgroupLimits(absent root interface) = (%v, %v) limit=%d/%t path=%s, want finite 400 at %s",
				got, gotErr, limit.Uint64(), present, path.String(), wantPath,
			)
		}
	})

	t.Run("negative a corrupt declaration is never absorbed as absence", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName, jobValue: "broken",
		})
		got, gotErr := foldCgroupLimits(context.Background(), cgroupV2MembershipForTest(), cgroupV2MountForTest(t, root))
		if got != (WorkloadMemoryLimit{}) || !errors.Is(gotErr, core.ErrHostFactsObservation) {
			t.Fatalf("foldCgroupLimits(corrupt declaration) = (%v, %v), want zero refusal", got, gotErr)
		}
	})

	t.Run("neutral a hierarchy declaring nothing anywhere is unavailable", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeCgroupHierarchyForTest(t, cgroupHierarchyFixture{
			root: root, limitName: cgroupV2LimitName,
		})
		got, gotErr := foldCgroupLimits(context.Background(), cgroupV2MembershipForTest(), cgroupV2MountForTest(t, root))
		path, pathPresent := got.InterfacePath()
		limit, limitPresent := got.LimitBytes()
		if gotErr != nil || got.State() != WorkloadMemoryLimitUnavailable ||
			got.Source() != WorkloadMemoryLimitSourceNone || got.Validate() != nil ||
			pathPresent || limitPresent || path.String() != "" || limit.Uint64() != 0 {
			t.Fatalf(
				"foldCgroupLimits(no declarations) = (%v, %v) path=%s/%t limit=%d/%t, want unavailable with nothing present",
				got, gotErr, path.String(), pathPresent, limit.Uint64(), limitPresent,
			)
		}
	})
}

func TestCgroupUnlimitedFoldReportsTheClosestPresentInterface(t *testing.T) {
	t.Parallel()

	presenceCases := []struct {
		name      string
		rootSet   bool
		teamSet   bool
		jobSet    bool
		wantLevel cgroupFixtureLevel
	}{
		{name: "all interfaces present selects current cgroup", rootSet: true, teamSet: true, jobSet: true, wantLevel: cgroupFixtureLevelJob},
		{name: "current and parent present selects current cgroup", teamSet: true, jobSet: true, wantLevel: cgroupFixtureLevelJob},
		{name: "current and root present selects current cgroup", rootSet: true, jobSet: true, wantLevel: cgroupFixtureLevelJob},
		{name: "only current interface present selects current cgroup", jobSet: true, wantLevel: cgroupFixtureLevelJob},
		{name: "parent and root present selects parent", rootSet: true, teamSet: true, wantLevel: cgroupFixtureLevelTeam},
		{name: "only parent interface present selects parent", teamSet: true, wantLevel: cgroupFixtureLevelTeam},
		{name: "only root interface present selects root", rootSet: true, wantLevel: cgroupFixtureLevelRoot},
	}
	providers := []struct {
		name      string
		limitName string
		unlimited string
		source    WorkloadMemoryLimitSource
	}{
		{name: "cgroup v2", limitName: cgroupV2LimitName, unlimited: "max", source: WorkloadMemoryLimitSourceCgroupV2},
		{name: "cgroup v1", limitName: cgroupV1LimitName, unlimited: strconv.FormatUint(cgroupV1UnlimitedMin, 10), source: WorkloadMemoryLimitSourceCgroupV1},
	}
	for _, provider := range providers {
		for _, tc := range presenceCases {
			t.Run(provider.name+" "+tc.name, func(t *testing.T) {
				t.Parallel()

				root := t.TempDir()
				fixture := cgroupHierarchyFixture{root: root, limitName: provider.limitName}
				if tc.rootSet {
					fixture.rootValue = provider.unlimited
				}
				if tc.teamSet {
					fixture.teamValue = provider.unlimited
				}
				if tc.jobSet {
					fixture.jobValue = provider.unlimited
				}
				writeCgroupHierarchyForTest(t, fixture)
				membership := cgroupMembership{path: "/team/job", source: provider.source}
				mount := cgroupMount{
					root: "/", mountPoint: mustAbsolutePathForHostfactsTest(t, root), source: provider.source,
				}
				got, gotErr := foldCgroupLimits(context.Background(), membership, mount)
				path, present := got.InterfacePath()
				wantPath := filepath.Join(root, provider.limitName)
				switch tc.wantLevel {
				case cgroupFixtureLevelJob:
					wantPath = filepath.Join(root, "team", "job", provider.limitName)
				case cgroupFixtureLevelTeam:
					wantPath = filepath.Join(root, "team", provider.limitName)
				}
				if gotErr != nil || got.State() != WorkloadMemoryLimitUnlimited ||
					got.Source() != provider.source || !present || path.String() != wantPath || got.Validate() != nil {
					t.Fatalf(
						"foldCgroupLimits(%s unlimited).InterfacePath() = %s/%t (source %v, state %v, err %v), want %s/%v",
						provider.name, path.String(), present, got.Source(), got.State(), gotErr, wantPath, provider.source,
					)
				}
			})
		}
	}
}

func TestCgroupMountAmbiguityTallySaturatesInsteadOfWrapping(t *testing.T) {
	t.Parallel()

	// The tally only distinguishes one mount from more than one. An eight-bit
	// tally that keeps counting returns to one after 256 equally-specific mounts,
	// which reports a hostile mount table as unambiguous.
	membership := cgroupMembership{path: "/tenant/job", source: WorkloadMemoryLimitSourceCgroupV2}
	var selection cgroupMountSelection
	for index := range 1 + 2*int(math.MaxUint8) {
		candidate := cgroupMount{
			root:       "/tenant",
			mountPoint: mustAbsolutePathForHostfactsTest(t, "/mount"+strconv.Itoa(index)),
			source:     WorkloadMemoryLimitSourceCgroupV2,
		}
		if err := selection.consider(candidate, membership); err != nil {
			t.Fatalf("cgroupMountSelection.consider(%d) error = %v, want nil", index, err)
		}
		if selection.count < cgroupMountAmbiguityCeiling && index > 0 {
			t.Fatalf("cgroup mount ambiguity tally = %d after %d equal mounts, want ambiguous", selection.count, index+1)
		}
	}
}

func cgroupV2MembershipForTest() cgroupMembership {
	return cgroupMembership{path: "/team/job", source: WorkloadMemoryLimitSourceCgroupV2}
}

func cgroupV2MountForTest(t *testing.T, root string) cgroupMount {
	t.Helper()
	return cgroupMount{
		root:       "/",
		mountPoint: mustAbsolutePathForHostfactsTest(t, root),
		source:     WorkloadMemoryLimitSourceCgroupV2,
	}
}
