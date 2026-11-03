package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestWorkloadMemoryHeadroomSelectsRemainingAncestor(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, childLimit, childCurrent, parentLimit, parentCurrent string
		want                                                       uint64
		parent                                                     bool
		state                                                      WorkloadMemoryLimitState
	}{
		{name: "larger parent has least remaining", childLimit: "1024", childCurrent: "256", parentLimit: "2048", parentCurrent: "1984", want: 64, parent: true, state: WorkloadMemoryLimitLimited},
		{name: "child has least remaining", childLimit: "2048", childCurrent: "1984", parentLimit: "1024", parentCurrent: "256", want: 64, state: WorkloadMemoryLimitLimited},
		{name: "at parent limit is exhausted", childLimit: "1024", childCurrent: "256", parentLimit: "2048", parentCurrent: "2048", parent: true, state: WorkloadMemoryLimitLimited},
		{name: "above parent limit never wraps", childLimit: "1024", childCurrent: "256", parentLimit: "2048", parentCurrent: "2049", parent: true, state: WorkloadMemoryLimitLimited},
		{name: "finite zero stays present", childLimit: "0", childCurrent: "0", parentLimit: "2048", parentCurrent: "1984", state: WorkloadMemoryLimitLimited},
		{name: "unlimited child retains parent", childLimit: "max", parentLimit: "2048", parentCurrent: "1984", want: 64, parent: true, state: WorkloadMemoryLimitLimited},
		{name: "unlimited parent retains child", childLimit: "1024", childCurrent: "256", parentLimit: "max", want: 768, state: WorkloadMemoryLimitLimited},
		{name: "all unlimited differs from absence", childLimit: "max", parentLimit: "max", state: WorkloadMemoryLimitUnlimited},
		{name: "no interfaces unavailable", state: WorkloadMemoryLimitUnavailable},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			mount := filepath.Join(root, "cgroup")
			child := filepath.Join(mount, "job")
			if err := os.MkdirAll(child, 0o700); err != nil {
				t.Fatal(err)
			}
			proc, mounts := filepath.Join(root, "membership"), filepath.Join(root, "mountinfo")
			for path, data := range map[string]string{proc: "0::/job\n", mounts: "30 23 0:27 / " + mount + " rw - cgroup2 cgroup rw\n"} {
				if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, file := range []struct{ path, data string }{{filepath.Join(child, "memory.max"), row.childLimit}, {filepath.Join(child, "memory.current"), row.childCurrent}, {filepath.Join(mount, "memory.max"), row.parentLimit}, {filepath.Join(mount, "memory.current"), row.parentCurrent}} {
				if file.data != "" {
					if err := os.WriteFile(file.path, []byte(file.data+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			p, err := core.ParseAbsolutePath(proc)
			if err != nil {
				t.Fatal(err)
			}
			m, err := core.ParseAbsolutePath(mounts)
			if err != nil {
				t.Fatal(err)
			}
			got, err := observeWorkloadMemoryHeadroomAt(t.Context(), p, m)
			if err != nil || got.Validate() != nil || got.BindingLimit().State() != row.state {
				t.Fatalf("headroom=%+v/%v, want state%s", got, err, row.state)
			}
			remaining, present := got.RemainingBytes()
			if remaining.Uint64() != row.want || present != (row.state == WorkloadMemoryLimitLimited) {
				t.Fatalf("remaining=%d/%t, want%d/%t", remaining.Uint64(), present, row.want, row.state == WorkloadMemoryLimitLimited)
			}
			if present {
				path, _ := got.BindingLimit().InterfacePath()
				want := filepath.Join(child, "memory.max")
				if row.parent {
					want = filepath.Join(mount, "memory.max")
				}
				if path.String() != want {
					t.Fatalf("binding=%s, want same ancestor%s", path, want)
				}
				current, charged := got.CurrentBytes()
				limit, limited := got.BindingLimit().LimitBytes()
				if !charged || !limited || current.Uint64() < limit.Uint64() && remaining.Uint64()+current.Uint64() != limit.Uint64() || current.Uint64() >= limit.Uint64() && remaining.Uint64() != 0 {
					t.Fatalf("pair=%+v, want native charge and saturating conservation", got)
				}
			}
		})
	}
}

func TestWorkloadMemoryHeadroomRefusesMissingOrMalformedCharge(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, value string
		missing     bool
		want        error
	}{
		{name: "finite charge missing", missing: true, want: fs.ErrNotExist},
		{name: "unlimited token is not charge", value: "max\n", want: core.ErrHostFactsObservation},
		{name: "noncanonical zero", value: "00\n", want: core.ErrHostFactsObservation},
		{name: "negative charge", value: "-1\n", want: core.ErrHostFactsObservation},
		{name: "trailing document", value: "1\n2\n", want: core.ErrHostFactsObservation},
		{name: "over maximum input", value: strings.Repeat("1", limitValueMaximumBytes+1), want: core.ErrHostFactsObservation},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "memory.max"), []byte("1024\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if !row.missing {
				if err := os.WriteFile(filepath.Join(directory, "memory.current"), []byte(row.value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path, err := core.ParseAbsolutePath(directory)
			if err != nil {
				t.Fatal(err)
			}
			component, err := cgroupV2LimitComponent()
			if err != nil {
				t.Fatal(err)
			}
			got, err := readWorkloadMemoryHeadroomLevel(t.Context(), path, component, WorkloadMemoryLimitSourceCgroupV2)
			if !errors.Is(err, row.want) || got != (WorkloadMemoryHeadroom{}) {
				t.Fatalf("charge refusal=%+v/%v, want zero and%v", got, err, row.want)
			}
		})
	}
}

func TestObserveWorkloadMemoryHeadroomNativeAndTerminalContext(t *testing.T) {
	t.Parallel()
	got, err := ObserveWorkloadMemoryHeadroom(t.Context())
	if err != nil || got.Validate() != nil {
		t.Fatalf("native headroom=%+v/%v, want validated observation", got, err)
	}
	binding := got.BindingLimit()
	limit, limited := binding.LimitBytes()
	current, charged := got.CurrentBytes()
	remaining, present := got.RemainingBytes()
	path, _ := binding.InterfacePath()
	if limited != charged || limited != present || limited && remaining.Uint64() > limit.Uint64() {
		t.Fatalf("native pair=%+v, want complete finite pairing", got)
	}
	if runtime.GOOS == "linux" && (binding.State() != WorkloadMemoryLimitLimited || !present || current.Uint64() == 0) {
		t.Fatalf("native managed Linux pair=%+v, want a finite charged cgroup", got)
	}
	if runtime.GOOS != "linux" && binding.State() != WorkloadMemoryLimitUnsupported {
		t.Fatalf("native state=%s on%s, want unsupported", binding.State(), runtime.GOOS)
	}
	t.Logf("native headroom state=%s source=%s limit=%d current=%d remaining=%d finite=%t binding=%s", binding.State(), binding.Source(), limit.Uint64(), current.Uint64(), remaining.Uint64(), present, path)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	refused, err := ObserveWorkloadMemoryHeadroom(ctx)
	if !errors.Is(err, context.Canceled) || refused != (WorkloadMemoryHeadroom{}) {
		t.Fatalf("cancelled observation=%+v/%v, want zero and cancellation", refused, err)
	}
	var absent context.Context
	refused, err = ObserveWorkloadMemoryHeadroom(absent)
	if !errors.Is(err, core.ErrNilContext) || refused != (WorkloadMemoryHeadroom{}) {
		t.Fatalf("nil observation=%+v/%v, want zero and nil-context identity", refused, err)
	}
}

func FuzzWorkloadMemoryChargeCanonical(f *testing.F) {
	for _, seed := range []string{"0", "1\n", "18446744073709551615\n", "max\n", "00", "-1", "1\n2", strings.Repeat("1", limitValueMaximumBytes+1)} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseWorkloadMemoryCharge(data)
		if len(data) > limitValueMaximumBytes {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != (core.ByteLength{}) {
				t.Fatalf("oversized charge=%v/%v, want zero typed refusal before oracle allocation", got, err)
			}
			return
		}
		token := strings.TrimSuffix(string(data), "\n")
		value, parseErr := strconv.ParseUint(token, 10, 64)
		_, nominalErr := core.NewByteLength(value)
		accepted := len(data) <= limitValueMaximumBytes && parseErr == nil && nominalErr == nil && strconv.FormatUint(value, 10) == token
		if !accepted {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != (core.ByteLength{}) {
				t.Fatalf("charge rejected=%v/%v, want zero typed refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Uint64() != value {
			t.Fatalf("charge accepted=%v/%v, want exact%d", got, err, value)
		}
		encoded := strconv.AppendUint(nil, got.Uint64(), 10)
		again, againErr := parseWorkloadMemoryCharge(encoded)
		if againErr != nil || again != got {
			t.Fatalf("canonical charge=%v/%v, want original%v", again, againErr, got)
		}
	})
}

func TestWorkloadMemoryHeadroomReadsV1Charge(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mount := filepath.Join(root, "memory")
	child := filepath.Join(mount, "job")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	proc, mounts := filepath.Join(root, "membership"), filepath.Join(root, "mountinfo")
	for _, file := range []struct{ path, data string }{
		{proc, "5:memory:/job\n"}, {mounts, "31 23 0:28 / " + mount + " rw - cgroup cgroup rw,memory\n"},
		{filepath.Join(child, "memory.limit_in_bytes"), "1024\n"}, {filepath.Join(child, "memory.usage_in_bytes"), "256\n"},
		{filepath.Join(mount, "memory.limit_in_bytes"), "2048\n"}, {filepath.Join(mount, "memory.usage_in_bytes"), "1984\n"},
	} {
		if err := os.WriteFile(file.path, []byte(file.data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := core.ParseAbsolutePath(proc)
	if err != nil {
		t.Fatal(err)
	}
	m, err := core.ParseAbsolutePath(mounts)
	if err != nil {
		t.Fatal(err)
	}
	got, err := observeWorkloadMemoryHeadroomAt(t.Context(), p, m)
	remaining, present := got.RemainingBytes()
	current, charged := got.CurrentBytes()
	path, _ := got.BindingLimit().InterfacePath()
	if err != nil || got.Validate() != nil || got.BindingLimit().Source() != WorkloadMemoryLimitSourceCgroupV1 || !present || !charged || remaining.Uint64() != 64 || current.Uint64() != 1984 || path.String() != filepath.Join(mount, "memory.limit_in_bytes") {
		t.Fatalf("v1 pair=%+v/%v, want same nativeparent charge1984 and remaining64", got, err)
	}
}
