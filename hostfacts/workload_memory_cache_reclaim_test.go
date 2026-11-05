package hostfacts

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestIdleCgroupCacheReclaimNativeFiles(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, events, stat        string
		maximum                   uint64
		wantWrite, missingControl bool
		want                      error
	}{
		{name: "empty exact inactive boundary", maximum: 64, wantWrite: true},
		{name: "empty ample inactive cache", maximum: 32, wantWrite: true},
		{name: "populated group stays untouched", events: "populated 1\nfrozen 0\n", maximum: 32},
		{name: "not enough inactive cache", maximum: 65},
		{name: "anonymous charge stays untouched", stat: "anon 1\nshmem 0\nfile_dirty 0\nfile_writeback 0\ninactive_file 64\n", maximum: 32},
		{name: "shared charge stays untouched", stat: "anon 0\nshmem 1\nfile_dirty 0\nfile_writeback 0\ninactive_file 64\n", maximum: 32},
		{name: "dirty charge stays untouched", stat: "anon 0\nshmem 0\nfile_dirty 1\nfile_writeback 0\ninactive_file 64\n", maximum: 32},
		{name: "writeback charge stays untouched", stat: "anon 0\nshmem 0\nfile_dirty 0\nfile_writeback 1\ninactive_file 64\n", maximum: 32},
		{name: "missing population refuses", events: "frozen 0\n", maximum: 32, want: core.ErrHostFactsObservation},
		{name: "duplicate population refuses", events: "populated 0\npopulated 0\n", maximum: 32, want: core.ErrHostFactsObservation},
		{name: "invalid population refuses", events: "populated 2\n", maximum: 32, want: core.ErrHostFactsObservation},
		{name: "missing charge refuses", stat: "anon 0\nshmem 0\nfile_dirty 0\nfile_writeback 0\n", maximum: 32, want: core.ErrHostFactsObservation},
		{name: "duplicate charge refuses", stat: "anon 0\nshmem 0\nfile_dirty 0\nfile_writeback 0\ninactive_file 64\ninactive_file 64\n", maximum: 32, want: core.ErrHostFactsObservation},
		{name: "missing control retains native cause", maximum: 32, missingControl: true, want: fs.ErrNotExist},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			events := row.events
			if events == "" {
				events = "populated 0\nfrozen 0\n"
			}
			stat := row.stat
			if stat == "" {
				stat = "anon 0\nshmem 0\nfile_dirty 0\nfile_writeback 0\ninactive_file 64\n"
			}
			for _, f := range []struct{ name, data string }{{"cgroup.events", events}, {"memory.stat", stat}} {
				if err := os.WriteFile(filepath.Join(directory, f.name), []byte(f.data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			control := filepath.Join(directory, "memory.reclaim")
			if !row.missingControl {
				if err := os.WriteFile(control, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			wrote, err := reclaimEmptyCgroupCache(t.Context(), root, row.maximum)
			if wrote != row.wantWrite || (row.want == nil && err != nil) || (row.want != nil && !errors.Is(err, row.want)) {
				t.Fatalf("reclaim=%t/%v, want %t/%v", wrote, err, row.wantWrite, row.want)
			}
			if !row.missingControl {
				data, err := os.ReadFile(control)
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				if row.wantWrite {
					want = strconv.FormatUint(row.maximum, 10) + " swappiness=0"
				}
				if string(data) != want {
					t.Fatalf("control=%q, want %q", data, want)
				}
			}
		})
	}
}

func TestIdleCgroupCacheReclaimWalksPastPopulatedAncestor(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "cgroup.events"), []byte("populated 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(directory, "idle")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct{ name, data string }{{"cgroup.events", "populated 0\n"}, {"memory.stat", "anon 0\nshmem 0\nfile_dirty 0\nfile_writeback 0\ninactive_file 64\n"}, {"memory.reclaim", ""}} {
		if err := os.WriteFile(filepath.Join(child, f.name), []byte(f.data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	wrote, err := reclaimEmptyCgroupCache(t.Context(), root, 32)
	if err != nil || !wrote {
		t.Fatalf("child reclaim=%t/%v, want true/nil", wrote, err)
	}
	data, err := os.ReadFile(filepath.Join(child, "memory.reclaim"))
	if err != nil || string(data) != "32 swappiness=0" {
		t.Fatalf("child control=%q/%v, want 32 swappiness=0", data, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "memory.reclaim")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("populated ancestor control=%v, want absent", err)
	}
}

func TestIdleCgroupCacheReclaimRetainsCancellationAndContracts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cause := errors.New("owned cancellation")
	cancel(cause)
	amount, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReclaimIdleWorkloadMemoryCache(ctx, amount); !errors.Is(err, cause) {
		t.Fatalf("cancellation=%v, want exact cause %v", err, cause)
	}
	if _, err := ReclaimIdleWorkloadMemoryCache(t.Context(), core.ByteLength{}); !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("zero=%v, want contract", err)
	}
	if _, err := ReclaimIdleWorkloadMemoryCache(nil, amount); !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context=%v, want nil-context cause", err)
	}
}

func FuzzCgroupCacheFieldCanonicalValue(f *testing.F) {
	for _, s := range []string{"0", "64", "18446744073709551615", "-1", "00", "+1", "18446744073709551616", "1\ninactive_file 2"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1024 || strings.ContainsAny(s, "\r\n\t ") {
			return
		}
		got, err := cgroupCacheField([]byte("inactive_file "+s+"\n"), "inactive_file")
		number, parseErr := strconv.ParseUint(s, 10, 64)
		canonical := parseErr == nil && strconv.FormatUint(number, 10) == s && !strings.ContainsAny(s, "\r\n\t ")
		if canonical && (err != nil || got != number) {
			t.Fatalf("value=%q got=%d/%v, want %d", s, got, err, number)
		}
		if !canonical && err == nil {
			t.Fatalf("value=%q got=%d, want refusal", s, got)
		}
	})
}

// One byte permits at most one native page request in the fleet's idle cache;
// this is not a grant or a claim that concurrently changing parent charge drops.
// The separate private-group fixture earns the actual charge-reduction claim.
func TestReclaimIdleWorkloadMemoryCacheNativeEntry(t *testing.T) {
	t.Parallel()
	before, err := ObserveWorkloadMemoryScopeParent(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	amount, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ReclaimIdleWorkloadMemoryCache(t.Context(), amount)
	if err != nil || after.Validate() != nil || after.BindingLimit().State() != before.BindingLimit().State() {
		t.Fatalf("native result=%+v/%v, want valid same availability as %+v", after, err, before)
	}
	beforePath, beforePresent := before.BindingLimit().InterfacePath()
	afterPath, afterPresent := after.BindingLimit().InterfacePath()
	beforeLimit, beforeFinite := before.BindingLimit().LimitBytes()
	afterLimit, afterFinite := after.BindingLimit().LimitBytes()
	if beforePath != afterPath || beforePresent != afterPresent || beforeLimit != afterLimit || beforeFinite != afterFinite {
		t.Fatalf("native binding=%+v->%+v, want unchanged native limit/path", before.BindingLimit(), after.BindingLimit())
	}
	if before.BindingLimit().State() == WorkloadMemoryLimitLimited {
		current, currentPresent := after.CurrentBytes()
		remaining, remainingPresent := after.RemainingBytes()
		if !currentPresent || !remainingPresent || remaining.Uint64() != afterLimit.Uint64()-min(afterLimit.Uint64(), current.Uint64()) {
			t.Fatalf("post charge=%d remaining=%d, want exact native saturating pair", current.Uint64(), remaining.Uint64())
		}
	}
}
