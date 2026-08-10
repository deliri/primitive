package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type stagedReadMutation uint8

const (
	stagedReadMutationZeroReceipt stagedReadMutation = iota
	stagedReadMutationNilContext
	stagedReadMutationCanceledContext
	stagedReadMutationRemove
	stagedReadMutationRename
	stagedReadMutationReplaceFile
	stagedReadMutationTruncate
	stagedReadMutationAppend
	stagedReadMutationChmod
	stagedReadMutationReplaceDirectory
)

type stagedReadMutationCase struct {
	wantErr  error
	name     string
	mutation stagedReadMutation
}

func TestOpenStagedReadHostileValidExtentMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		wantSize int
	}{
		{name: "empty stream"},
		{name: "one byte floor", wantSize: 1},
		{name: "two bytes", wantSize: 2},
		{name: "stream buffer minus one", wantSize: (32 << 10) - 1},
		{name: "stream buffer exact", wantSize: 32 << 10},
		{name: "stream buffer plus one", wantSize: (32 << 10) + 1},
		{name: "two buffers minus one", wantSize: (64 << 10) - 1},
		{name: "two buffers exact", wantSize: 64 << 10},
		{name: "two buffers plus one", wantSize: (64 << 10) + 1},
		{name: "three buffers plus tail", wantSize: (96 << 10) + 17},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := requireTestRoot(t, t.TempDir())
			payload := deterministicPayload(tc.wantSize)
			staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
				Source: bytes.NewReader(payload),
				Temporary: filestore.Location{
					Root: root, Path: mustRelativePath(t, ".valid-stage"),
				},
				Mode: 0o600, MaximumBytes: mustByteCount(t, uint64(max(tc.wantSize, 1))),
			})
			if err != nil {
				t.Fatalf("Stage() error = %v, want nil", err)
			}
			file, err := filestore.OpenStagedRead(t.Context(), staged)
			if err != nil {
				t.Fatalf("OpenStagedRead() error = %v, want nil", err)
			}
			got, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("io.ReadAll() error = %v, want nil", err)
			}
			if err := file.Close(); err != nil {
				t.Fatalf("os.File.Close() error = %v, want nil", err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatalf("opened staged bytes = %d, want exact %d", len(got), len(payload))
			}
			if err := filestore.Discard(t.Context(), staged); err != nil {
				t.Fatalf("Discard() error = %v, want nil", err)
			}
		})
	}
}

func TestOpenStagedReadHostileRefusalMatrix(t *testing.T) {
	t.Parallel()

	cases := []stagedReadMutationCase{
		{name: "zero receipt", mutation: stagedReadMutationZeroReceipt, wantErr: core.ErrFilestoreContract},
		{name: "nil context", mutation: stagedReadMutationNilContext, wantErr: core.ErrNilContext},
		{name: "canceled context", mutation: stagedReadMutationCanceledContext, wantErr: context.Canceled},
		{name: "removed custody name", mutation: stagedReadMutationRemove, wantErr: core.ErrFilestoreActivation},
		{name: "renamed custody name", mutation: stagedReadMutationRename, wantErr: core.ErrFilestoreActivation},
		{name: "same-size replacement file", mutation: stagedReadMutationReplaceFile, wantErr: core.ErrFilestoreActivationIndeterminate},
		{name: "truncated staged inode", mutation: stagedReadMutationTruncate, wantErr: core.ErrFilestoreSize},
		{name: "grown staged inode", mutation: stagedReadMutationAppend, wantErr: core.ErrFilestoreSize},
		{name: "permission-mutated staged inode", mutation: stagedReadMutationChmod, wantErr: core.ErrFilestoreActivation},
		{name: "replacement directory", mutation: stagedReadMutationReplaceDirectory, wantErr: core.ErrFilestoreActivationIndeterminate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := requireTestRoot(t, t.TempDir())
			staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
				Source: bytes.NewReader([]byte{1, 2}),
				Temporary: filestore.Location{
					Root: root, Path: mustRelativePath(t, ".hostile-stage"),
				},
				Mode: 0o600, MaximumBytes: mustByteCount(t, 2),
			})
			if err != nil {
				t.Fatalf("Stage() error = %v, want nil", err)
			}
			ctx := context.Context(t.Context())
			candidate := staged
			switch tc.mutation {
			case stagedReadMutationZeroReceipt:
				candidate = filestore.StagedFile{}
			case stagedReadMutationNilContext:
				ctx = nil
			case stagedReadMutationCanceledContext:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(t.Context())
				cancel()
			case stagedReadMutationRemove:
				if err := root.Remove(".hostile-stage"); err != nil {
					t.Fatalf("os.Root.Remove() error = %v, want nil", err)
				}
			case stagedReadMutationRename:
				if err := root.Rename(".hostile-stage", ".renamed-stage"); err != nil {
					t.Fatalf("os.Root.Rename() error = %v, want nil", err)
				}
			case stagedReadMutationReplaceFile:
				replaceStagedReadName(t, root, false)
			case stagedReadMutationTruncate:
				mutateStagedReadFile(t, root, stagedReadMutationTruncate)
			case stagedReadMutationAppend:
				mutateStagedReadFile(t, root, stagedReadMutationAppend)
			case stagedReadMutationChmod:
				mutateStagedReadFile(t, root, stagedReadMutationChmod)
			case stagedReadMutationReplaceDirectory:
				replaceStagedReadName(t, root, true)
			default:
				t.Fatalf("staged read mutation = %d, want a published test mutation", tc.mutation)
			}
			file, gotErr := filestore.OpenStagedRead(ctx, candidate)
			if !errors.Is(gotErr, tc.wantErr) || file != nil {
				t.Fatalf("OpenStagedRead(hostile mutation) = (%v, %v), want nil and errors.Is %v",
					file, gotErr, tc.wantErr)
			}
		})
	}
}

func replaceStagedReadName(t *testing.T, root *os.Root, directory bool) {
	t.Helper()

	if err := root.Rename(".hostile-stage", ".original-stage"); err != nil {
		t.Fatalf("os.Root.Rename(original) error = %v, want nil", err)
	}
	if directory {
		if err := root.Mkdir(".hostile-stage", 0o700); err != nil {
			t.Fatalf("os.Root.Mkdir(replacement) error = %v, want nil", err)
		}
		return
	}
	replacement, err := root.OpenFile(".hostile-stage", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatalf("os.Root.OpenFile(replacement) error = %v, want nil", err)
	}
	if _, err := replacement.Write([]byte{3, 4}); err != nil {
		t.Fatalf("replacement.Write() error = %v, want nil", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("replacement.Close() error = %v, want nil", err)
	}
}

func mutateStagedReadFile(t *testing.T, root *os.Root, mutation stagedReadMutation) {
	t.Helper()

	file, err := root.OpenFile(".hostile-stage", os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("os.Root.OpenFile(mutation) error = %v, want nil", err)
	}
	switch mutation {
	case stagedReadMutationTruncate:
		err = file.Truncate(1)
	case stagedReadMutationAppend:
		_, err = file.Write([]byte{3})
	case stagedReadMutationChmod:
		err = file.Chmod(0o666)
	default:
		t.Fatalf("file mutation = %d, want truncate append or chmod", mutation)
	}
	if err != nil {
		t.Fatalf("staged file mutation error = %v, want nil", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("mutated file Close() error = %v, want nil", err)
	}
}
