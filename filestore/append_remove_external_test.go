package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestOpenAppendReturnsTheRealOSFile(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	request := filestore.AppendRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
		Mode:     0o600,
		Append:   filestore.AppendCreateOrOpen,
	}
	for _, line := range []string{"first\n", "second\n"} {
		file, openErr := filestore.OpenAppend(t.Context(), request)
		if openErr != nil {
			t.Fatalf("OpenAppend() error = %v, want nil", openErr)
		}
		if _, writeErr := file.Write([]byte(line)); writeErr != nil {
			t.Fatalf("*os.File.Write() error = %v, want nil", writeErr)
		}
		if syncErr := file.Sync(); syncErr != nil {
			t.Fatalf("*os.File.Sync() error = %v, want nil", syncErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatalf("*os.File.Close() error = %v, want nil", closeErr)
		}
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "ledger"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first\nsecond\n" {
		t.Fatalf("ledger bytes = %q, want %q", got, "first\nsecond\n")
	}
}

func TestRotateAppendOwnsOnlyThePhysicalCutover(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger-0001")},
		Mode:     0o600,
		Append:   filestore.AppendCreate,
	})
	if err != nil {
		t.Fatalf("OpenAppend() error = %v, want nil", err)
	}
	if _, err := outgoing.Write([]byte("outgoing\n")); err != nil {
		t.Fatal(err)
	}
	incoming, err := filestore.RotateAppend(t.Context(), filestore.RotationRequest{
		Outgoing: outgoing,
		Incoming: filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger-0002")},
			Mode:     0o600,
			Append:   filestore.AppendCreate,
		},
	})
	if err != nil {
		t.Fatalf("RotateAppend() error = %v, want nil", err)
	}
	if _, err := outgoing.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("outgoing Write() error = %v, want %v", err, os.ErrClosed)
	}
	if _, err := incoming.Write([]byte("incoming\n")); err != nil {
		t.Fatal(err)
	}
	if err := incoming.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := incoming.Close(); err != nil {
		t.Fatal(err)
	}
	gotFirst, err := os.ReadFile(filepath.Join(rootDirectory, "ledger-0001"))
	if err != nil {
		t.Fatal(err)
	}
	gotSecond, err := os.ReadFile(filepath.Join(rootDirectory, "ledger-0002"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotFirst) != "outgoing\n" || string(gotSecond) != "incoming\n" {
		t.Fatalf("rotated bytes = (%q, %q), want (%q, %q)", gotFirst, gotSecond, "outgoing\n", "incoming\n")
	}
}

