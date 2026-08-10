package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type stageDestinationMutation uint8

const (
	stageDestinationMutationNone stageDestinationMutation = iota
	stageDestinationMutationTruncate
	stageDestinationMutationAppend
	stageDestinationMutationChmod
	stageDestinationMutationClose
	stageDestinationMutationCancelFinish
)

type stageDestinationCase struct {
	wantErr     error
	name        string
	chunks      [][]byte
	wantExtent  uint64
	wantWritten uint64
	mutation    stageDestinationMutation
}

type stageDestinationIngressMutation uint8

const (
	stageDestinationIngressNilContext stageDestinationIngressMutation = iota
	stageDestinationIngressCanceledContext
	stageDestinationIngressZeroRequest
	stageDestinationIngressNilRoot
	stageDestinationIngressZeroPath
	stageDestinationIngressRootPath
	stageDestinationIngressZeroMode
	stageDestinationIngressTypeMode
	stageDestinationIngressExistingFile
	stageDestinationIngressExistingDirectory
	stageDestinationIngressAbsentParent
)

type stageDestinationIngressCase struct {
	wantErr  error
	name     string
	mutation stageDestinationIngressMutation
}

func TestOpenStageDestinationHostileIngressMatrix(t *testing.T) {
	t.Parallel()

	cases := []stageDestinationIngressCase{
		{name: "nil context", mutation: stageDestinationIngressNilContext, wantErr: core.ErrNilContext},
		{name: "canceled context", mutation: stageDestinationIngressCanceledContext, wantErr: context.Canceled},
		{name: "zero request", mutation: stageDestinationIngressZeroRequest, wantErr: core.ErrFilestoreContract},
		{name: "nil root", mutation: stageDestinationIngressNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "zero relative path", mutation: stageDestinationIngressZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "root entry path", mutation: stageDestinationIngressRootPath, wantErr: core.ErrFilestoreContract},
		{name: "zero permission mode", mutation: stageDestinationIngressZeroMode, wantErr: core.ErrFilestoreContract},
		{name: "filesystem type bits in permission mode", mutation: stageDestinationIngressTypeMode, wantErr: core.ErrFilestoreContract},
		{name: "existing regular file", mutation: stageDestinationIngressExistingFile, wantErr: core.ErrFilestoreConflict},
		{name: "existing directory", mutation: stageDestinationIngressExistingDirectory, wantErr: core.ErrFilestoreConflict},
		{name: "absent parent directory", mutation: stageDestinationIngressAbsentParent, wantErr: core.ErrFilestoreActivation},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			ctx := context.Context(t.Context())
			request := filestore.StageDestinationRequest{
				Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".ingress")},
				Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 0),
			}
			preexisting := false
			switch tc.mutation {
			case stageDestinationIngressNilContext:
				ctx = nil
			case stageDestinationIngressCanceledContext:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(t.Context())
				cancel()
			case stageDestinationIngressZeroRequest:
				request = filestore.StageDestinationRequest{}
			case stageDestinationIngressNilRoot:
				request.Temporary.Root = nil
			case stageDestinationIngressZeroPath:
				request.Temporary.Path = core.RelativePath{}
			case stageDestinationIngressRootPath:
				request.Temporary.Path = mustRelativePath(t, ".")
			case stageDestinationIngressZeroMode:
				request.Mode = 0
			case stageDestinationIngressTypeMode:
				request.Mode = fs.ModeDir | 0o600
			case stageDestinationIngressExistingFile:
				preexisting = true
				if err := os.WriteFile(filepath.Join(directory, ".ingress"), []byte{1}, 0o600); err != nil {
					t.Fatalf("WriteFile(preexisting file) error = %v, want nil", err)
				}
			case stageDestinationIngressExistingDirectory:
				preexisting = true
				if err := os.Mkdir(filepath.Join(directory, ".ingress"), 0o700); err != nil {
					t.Fatalf("Mkdir(preexisting directory) error = %v, want nil", err)
				}
			case stageDestinationIngressAbsentParent:
				request.Temporary.Path = mustRelativePath(t, "absent/.ingress")
			default:
				t.Fatalf("stage destination ingress mutation = %d, want a published test mutation", tc.mutation)
			}

			destination, gotErr := filestore.OpenStageDestination(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("OpenStageDestination() error = %v, want errors.Is %v", gotErr, tc.wantErr)
			}
			if destination != nil {
				t.Fatalf("OpenStageDestination() destination = non-nil, want nil")
			}
			if preexisting {
				return
			}
			if _, statErr := os.Stat(filepath.Join(directory, ".ingress")); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("Stat(rejected destination) error = %v, want errors.Is %v", statErr, fs.ErrNotExist)
			}
		})
	}
}

