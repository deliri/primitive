package hostfacts

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// cpu.stat is flat-keyed "KEY VALUE" per line; only usage_usec is retained.
// https://docs.kernel.org/admin-guide/cgroup-v2.html
func TestCPUStatFoldLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       uint64
		wantErr    error
	}{
		{name: "kernel layout", text: "usage_usec 1234\nuser_usec 1000\nsystem_usec 234\nnice_usec 0\n", want: 1234},
		{name: "usage last", text: "user_usec 1\nsystem_usec 2\nusage_usec 3\n", want: 3},
		{name: "zero usage", text: "usage_usec 0\n"},
		{name: "maximum counter", text: "usage_usec 18446744073709551615\n", want: math.MaxUint64},
		{name: "counter overflow", text: "usage_usec 18446744073709551616\n", wantErr: core.ErrHostFactsObservation},
		{name: "missing usage", text: "user_usec 1\n", wantErr: core.ErrHostFactsObservation},
		{name: "empty file", text: "", wantErr: core.ErrHostFactsObservation},
		{name: "duplicate usage", text: "usage_usec 1\nusage_usec 2\n", wantErr: core.ErrHostFactsObservation},
		{name: "signed value", text: "usage_usec +1\n", wantErr: core.ErrHostFactsObservation},
		{name: "negative value", text: "usage_usec -1\n", wantErr: core.ErrHostFactsObservation},
		{name: "empty value", text: "usage_usec \n", wantErr: core.ErrHostFactsObservation},
		{name: "extra field", text: "usage_usec 1 2\n", wantErr: core.ErrHostFactsObservation},
		{name: "missing separator", text: "usage_usec\n", wantErr: core.ErrHostFactsObservation},
		{name: "prefix key is not usage", text: "usage_usec_total 9\nusage_usec 4\n", want: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var fold cpuStatFold
			var err error
			for _, line := range strings.SplitAfter(tc.text, "\n") {
				line = strings.TrimSuffix(line, "\n")
				if line == "" {
					continue
				}
				if err = fold.visit([]byte(line)); err != nil {
					break
				}
			}
			if err == nil && !fold.seen {
				err = core.ErrHostFactsObservation
			}
			if !errors.Is(err, tc.wantErr) || tc.wantErr == nil && fold.usage != tc.want {
				t.Fatalf("fold=(%d,%v), want (%d,%v)", fold.usage, err, tc.want, tc.wantErr)
			}
		})
	}
}

// systemctl show --value prints the unit's ControlGroup path or nothing.
func TestParseSystemdControlGroupLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text, want string
		wantErr          error
	}{
		{name: "user scope", text: "/user.slice/user-1000.slice/user@1000.service/proof.slice/a.scope\n", want: "/user.slice/user-1000.slice/user@1000.service/proof.slice/a.scope"},
		{name: "without newline", text: "/system.slice/a.scope", want: "/system.slice/a.scope"},
		{name: "no control group", text: "\n"},
		{name: "empty output", text: ""},
		{name: "relative path", text: "system.slice/a.scope\n", wantErr: core.ErrHostFactsObservation},
		{name: "unclean path", text: "/a/../b.scope\n", wantErr: core.ErrHostFactsObservation},
		{name: "two lines", text: "/a.scope\n/b.scope\n", wantErr: core.ErrHostFactsObservation},
		{name: "carriage return", text: "/a.scope\r\n", wantErr: core.ErrHostFactsObservation},
		{name: "deleted cgroup", text: "/a.scope (deleted)\n", wantErr: core.ErrHostFactsObservation},
		{name: "NUL", text: "/a\x00.scope\n", wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSystemdControlGroup([]byte(tc.text))
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("control group=(%q,%v), want (%q,%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

// A scope's rate is its CPU delta over the whole host window, rounded down.
func TestScopeBusyMilliLayerTriad(t *testing.T) {
	t.Parallel()
	const second = int64(1_000_000_000)
	for _, tc := range []struct {
		name          string
		before, after uint64
		window        int64
		want          uint64
		present       bool
		wantErr       error
	}{
		{name: "one full CPU", after: 1_000_000, window: second, want: 1000, present: true},
		{name: "two CPUs over two seconds", before: 5, after: 4_000_005, window: 2 * second, want: 2000, present: true},
		{name: "one milli exactly", after: 1000, window: second, want: 1, present: true},
		{name: "just below one milli rounds down", after: 999, window: second, present: true},
		{name: "idle scope", before: 7, after: 7, window: second, present: true},
		{name: "longer window lowers rate", after: 1_000_000, window: second + 1, want: 999, present: true},
		{name: "replaced counter is absent", before: 9, after: 8, window: second},
		{name: "zero window refused", after: 1, wantErr: core.ErrHostFactsObservation},
		{name: "negative window refused", after: 1, window: -1, wantErr: core.ErrHostFactsObservation},
		{name: "quotient overflow refused", after: math.MaxUint64, window: 1, wantErr: core.ErrHostFactsObservation},
		{name: "largest delta over large window", after: math.MaxUint64, window: math.MaxInt64, want: 2000000, present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, present, err := scopeBusyMilli(tc.before, tc.after, tc.window)
			if !errors.Is(err, tc.wantErr) || got != tc.want || present != tc.present {
				t.Fatalf("busy=(%d,%v,%v), want (%d,%v,%v)", got, present, err, tc.want, tc.present, tc.wantErr)
			}
		})
	}
}

func TestScopesBusyReportsAbsenceAsZero(t *testing.T) {
	t.Parallel()
	units := []SystemdScopeRequest{scopeRequest(t, "a.scope"), scopeRequest(t, "b.scope"), scopeRequest(t, "c.scope")}
	cgroups := []scopeCgroup{{scope: units[0], present: true}, {scope: units[1], present: true}, {scope: units[2]}}
	before := []scopeSample{{usage: 10, present: true}, {usage: 10, present: true}, {}}
	after := []scopeSample{{usage: 500_010, present: true}, {}, {}}
	got, err := scopesBusy(cgroups, before, after, 1_000_000_000)
	if err != nil || len(got) != 3 {
		t.Fatalf("scopes=(%+v,%v)", got, err)
	}
	want := []ScopeCPU{{Scope: units[0], BusyCPUMilli: 500, Present: true}, {Scope: units[1]}, {Scope: units[2]}}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scope %d=%+v, want %+v", i, got[i], want[i])
		}
	}
	if _, err := scopesBusy(cgroups, before[:2], after, 1); !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("short readings=%v, want observation refusal", err)
	}
}

