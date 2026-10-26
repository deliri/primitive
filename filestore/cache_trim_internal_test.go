package filestore

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The walk and the eviction are separated by the external sort, so a
// concurrent Go build can remove an observed entry in between. Eviction of a
// real record must then settle as evicted bytes without a removal claim.
func TestEvictCacheTrimRecordSettlesVanishedAndRefusedEntries(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name         string
		vanish, busy bool
		malformed    bool
		want         error
		wantCause    error
		wantObserved CacheTrimObservation
	}{
		{name: "positive present entry is removed", wantObserved: CacheTrimObservation{AfterBytes: 3, RemovedFiles: 1}},
		{name: "neutral vanished entry is evicted without a removal claim", vanish: true, wantObserved: CacheTrimObservation{AfterBytes: 3}},
		{name: "negative non-empty directory replacement preserves native refusal", busy: true, want: core.ErrFilestoreCleanup, wantCause: syscall.ENOTEMPTY, wantObserved: CacheTrimObservation{AfterBytes: 10}},
		{name: "negative malformed record is refused", malformed: true, want: core.ErrFilestoreContract, wantObserved: CacheTrimObservation{AfterBytes: 10}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "entry")
			if err := os.WriteFile(name, make([]byte, 7), 0o600); err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(name)
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("entry")
			if err != nil {
				t.Fatal(err)
			}
			record, err := appendCacheTrimRecord(nil, path, info, 7)
			if err != nil {
				t.Fatal(err)
			}
			if row.vanish || row.busy {
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			}
			if row.busy {
				if err := os.MkdirAll(filepath.Join(name, "child"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if row.malformed {
				record = record[1:]
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			observation := CacheTrimObservation{AfterBytes: 10}
			err = evictCacheTrimRecord(root, record, &observation)
			if !errors.Is(err, row.want) || (row.wantCause != nil && !errors.Is(err, row.wantCause)) {
				t.Fatalf("evict = %v, want %v and %v", err, row.want, row.wantCause)
			}
			if observation != row.wantObserved {
				t.Fatalf("observation = %+v, want %+v", observation, row.wantObserved)
			}
			_, err = os.Lstat(name)
			if present := err == nil; present != (row.busy || row.malformed) {
				t.Fatalf("entry present = %v (%v), want %v", present, err, row.busy || row.malformed)
			}
		})
	}
}