func TestStageDestinationHostileExtentAndFinalizationMatrix(t *testing.T) {
	t.Parallel()

	blockMinusOne := bytes.Repeat([]byte{0x41}, (32<<10)-1)
	block := bytes.Repeat([]byte{0x42}, 32<<10)
	blockPlusOne := bytes.Repeat([]byte{0x43}, (32<<10)+1)
	cases := []stageDestinationCase{
		{name: "neutral zero byte stream matches zero declaration"},
		{name: "one byte exactly at positive floor", chunks: [][]byte{{1}}, wantExtent: 1, wantWritten: 1},
		{name: "two bytes in one write", chunks: [][]byte{{1, 2}}, wantExtent: 2, wantWritten: 2},
		{name: "two bytes fragmented at every boundary", chunks: [][]byte{{1}, {2}}, wantExtent: 2, wantWritten: 2},
		{name: "empty writes surround exact bytes", chunks: [][]byte{nil, {1}, nil, {2}, nil}, wantExtent: 2, wantWritten: 2},
		{name: "stream buffer minus one exact", chunks: [][]byte{blockMinusOne}, wantExtent: uint64(len(blockMinusOne)), wantWritten: uint64(len(blockMinusOne))},
		{name: "stream buffer exact in two halves", chunks: [][]byte{block[:16<<10], block[16<<10:]}, wantExtent: uint64(len(block)), wantWritten: uint64(len(block))},
		{name: "stream buffer plus one exact", chunks: [][]byte{blockPlusOne}, wantExtent: uint64(len(blockPlusOne)), wantWritten: uint64(len(blockPlusOne))},
		{name: "sixty four kibibytes minus one fragmented", chunks: [][]byte{blockMinusOne, block}, wantExtent: (64 << 10) - 1, wantWritten: (64 << 10) - 1},
		{name: "three bytes across three writes", chunks: [][]byte{{1}, {2}, {3}}, wantExtent: 3, wantWritten: 3},
		{name: "declared zero receives one byte", chunks: [][]byte{{1}}, wantWritten: 1, wantErr: core.ErrFilestoreSize},
		{name: "declared one receives zero bytes", wantExtent: 1, wantErr: core.ErrFilestoreSize},
		{name: "declared one receives two bytes", chunks: [][]byte{{1, 2}}, wantExtent: 1, wantWritten: 2, wantErr: core.ErrFilestoreSize},
		{name: "declared two receives one byte", chunks: [][]byte{{1}}, wantExtent: 2, wantWritten: 1, wantErr: core.ErrFilestoreSize},
		{name: "declared two receives three fragmented bytes", chunks: [][]byte{{1}, {2, 3}}, wantExtent: 2, wantWritten: 3, wantErr: core.ErrFilestoreSize},
		{name: "stream buffer declaration receives one fewer", chunks: [][]byte{blockMinusOne}, wantExtent: uint64(len(block)), wantWritten: uint64(len(blockMinusOne)), wantErr: core.ErrFilestoreSize},
		{name: "stream buffer declaration receives one extra", chunks: [][]byte{blockPlusOne}, wantExtent: uint64(len(block)), wantWritten: uint64(len(blockPlusOne)), wantErr: core.ErrFilestoreSize},
		{name: "correct bytes truncated before finish", chunks: [][]byte{{1, 2}}, wantExtent: 2, mutation: stageDestinationMutationTruncate, wantWritten: 2, wantErr: core.ErrFilestoreSize},
		{name: "correct bytes appended before finish", chunks: [][]byte{{1, 2}}, wantExtent: 2, mutation: stageDestinationMutationAppend, wantWritten: 2, wantErr: core.ErrFilestoreSize},
		{name: "producer permission drift is restored before custody", chunks: [][]byte{{1, 2}}, wantExtent: 2, mutation: stageDestinationMutationChmod, wantWritten: 2},
		{name: "producer closes destination before finish", chunks: [][]byte{{1}}, wantExtent: 1, mutation: stageDestinationMutationClose, wantWritten: 1, wantErr: core.ErrFilestoreActivation},
		{name: "finish context canceled after exact bytes", chunks: [][]byte{{1}}, wantExtent: 1, mutation: stageDestinationMutationCancelFinish, wantWritten: 1, wantErr: context.Canceled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
				Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".matrix")},
				Mode:      0o600, ExpectedBytes: stageDestinationLength(t, tc.wantExtent),
			})
			if err != nil {
				t.Fatalf("OpenStageDestination() error = %v, want nil", err)
			}
			file, err := destination.File()
			if err != nil {
				t.Fatalf("StageDestination.File() error = %v, want nil", err)
			}
			var gotWritten uint64
			for _, chunk := range tc.chunks {
				written, writeErr := file.Write(chunk)
				gotWritten += uint64(written)
				if writeErr != nil {
					t.Fatalf("os.File.Write() error = %v after %d bytes, want nil", writeErr, gotWritten)
				}
			}
			if gotWritten != tc.wantWritten {
				t.Fatalf("os.File total written = %d, want %d", gotWritten, tc.wantWritten)
			}
			finishContext := t.Context()
			switch tc.mutation {
			case stageDestinationMutationNone:
			case stageDestinationMutationTruncate:
				if err := file.Truncate(1); err != nil {
					t.Fatalf("os.File.Truncate(1) error = %v, want nil", err)
				}
			case stageDestinationMutationAppend:
				if _, err := file.Write([]byte{3}); err != nil {
					t.Fatalf("os.File.Write(appended mutation) error = %v, want nil", err)
				}
			case stageDestinationMutationChmod:
				if err := file.Chmod(0o666); err != nil {
					t.Fatalf("os.File.Chmod(permission mutation) error = %v, want nil", err)
				}
			case stageDestinationMutationClose:
				if err := file.Close(); err != nil {
					t.Fatalf("os.File.Close() error = %v, want nil", err)
				}
			case stageDestinationMutationCancelFinish:
				var cancel context.CancelFunc
				finishContext, cancel = context.WithCancel(t.Context())
				cancel()
			default:
				t.Fatalf("stage destination mutation = %d, want a published test mutation", tc.mutation)
			}
			staged, gotErr := filestore.FinishStageDestination(finishContext, destination)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("FinishStageDestination() error = %v, want errors.Is %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if gotErr := staged.Validate(); !errors.Is(gotErr, core.ErrFilestoreContract) {
					t.Fatalf("refused StagedFile.Validate() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
				}
				if _, statErr := os.Stat(filepath.Join(directory, ".matrix")); !errors.Is(statErr, fs.ErrNotExist) {
					t.Fatalf("Stat(refused destination) error = %v, want errors.Is %v", statErr, fs.ErrNotExist)
				}
				return
			}
			wantBytes := bytes.Join(tc.chunks, nil)
			gotBytes, readErr := os.ReadFile(filepath.Join(directory, ".matrix"))
			if readErr != nil {
				t.Fatalf("ReadFile(finished destination) error = %v, want nil", readErr)
			}
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Fatalf("finished destination bytes = %d, want exact %d", len(gotBytes), len(wantBytes))
			}
			info, statErr := os.Stat(filepath.Join(directory, ".matrix"))
			if statErr != nil {
				t.Fatalf("Stat(finished destination) error = %v, want nil", statErr)
			}
			if gotMode := info.Mode().Perm(); gotMode != fs.FileMode(0o600) {
				t.Fatalf("finished destination mode = %#o, want %#o", gotMode, fs.FileMode(0o600))
			}
			if discardErr := filestore.Discard(t.Context(), staged); discardErr != nil {
				t.Fatalf("Discard(finished destination) error = %v, want nil", discardErr)
			}
		})
	}
}