func TestRotateAppendNeverOverwritesAnIncomingGeneration(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	if err := os.WriteFile(filepath.Join(rootDirectory, "ledger-0002"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger-0001")},
		Mode:     0o600,
		Append:   filestore.AppendCreate,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, gotErr := filestore.RotateAppend(t.Context(), filestore.RotationRequest{
		Outgoing: outgoing,
		Incoming: filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger-0002")},
			Mode:     0o600,
			Append:   filestore.AppendCreate,
		},
	})
	if !errors.Is(gotErr, core.ErrFilestoreConflict) || !errors.Is(gotErr, os.ErrExist) {
		t.Fatalf("RotateAppend() error = %v, want %v and %v", gotErr, core.ErrFilestoreConflict, os.ErrExist)
	}
	if _, err := outgoing.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("outgoing Write() after failed rotation error = %v, want %v", err, os.ErrClosed)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "ledger-0002"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "existing" {
		t.Fatalf("incoming generation = %q, want %q", got, "existing")
	}
}

func TestTenThousandIndependentWritersHaveNoPrimitiveCoordinationBottleneck(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	const writers = 10_000
	requests := make([]filestore.WriteRequest, writers)
	for index := range writers {
		name := fmt.Sprintf("agent-%04d", index)
		data := []byte(name)
		requests[index] = filestore.WriteRequest{
			Source: bytes.NewReader(data),
			Location: filestore.Location{
				Root: root,
				Path: mustRelativePath(t, name),
			},
			Temporary:    mustRelativePath(t, "."+name+"-stage"),
			Mode:         0o600,
			Install:      filestore.InstallCreate,
			MaximumBytes: mustByteCount(t, uint64(len(data))),
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	var group sync.WaitGroup
	failures := make(chan error, writers)
	for index := range writers {
		group.Go(func() {
			_, writeErr := filestore.Write(ctx, requests[index])
			failures <- writeErr
		})
	}
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(failures)
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		cancel()
		select {
		case <-done:
			t.Fatalf("10,000 independent Write() calls required cancellation to terminate: %v", ctx.Err())
		case <-time.After(time.Minute):
			t.Fatalf("10,000 independent Write() calls did not exit within %s after cancellation: %v", time.Minute, ctx.Err())
		}
	}
	for gotErr := range failures {
		if gotErr != nil {
			t.Fatalf("independent Write() error = %v, want nil", gotErr)
		}
	}
	entries, err := os.ReadDir(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != writers {
		t.Fatalf("independent file count = %d, want %d", len(entries), writers)
	}
	for index := range writers {
		name := fmt.Sprintf("agent-%04d", index)
		got, readErr := os.ReadFile(filepath.Join(rootDirectory, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(got) != name {
			t.Fatalf("independent file %s bytes = %q, want %q", name, got, name)
		}
	}
}

func TestCreateOnlyWritersResolveRealNamespaceContentionWithoutPrimitiveLocks(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	const writers = 256
	requests := make([]filestore.WriteRequest, writers)
	payloads := make([][]byte, writers)
	for index := range writers {
		payloads[index] = []byte(fmt.Sprintf("writer-%04d-complete-payload", index))
		requests[index] = filestore.WriteRequest{
			Source: bytes.NewReader(payloads[index]),
			Location: filestore.Location{
				Root: root,
				Path: mustRelativePath(t, "shared-target"),
			},
			Temporary:    mustRelativePath(t, fmt.Sprintf(".shared-target-stage-%04d", index)),
			Mode:         0o600,
			Install:      filestore.InstallCreate,
			MaximumBytes: mustByteCount(t, uint64(len(payloads[index]))),
		}
	}
	type result struct {
		err   error
		index int
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	results := make(chan result, writers)
	var group sync.WaitGroup
	for index := range writers {
		group.Go(func() {
			_, writeErr := filestore.Write(ctx, requests[index])
			results <- result{
				index: index,
				err:   writeErr,
			}
		})
	}
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(results)
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		cancel()
		select {
		case <-done:
			t.Fatalf("contending create-only Write() calls required cancellation to terminate: %v", ctx.Err())
		case <-time.After(time.Minute):
			t.Fatalf("contending create-only Write() calls did not exit within %s after cancellation: %v", time.Minute, ctx.Err())
		}
	}
	successes := 0
	winner := -1
	for got := range results {
		if got.err == nil {
			successes++
			winner = got.index
			continue
		}
		if !errors.Is(got.err, core.ErrFilestoreConflict) ||
			!errors.Is(got.err, os.ErrExist) {
			t.Fatalf("contending Write(%d) error = %v, want %v and %v", got.index, got.err, core.ErrFilestoreConflict, os.ErrExist)
		}
	}
	if successes != 1 {
		t.Fatalf("contending create-only successes = %d, want 1", successes)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "shared-target"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payloads[winner]) {
		t.Fatalf("shared target = %q, want complete winner payload %q", got, payloads[winner])
	}
	entries, err := os.ReadDir(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "shared-target" {
		t.Fatalf("entries after contending create-only writes = %v, want only shared-target", entries)
	}
}

func TestRemoveSynchronizesOneNamedEntryWithoutRecursivePolicy(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	if err := os.WriteFile(filepath.Join(rootDirectory, "confirmed"), []byte("uploaded"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, "active"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := filestore.Remove(t.Context(), filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "confirmed")},
	}); err != nil {
		t.Fatalf("Remove() error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(rootDirectory, "confirmed")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(confirmed) error = %v, want %v", err, os.ErrNotExist)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "active"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("active bytes = %q, want %q", got, "keep")
	}
}

func TestNamespaceConcurrencyLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive distinct targets complete independently without shared coordination", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		const writers = 32
		requests := make([]filestore.WriteRequest, writers)
		for index := range writers {
			name := fmt.Sprintf("distinct-%02d", index)
			requests[index] = filestore.WriteRequest{
				Source:       bytes.NewReader([]byte(name)),
				Location:     filestore.Location{Root: root, Path: mustRelativePath(t, name)},
				Temporary:    mustRelativePath(t, "."+name+"-stage"),
				Mode:         0o600,
				Install:      filestore.InstallCreate,
				MaximumBytes: mustByteCount(t, uint64(len(name))),
			}
		}
		results := runConcurrentWriteRequests(t, requests)
		for _, got := range results {
			if got.err != nil {
				t.Fatalf("distinct Write(%d) error = %v, want nil", got.index, got.err)
			}
		}
		entries, err := os.ReadDir(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != writers {
			t.Fatalf("distinct target count = %d, want %d", len(entries), writers)
		}
	})
	t.Run("negative same-target contention lets exactly one namespace operation win", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		const writers = 32
		requests := make([]filestore.WriteRequest, writers)
		payloads := make([][]byte, writers)
		for index := range writers {
			payloads[index] = []byte(fmt.Sprintf("contender-%02d", index))
			requests[index] = filestore.WriteRequest{
				Source:       bytes.NewReader(payloads[index]),
				Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "shared")},
				Temporary:    mustRelativePath(t, fmt.Sprintf(".shared-stage-%02d", index)),
				Mode:         0o600,
				Install:      filestore.InstallCreate,
				MaximumBytes: mustByteCount(t, uint64(len(payloads[index]))),
			}
		}
		results := runConcurrentWriteRequests(t, requests)
		successes := 0
		winner := -1
		for _, got := range results {
			if got.err == nil {
				successes++
				winner = got.index
				continue
			}
			if !errors.Is(got.err, core.ErrFilestoreConflict) ||
				!errors.Is(got.err, os.ErrExist) {
				t.Fatalf("same-target Write(%d) error = %v, want %v and %v", got.index, got.err, core.ErrFilestoreConflict, os.ErrExist)
			}
		}
		if successes != 1 {
			t.Fatalf("same-target successes = %d, want 1", successes)
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, "shared"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, payloads[winner]) {
			t.Fatalf("same-target bytes = %q, want complete winner payload %q", got, payloads[winner])
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"shared"})
	})
	t.Run("neutral empty request set creates no workers and no namespace entries", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		results := runConcurrentWriteRequests(t, nil)
		if len(results) != 0 {
			t.Fatalf("empty concurrent results = %v, want none", results)
		}
		requireDirectoryEntryNames(t, rootDirectory, nil)
	})
}

type concurrentWriteResult struct {
	err   error
	index int
}

func runConcurrentWriteRequests(
	t *testing.T,
	requests []filestore.WriteRequest,
) []concurrentWriteResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	results := make(chan concurrentWriteResult, len(requests))
	var group sync.WaitGroup
	for index := range requests {
		group.Go(func() {
			_, writeErr := filestore.Write(ctx, requests[index])
			results <- concurrentWriteResult{err: writeErr, index: index}
		})
	}
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(results)
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		cancel()
		select {
		case <-done:
			t.Fatalf("concurrent Write() calls required cancellation to terminate: %v", ctx.Err())
		case <-time.After(time.Minute):
			t.Fatalf("concurrent Write() calls did not exit within %s after cancellation: %v", time.Minute, ctx.Err())
		}
	}
	got := make([]concurrentWriteResult, 0, len(requests))
	for result := range results {
		got = append(got, result)
	}
	return got
}