func TestResourceUsageScopeContracts(t *testing.T) {
	t.Parallel()
	a, b := scopeRequest(t, "a.scope"), scopeRequest(t, "b.scope")
	userA := SystemdScopeRequest{Unit: a.Unit, UserManager: true}
	for _, tc := range []struct {
		name    string
		scopes  []SystemdScopeRequest
		wantErr error
	}{
		{name: "no scopes"},
		{name: "distinct scopes", scopes: []SystemdScopeRequest{a, b}},
		{name: "same unit in two managers", scopes: []SystemdScopeRequest{a, userA}},
		{name: "duplicate scope refused", scopes: []SystemdScopeRequest{a, b, a}, wantErr: core.ErrHostFactsContract},
		{name: "invalid unit refused", scopes: []SystemdScopeRequest{{}}, wantErr: core.ErrHostFactsContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ResourceUsageRequest{Interval: resourceDuration(t, "1s"), Scopes: tc.scopes}.Validate()
			if !errors.Is(err, tc.wantErr) || tc.wantErr == nil && err != nil {
				t.Fatalf("validate=%v, want %v", err, tc.wantErr)
			}
		})
	}
	sample := ResourceUsage{window: resourceDuration(t, "1ns"), cpus: LogicalCPUCount{count: 2}, total: byteLength(t, 2), available: byteLength(t, 1)}
	for _, tc := range []struct {
		name   string
		scope  ScopeCPU
		accept bool
	}{
		{name: "present at host ceiling", scope: ScopeCPU{Scope: a, BusyCPUMilli: 2000, Present: true}, accept: true},
		{name: "absent zero", scope: ScopeCPU{Scope: a}, accept: true},
		{name: "above host CPUs", scope: ScopeCPU{Scope: a, BusyCPUMilli: 2001, Present: true}},
		{name: "absent with usage", scope: ScopeCPU{Scope: a, BusyCPUMilli: 1}},
		{name: "invalid unit", scope: ScopeCPU{Present: true}},
	} {
		sample.scopes = []ScopeCPU{tc.scope}
		if err := sample.Validate(); (err == nil) != tc.accept || err != nil && !errors.Is(err, core.ErrHostFactsContract) {
			t.Fatalf("%s: validate=%v, accept %v", tc.name, err, tc.accept)
		}
	}
	if err := sameScopes([]SystemdScopeRequest{a, b}, []ScopeCPU{{Scope: b}, {Scope: a}}); !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("reordered scopes=%v, want observation refusal", err)
	}
	if err := sameScopes([]SystemdScopeRequest{a}, nil); !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("missing scope=%v, want observation refusal", err)
	}
	usage := ResourceUsage{scopes: []ScopeCPU{{Scope: a}}}
	copied := usage.Scopes()
	copied[0].Present = true
	if usage.scopes[0].Present {
		t.Fatal("Scopes exposed internal storage")
	}
}

// Lines are generated from typed facts; the oracle recomputes the retained
// counter independently of the fold.
func FuzzCPUStatFoldNativeLines(f *testing.F) {
	f.Add("usage_usec", uint64(7), true, "user_usec", uint64(3))
	f.Add("usage_usec", uint64(math.MaxUint64), false, "usage_usec", uint64(1))
	f.Add("nice_usec", uint64(0), true, "usage_usec", uint64(0))
	f.Fuzz(func(t *testing.T, firstKey string, first uint64, plus bool, secondKey string, second uint64) {
		value := strconv.FormatUint(first, 10)
		if plus {
			value = "+" + value
		}
		lines := []string{firstKey + " " + value, secondKey + " " + strconv.FormatUint(second, 10)}
		var fold cpuStatFold
		var err error
		for _, line := range lines {
			if err = fold.visit([]byte(line)); err != nil {
				break
			}
		}
		// A key containing a separator changes the native line shape itself.
		malformed := func(key string) bool { return strings.ContainsAny(key, " \n") }
		if malformed(firstKey) || malformed(secondKey) {
			return
		}
		usageCount := 0
		var want uint64
		refused := firstKey == "" || secondKey == ""
		if firstKey == cgroupCPUUsageKey {
			usageCount++
			want = first
			refused = refused || plus
		}
		if secondKey == cgroupCPUUsageKey {
			usageCount++
			want = second
		}
		refused = refused || usageCount > 1
		if refused {
			if err == nil {
				t.Fatalf("%q accepted", lines)
			}
			return
		}
		if err != nil || fold.seen != (usageCount == 1) || usageCount == 1 && fold.usage != want {
			t.Fatalf("%q fold=(%d,%v,%v), want %d once=%v", lines, fold.usage, fold.seen, err, want, usageCount == 1)
		}
	})
}

func scopeRequest(t *testing.T, unit string) SystemdScopeRequest {
	t.Helper()
	name, err := core.ParsePathComponent(unit)
	if err != nil {
		t.Fatal(err)
	}
	return SystemdScopeRequest{Unit: name}
}

func byteLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatal(err)
	}
	return length
}