func TestStageDestinationLinearOwnershipRefusesCopiesAndReuse(t *testing.T) {
	t.Parallel()

	t.Run("copied handle cannot disclose finish or abandon original file", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".copy")},
			Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 0),
		})
		if err != nil {
			t.Fatalf("OpenStageDestination() error = %v, want nil", err)
		}
		copied := *destination
		if _, gotErr := copied.File(); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("copied StageDestination.File() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if _, gotErr := filestore.FinishStageDestination(t.Context(), &copied); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("FinishStageDestination(copied) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if gotErr := filestore.AbandonStageDestination(&copied); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("AbandonStageDestination(copied) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if gotErr := filestore.AbandonStageDestination(destination); gotErr != nil {
			t.Fatalf("AbandonStageDestination(original) error = %v, want nil", gotErr)
		}
	})

	t.Run("finished handle cannot disclose finish or abandon transferred custody", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".finished")},
			Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 0),
		})
		if err != nil {
			t.Fatalf("OpenStageDestination() error = %v, want nil", err)
		}
		staged, err := filestore.FinishStageDestination(t.Context(), destination)
		if err != nil {
			t.Fatalf("FinishStageDestination() error = %v, want nil", err)
		}
		if _, gotErr := destination.File(); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("finished StageDestination.File() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if _, gotErr := filestore.FinishStageDestination(t.Context(), destination); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("FinishStageDestination(second) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if gotErr := filestore.AbandonStageDestination(destination); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("AbandonStageDestination(after finish) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if err := filestore.Discard(t.Context(), staged); err != nil {
			t.Fatalf("Discard() error = %v, want nil", err)
		}
	})

	t.Run("abandoned handle cannot disclose finish or abandon removed custody", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".abandoned")},
			Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 0),
		})
		if err != nil {
			t.Fatalf("OpenStageDestination() error = %v, want nil", err)
		}
		if err := filestore.AbandonStageDestination(destination); err != nil {
			t.Fatalf("AbandonStageDestination() error = %v, want nil", err)
		}
		if _, gotErr := destination.File(); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("abandoned StageDestination.File() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if _, gotErr := filestore.FinishStageDestination(t.Context(), destination); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("FinishStageDestination(after abandon) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
		if gotErr := filestore.AbandonStageDestination(destination); !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("AbandonStageDestination(second) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
		}
	})
}

