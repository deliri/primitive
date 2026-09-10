package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	filesystemConcurrencyProofTimeout = 10 * time.Minute
	filesystemCancellationTimeout     = time.Minute
)

type namespaceWriteObservation struct {
	receipt filestore.CommitRequest
	err     error
	index   int
}

// A ready barrier releases all owned goroutines together. The OS still owns
// their scheduling: this proves exact concurrent effects, not latency or that
// every syscall executes simultaneously. The 10,000-writer pressure is retained.
func TestNamespaceConcurrencyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		writers       int
		shared        bool
		empty         bool
		canceled      bool
		preexisting   bool
		wantSuccesses int
		wantFiles     int
	}{
		{name: "ten thousand independent binary writes retain every receipt and file", writers: 10_000, wantSuccesses: 10_000, wantFiles: 10_000},
		{name: "sixty four exclusive contenders produce exactly one complete winner", writers: 64, shared: true, wantSuccesses: 1, wantFiles: 1},
		{name: "empty sources still produce exactly their declared empty files", writers: 64, empty: true, wantSuccesses: 64, wantFiles: 64},
		{name: "canceled contenders create no temporary or target entries", writers: 64, shared: true, canceled: true},
		{name: "an existing empty target cannot be mistaken for absent", writers: 64, shared: true, preexisting: true, wantFiles: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			unrelated := []byte{31, 0, 255, 9}
			if err := os.WriteFile(directory+"/unrelated", unrelated, 0o600); err != nil {
				t.Fatal(err)
			}
			var previous os.FileInfo
			if tc.preexisting {
				if err := os.WriteFile(directory+"/shared", nil, 0o600); err != nil {
					t.Fatal(err)
				}
				var err error
				previous, err = root.Lstat("shared")
				if err != nil {
					t.Fatal(err)
				}
			}
			requests := make([]filestore.WriteRequest, tc.writers)
			payloads := make([][]byte, tc.writers)
			for index := range requests {
				target := fmt.Sprintf("target-%05d", index)
				if tc.shared {
					target = "shared"
				}
				payloads[index] = []byte{byte(index >> 8), byte(index), 0, 255}
				if tc.empty {
					payloads[index] = nil
				}
				requests[index] = filestore.WriteRequest{Source: bytes.NewReader(payloads[index]), Location: filestore.Location{Root: root, Path: mustRelativePath(t, target)}, Temporary: mustRelativePath(t, fmt.Sprintf("stage-%05d", index)), Mode: 0o600, Install: filestore.InstallCreate}
				if err := requests[index].Validate(); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := newFilesystemBackstop(t.Context(), t, filesystemConcurrencyProofTimeout)
			defer cancel()
			if tc.canceled {
				cancel()
			}
			start := make(chan struct{})
			results := make(chan namespaceWriteObservation, tc.writers)
			var ready, workers sync.WaitGroup
			ready.Add(tc.writers)
			for index := range requests {
				workers.Go(func() {
					ready.Done()
					<-start
					receipt, err := filestore.Write(ctx, requests[index])
					results <- namespaceWriteObservation{receipt: receipt, err: err, index: index}
				})
			}
			ready.Wait()
			close(start)
			done := make(chan struct{})
			go func() { workers.Wait(); close(results); close(done) }()
			// The test lifetime bounds completion even when the operation starts with
			// an already-canceled context. Cancellation is an input, not a timeout win.
			completion, stopCompletion := newFilesystemBackstop(t.Context(), t, filesystemConcurrencyProofTimeout)
			defer stopCompletion()
			select {
			case <-done:
			case <-completion.Done():
				cancel()
				cleanup, stopCleanup := newFilesystemBackstop(context.Background(), t, filesystemCancellationTimeout)
				defer stopCleanup()
				select {
				case <-done:
					t.Fatalf("workers required backstop cancellation: %v", completion.Err())
				case <-cleanup.Done():
					t.Fatalf("owned workers failed to terminate after cancellation: %v", cleanup.Err())
				}
			}
			seen := make([]bool, tc.writers)
			gotResults, gotSuccesses := 0, 0
			winner := -1
			for got := range results {
				gotResults++
				if got.index < 0 || got.index >= len(seen) || seen[got.index] {
					t.Fatalf("result index = %d, want exactly one result per writer", got.index)
				}
				seen[got.index] = true
				if got.receipt != (filestore.CommitRequest{}) {
					t.Errorf("writer %d receipt = %+v, want zero after settled success or refusal", got.index, got.receipt)
				}
				if got.err == nil {
					gotSuccesses++
					winner = got.index
					if tc.canceled || tc.preexisting {
						t.Errorf("writer %d error = nil, want refusal before replacing existing bytes", got.index)
					}
					continue
				}
				if tc.canceled {
					if !errors.Is(got.err, context.Canceled) {
						t.Errorf("writer %d = %v, want canceled", got.index, got.err)
					}
				} else if !tc.shared || !errors.Is(got.err, core.ErrFilestoreConflict) || !errors.Is(got.err, os.ErrExist) {
					t.Errorf("writer %d = %v, want exclusive-creation conflict only for shared targets", got.index, got.err)
				}
			}
			if gotResults != tc.writers || gotSuccesses != tc.wantSuccesses {
				t.Fatalf("results/successes = (%d,%d), want (%d,%d)", gotResults, gotSuccesses, tc.writers, tc.wantSuccesses)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != tc.wantFiles+1 {
				t.Fatalf("entries = (%v,%v), want %d targets plus neighbor", entries, err, tc.wantFiles)
			}
			for index := range tc.wantFiles {
				targetIndex := index
				if tc.shared {
					targetIndex = max(winner, 0)
				}
				path := requests[targetIndex].Location.Path.String()
				wantBytes := payloads[targetIndex]
				if tc.preexisting {
					wantBytes = nil
				}
				gotBytes, err := os.ReadFile(directory + "/" + path)
				if err != nil || !bytes.Equal(gotBytes, wantBytes) {
					t.Errorf("target %s = (%v,%v), want %v", path, gotBytes, err, wantBytes)
				}
				info, err := root.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != int64(len(wantBytes)) {
					t.Errorf("target %s metadata = %v, want exact regular extent and permissions", path, info)
				}
				if tc.preexisting && (!os.SameFile(previous, info) || previous.ModTime().UnixNano() != info.ModTime().UnixNano()) {
					t.Errorf("existing target = %v, want original empty inode %v", info, previous)
				}
			}
			gotNeighbor, err := os.ReadFile(directory + "/unrelated")
			if err != nil || !bytes.Equal(gotNeighbor, unrelated) {
				t.Errorf("neighbor = (%v,%v), want %v", gotNeighbor, err, unrelated)
			}
		})
	}
}
