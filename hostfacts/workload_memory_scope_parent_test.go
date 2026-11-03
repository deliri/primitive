package hostfacts

import (
	"context"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestWorkloadMemoryScopeParentUsesFutureAncestry(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, targetLimit, targetCurrent, rootLimit, rootCurrent string
		want                                                     uint64
		state                                                    WorkloadMemoryLimitState
	}{
		{name: "future parent is tighter than current worker session", targetLimit: "1024", targetCurrent: "960", rootLimit: "max", want: 64, state: WorkloadMemoryLimitLimited},
		{name: "enclosing ancestor is tighter than future parent", targetLimit: "1024", targetCurrent: "960", rootLimit: "128", rootCurrent: "80", want: 48, state: WorkloadMemoryLimitLimited},
		{name: "future parent exhausted", targetLimit: "1024", targetCurrent: "1024", rootLimit: "max", state: WorkloadMemoryLimitLimited},
		{name: "future parent above limit saturates", targetLimit: "1024", targetCurrent: "1025", rootLimit: "max", state: WorkloadMemoryLimitLimited},
		{name: "unlimited future parent retains common finite ancestor", targetLimit: "max", rootLimit: "128", rootCurrent: "80", want: 48, state: WorkloadMemoryLimitLimited},
		{name: "native unlimited interfaces stay unlimited", targetLimit: "max", rootLimit: "max", state: WorkloadMemoryLimitUnlimited},
		{name: "absent memory interfaces stay unavailable", state: WorkloadMemoryLimitUnavailable},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			files, mount := scopeParentFilesFixture(t)
			for _, file := range []struct{ path, value string }{
				{filepath.Join(mount, "manager", "app.slice", "memory.max"), row.targetLimit},
				{filepath.Join(mount, "manager", "app.slice", "memory.current"), row.targetCurrent},
				{filepath.Join(mount, "memory.max"), row.rootLimit},
				{filepath.Join(mount, "memory.current"), row.rootCurrent},
			} {
				if file.value != "" {
					if err := os.WriteFile(file.path, []byte(file.value+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := observeWorkloadMemoryScopeParentAt(t.Context(), files, []byte("/manager/app.slice\n"))
			remaining, present := got.RemainingBytes()
			if err != nil || got.Validate() != nil || got.BindingLimit().State() != row.state || remaining.Uint64() != row.want || present != (row.state == WorkloadMemoryLimitLimited) {
				t.Fatalf("future headroom=%+v/%v remaining=%d/%t, want state%s remaining%d", got, err, remaining.Uint64(), present, row.state, row.want)
			}
			if path, ok := got.BindingLimit().InterfacePath(); ok && strings.Contains(path.String(), "session.scope") {
				t.Fatalf("binding=%s, want the future branch or its enclosing ancestor", path)
			}
		})
	}
}

func TestWorkloadMemoryScopeParentRefusesUnknownOrMissingTarget(t *testing.T) {
	t.Parallel()
	for _, record := range []string{"", "\n", "/absent/app.slice\n"} {
		t.Run("record="+record, func(t *testing.T) {
			t.Parallel()
			files, mount := scopeParentFilesFixture(t)
			if err := os.WriteFile(filepath.Join(mount, "memory.max"), []byte("max\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := observeWorkloadMemoryScopeParentAt(t.Context(), files, []byte(record))
			if err != nil || got.Validate() != nil || got.BindingLimit().State() != WorkloadMemoryLimitUnavailable {
				t.Fatalf("unknown target=%+v/%v, want unavailable even with an unlimited visible root", got, err)
			}
		})
	}
}

func TestWorkloadMemoryScopeParentPreservesChargeAndContextRefusals(t *testing.T) {
	t.Parallel()
	files, mount := scopeParentFilesFixture(t)
	if err := os.WriteFile(filepath.Join(mount, "manager", "app.slice", "memory.max"), []byte("1024\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := observeWorkloadMemoryScopeParentAt(t.Context(), files, []byte("/manager/app.slice\n"))
	if !errors.Is(err, fs.ErrNotExist) || got != (WorkloadMemoryHeadroom{}) {
		t.Fatalf("missing charge=%+v/%v, want zero and native fs.ErrNotExist", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err = observeWorkloadMemoryScopeParentAt(ctx, files, []byte("/manager/app.slice\n"))
	if !errors.Is(err, context.Canceled) || got != (WorkloadMemoryHeadroom{}) {
		t.Fatalf("cancelled observation=%+v/%v, want zero and cancellation", got, err)
	}
}

func TestWorkloadMemoryScopeParentControlRecordAndIdentity(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"relative", "/manager/../app.slice", "/manager/app.slice\n/other", "/manager/app.slice\r\n", "/bad\x00name", string([]byte{'/', 255}), strings.Repeat("/x", procLineMaximumBytes)} {
		t.Run("bad record="+value[:min(len(value), 40)], func(t *testing.T) {
			t.Parallel()
			got, present, err := parseWorkloadMemoryScopeParent([]byte(value))
			if !errors.Is(err, core.ErrHostFactsObservation) || got != "" || present {
				t.Fatalf("record=%q/%t/%v, want zero and typed refusal", got, present, err)
			}
		})
	}
	for _, row := range []struct {
		before, after string
		changed       bool
	}{
		{before: "/manager/app.slice\n", after: "/manager/app.slice"},
		{before: "", after: "\n"},
		{before: "/manager/app.slice\n", after: "", changed: true},
		{before: "/manager/app.slice\n", after: "/other/app.slice\n", changed: true},
	} {
		err := sameWorkloadMemoryScopeParent([]byte(row.before), []byte(row.after))
		if (err != nil) != row.changed || err != nil && !errors.Is(err, core.ErrHostFactsObservation) {
			t.Fatalf("identity %q->%q=%v, want changed=%t", row.before, row.after, err, row.changed)
		}
	}
}

func FuzzWorkloadMemoryScopeParentRecord(f *testing.F) {
	for _, seed := range []string{"", "\n", "/", "/manager/app.slice\n", "/with space/app.slice", "relative", "/a/../b", "/a\x00b", "/a\n/b"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// A valid absolute unit path is constructed independently for every
		// mutation, so blanket refusal cannot satisfy the semantic oracle.
		want := "/user-" + hex.EncodeToString(data[:min(len(data), 64)]) + "/app.slice"
		generated, generatedPresent, generatedErr := parseWorkloadMemoryScopeParent([]byte(want + "\n"))
		if generatedErr != nil || !generatedPresent || generated != want {
			t.Fatalf("generated record=%q/%t/%v, want exact %q", generated, generatedPresent, generatedErr, want)
		}
		if _, _, badErr := parseWorkloadMemoryScopeParent([]byte(want + "\n/foreign")); !errors.Is(badErr, core.ErrHostFactsObservation) {
			t.Fatalf("multiple records=%v, want typed refusal", badErr)
		}
		value, present, err := parseWorkloadMemoryScopeParent(data)
		if err != nil {
			if !errors.Is(err, core.ErrHostFactsObservation) || value != "" || present {
				t.Fatalf("refusal=%q/%t/%v, want typed refusal without value", value, present, err)
			}
			return
		}
		if !present {
			if value != "" || len(data) > 1 || len(data) == 1 && data[0] != '\n' {
				t.Fatalf("absence=%q from%q, want empty or one final newline", value, data)
			}
			return
		}
		if value == "" || value[0] != '/' || strings.ContainsAny(value, "\r\n\x00") || value != string(data) && value+"\n" != string(data) {
			t.Fatalf("accepted record=%q from%q, want exact absolute single native record", value, data)
		}
		if err := sameWorkloadMemoryScopeParent([]byte(value), []byte(value+"\n")); err != nil {
			t.Fatalf("native newline equivalence=%v, want preserved identity", err)
		}
	})
}

func scopeParentFilesFixture(t *testing.T) (workloadMemoryScopeParentFiles, string) {
	t.Helper()
	root := t.TempDir()
	mount := filepath.Join(root, "cgroup")
	for _, directory := range []string{filepath.Join(mount, "session.scope"), filepath.Join(mount, "manager", "app.slice")} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	proc, mounts := filepath.Join(root, "membership"), filepath.Join(root, "mountinfo")
	for _, file := range []struct{ path, text string }{
		{proc, "0::/session.scope\n"},
		{mounts, "30 23 0:27 / " + mount + " rw - cgroup2 cgroup rw\n"},
		{filepath.Join(mount, "session.scope", "memory.max"), "2048\n"},
		{filepath.Join(mount, "session.scope", "memory.current"), "0\n"},
	} {
		if err := os.WriteFile(file.path, []byte(file.text), 0o600); err != nil {
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
	return workloadMemoryScopeParentFiles{membership: p, mounts: m}, mount
}