func TestStageDestinationCommitRefusesPostFinishMutation(t *testing.T) {
	t.Parallel()

	t.Run("extent mutation after finish", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		staged := finishOneByteStageDestination(t, root, ".extent")
		file, err := root.OpenFile(".extent", os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatalf("os.Root.OpenFile(extent mutation) error = %v, want nil", err)
		}
		if _, err := file.Write([]byte{2}); err != nil {
			t.Fatalf("os.File.Write(extent mutation) error = %v, want nil", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("os.File.Close(extent mutation) error = %v, want nil", err)
		}
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: mustRelativePath(t, "extent"), Install: filestore.InstallCreate,
		})
		if !errors.Is(gotErr, core.ErrFilestoreSize) {
			t.Fatalf("Commit(extent mutation) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreSize)
		}
		if err := filestore.Discard(t.Context(), staged); err != nil {
			t.Fatalf("Discard(extent mutation) error = %v, want nil", err)
		}
	})

	t.Run("permission mutation after finish", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := requireTestRoot(t, directory)
		staged := finishOneByteStageDestination(t, root, ".mode")
		file, err := root.OpenFile(".mode", os.O_WRONLY, 0)
		if err != nil {
			t.Fatalf("os.Root.OpenFile(permission mutation) error = %v, want nil", err)
		}
		if err := file.Chmod(0o666); err != nil {
			t.Fatalf("os.File.Chmod(permission mutation) error = %v, want nil", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("os.File.Close(permission mutation) error = %v, want nil", err)
		}
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: mustRelativePath(t, "mode"), Install: filestore.InstallCreate,
		})
		if !errors.Is(gotErr, core.ErrFilestoreActivation) {
			t.Fatalf("Commit(permission mutation) error = %v, want errors.Is %v", gotErr, core.ErrFilestoreActivation)
		}
		if err := filestore.Discard(t.Context(), staged); err != nil {
			t.Fatalf("Discard(permission mutation) error = %v, want nil", err)
		}
	})
}

func finishOneByteStageDestination(t *testing.T, root *os.Root, name string) filestore.StagedFile {
	t.Helper()

	destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{
		Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, name)},
		Mode:      0o600, ExpectedBytes: stageDestinationLength(t, 1),
	})
	if err != nil {
		t.Fatalf("OpenStageDestination() error = %v, want nil", err)
	}
	file, err := destination.File()
	if err != nil {
		t.Fatalf("StageDestination.File() error = %v, want nil", err)
	}
	if gotWritten, gotErr := file.Write([]byte{1}); gotWritten != 1 || gotErr != nil {
		t.Fatalf("os.File.Write() = (%d, %v), want (1, nil)", gotWritten, gotErr)
	}
	staged, err := filestore.FinishStageDestination(t.Context(), destination)
	if err != nil {
		t.Fatalf("FinishStageDestination() error = %v, want nil", err)
	}
	return staged
}
