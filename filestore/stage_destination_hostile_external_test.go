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
	stageDestinationMutationNilContext
)

type stageDestinationCase struct {
	wantErr     error
	wantNative  error
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
	stageDestinationIngressDanglingLink
	stageDestinationIngressOutsideLink
	stageDestinationIngressClosedRoot
)

func TestOpenStageDestinationHostileIngressMatrix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		mutation            stageDestinationIngressMutation
		wantErr, wantNative error
	}{
		{name: "nil context cannot create custody", mutation: stageDestinationIngressNilContext, wantErr: core.ErrNilContext},
		{name: "canceled context cannot create custody", mutation: stageDestinationIngressCanceledContext, wantErr: context.Canceled},
		{name: "zero request cannot invent root or path", mutation: stageDestinationIngressZeroRequest, wantErr: core.ErrFilestoreContract},
		{name: "missing root cannot escape into process cwd", mutation: stageDestinationIngressNilRoot, wantErr: core.ErrFilestoreContract},
		{name: "zero path cannot become a default temporary", mutation: stageDestinationIngressZeroPath, wantErr: core.ErrFilestoreContract},
		{name: "root name cannot become a mutable file", mutation: stageDestinationIngressRootPath, wantErr: core.ErrFilestoreContract},
		{name: "zero permissions cannot silently choose a default", mutation: stageDestinationIngressZeroMode, wantErr: core.ErrFilestoreContract},
		{name: "file type bits cannot be permission intent", mutation: stageDestinationIngressTypeMode, wantErr: core.ErrFilestoreContract},
		{name: "existing binary file cannot be truncated", mutation: stageDestinationIngressExistingFile, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing directory cannot lose its child", mutation: stageDestinationIngressExistingDirectory, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "missing parent cannot be synthesized", mutation: stageDestinationIngressAbsentParent, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
		{name: "dangling symlink remains an occupied name", mutation: stageDestinationIngressDanglingLink, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "outside symlink cannot create or truncate referent", mutation: stageDestinationIngressOutsideLink, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "closed rooted capability retains native refusal", mutation: stageDestinationIngressClosedRoot, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			ctx := context.Context(t.Context())
			request := filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600, ExpectedBytes: stageDestinationLength(t, 0)}
			payload := []byte{0, 255, 7}
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), payload, 0o600); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			outsidePath := filepath.Join(outside, "outside")
			if err := os.WriteFile(outsidePath, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			outsideInfo, err := os.Lstat(outsidePath)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.mutation {
			case stageDestinationIngressNilContext:
				ctx = nil
			case stageDestinationIngressCanceledContext:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
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
				if err := os.WriteFile(filepath.Join(directory, "stage"), payload, 0o640); err != nil {
					t.Fatal(err)
				}
			case stageDestinationIngressExistingDirectory:
				if err := root.Mkdir("stage", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "stage", "child"), payload, 0o600); err != nil {
					t.Fatal(err)
				}
			case stageDestinationIngressAbsentParent:
				request.Temporary.Path = mustRelativePath(t, "missing/stage")
			case stageDestinationIngressDanglingLink:
				if err := root.Symlink("absent", "stage"); err != nil {
					t.Fatal(err)
				}
			case stageDestinationIngressOutsideLink:
				if err := root.Symlink(outsidePath, "stage"); err != nil {
					t.Fatal(err)
				}
			case stageDestinationIngressClosedRoot:
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("ingress mutation = %d, want declared case", tc.mutation)
			}
			want, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			original, originalErr := os.Lstat(filepath.Join(directory, "stage"))
			if originalErr != nil && !errors.Is(originalErr, fs.ErrNotExist) {
				t.Fatal(originalErr)
			}
			got, gotErr := filestore.OpenStageDestination(ctx, request)
			if got != nil {
				t.Cleanup(func() { _ = filestore.AbandonStageDestination(got) })
			}
			if got != nil || !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("OpenStageDestination = (%v,%v), want nil and %v with native %v", got, gotErr, tc.wantErr, tc.wantNative)
			}
			gotNamespace, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(gotNamespace) != len(want) {
				t.Fatalf("namespace = %+v, want unchanged %+v", gotNamespace, want)
			}
			for i, got := range gotNamespace {
				if got.name != want[i].name || got.mode != want[i].mode || got.target != want[i].target || !bytes.Equal(got.data, want[i].data) {
					t.Fatalf("entry %d = %+v, want %+v", i, got, want[i])
				}
			}
			if original != nil {
				retained, err := os.Lstat(filepath.Join(directory, "stage"))
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(original, retained) || !original.ModTime().Equal(retained.ModTime()) {
					t.Fatalf("occupied inode = %v, want unchanged %v", retained, original)
				}
			}
			gotOutside, err := os.ReadFile(outsidePath)
			if err != nil || !bytes.Equal(gotOutside, payload) {
				t.Fatalf("outside bytes = (%v,%v), want %v", gotOutside, err, payload)
			}
			retained, err := os.Lstat(outsidePath)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(retained, outsideInfo) || retained.Mode() != outsideInfo.Mode() || !retained.ModTime().Equal(outsideInfo.ModTime()) {
				t.Fatalf("outside metadata = %v, want unchanged %v", retained, outsideInfo)
			}
		})
	}
}

func TestStageDestinationHostileExtentAndFinalizationMatrix(t *testing.T) {
	t.Parallel()

	cases := []stageDestinationCase{
		{name: "neutral zero byte stream matches zero declaration"},
		{name: "one byte exactly at positive floor", chunks: [][]byte{{1}}, wantExtent: 1, wantWritten: 1},
		{name: "empty writes cannot erase fragmented binary bytes", chunks: [][]byte{nil, {0}, nil, {255}, nil}, wantExtent: 2, wantWritten: 2},
		{name: "declared zero receives one byte", chunks: [][]byte{{1}}, wantWritten: 1, wantErr: core.ErrFilestoreSize},
		{name: "declared one receives zero bytes", wantExtent: 1, wantErr: core.ErrFilestoreSize},
		{name: "declared one receives two bytes", chunks: [][]byte{{1, 2}}, wantExtent: 1, wantWritten: 2, wantErr: core.ErrFilestoreSize},
		{name: "correct bytes truncated before finish", chunks: [][]byte{{1, 2}}, wantExtent: 2, mutation: stageDestinationMutationTruncate, wantWritten: 2, wantErr: core.ErrFilestoreSize},
		{name: "correct bytes appended before finish", chunks: [][]byte{{1, 2}}, wantExtent: 2, mutation: stageDestinationMutationAppend, wantWritten: 2, wantErr: core.ErrFilestoreSize},
		{name: "producer permission drift is restored before custody", chunks: [][]byte{{1, 2}}, wantExtent: 2, mutation: stageDestinationMutationChmod, wantWritten: 2},
		{name: "producer closes destination before finish", chunks: [][]byte{{1}}, wantExtent: 1, mutation: stageDestinationMutationClose, wantWritten: 1, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
		{name: "finish context canceled after exact bytes", chunks: [][]byte{{1}}, wantExtent: 1, mutation: stageDestinationMutationCancelFinish, wantWritten: 1, wantErr: context.Canceled},
		{name: "nil finish context cannot leak acquired custody", chunks: [][]byte{{0, 255}}, wantExtent: 2, mutation: stageDestinationMutationNilContext, wantWritten: 2, wantErr: core.ErrNilContext},
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
			case stageDestinationMutationNilContext:
				finishContext = nil
			case stageDestinationMutationCancelFinish:
				var cancel context.CancelFunc
				finishContext, cancel = context.WithCancel(t.Context())
				cancel()
			default:
				t.Fatalf("stage destination mutation = %d, want a published test mutation", tc.mutation)
			}
			staged, gotErr := filestore.FinishStageDestination(finishContext, destination)
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("FinishStageDestination() error = %v, want errors.Is %v", gotErr, tc.wantErr)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("settled file = %v, want closed", err)
			}
			gotFile, fileErr := destination.File()
			if gotFile != nil || !errors.Is(fileErr, core.ErrFilestoreContract) {
				t.Fatalf("settled handle = (%v,%v), want no capability", gotFile, fileErr)
			}
			if tc.wantErr != nil {
				if staged != (filestore.StagedFile{}) {
					t.Fatalf("refused receipt = %+v, want zero", staged)
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

type stageOwnershipFixture uint8

const (
	stageOwnershipNil stageOwnershipFixture = iota
	stageOwnershipZero
	stageOwnershipCopiedLive
	stageOwnershipCopiedFinished
	stageOwnershipFinished
	stageOwnershipAbandoned
)

func TestStageDestinationLinearOwnershipRefusesCopiesAndReuse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                        string
		fixture                     stageOwnershipFixture
		cancel                      bool
		wantErr                     error
		wantOriginalLive, wantStage bool
	}{
		{name: "nil handle cannot mint a file or receipt", fixture: stageOwnershipNil, wantErr: core.ErrFilestoreContract},
		{name: "zero handle cannot settle unrelated namespace", fixture: stageOwnershipZero, wantErr: core.ErrFilestoreContract},
		{name: "copy cannot acquire or settle original live custody", fixture: stageOwnershipCopiedLive, wantErr: core.ErrFilestoreContract, wantOriginalLive: true, wantStage: true},
		{name: "canceled finish on a copy cannot close the original", fixture: stageOwnershipCopiedLive, cancel: true, wantErr: core.ErrFilestoreContract, wantOriginalLive: true, wantStage: true},
		{name: "copy retained before finish cannot reclaim transferred custody", fixture: stageOwnershipCopiedFinished, wantErr: core.ErrFilestoreContract, wantStage: true},
		{name: "finished handle cannot reclaim its durable receipt", fixture: stageOwnershipFinished, wantErr: core.ErrFilestoreContract, wantStage: true},
		{name: "canceled repeated finish cannot discard transferred custody", fixture: stageOwnershipFinished, cancel: true, wantErr: core.ErrFilestoreContract, wantStage: true},
		{name: "abandoned handle cannot invent a removed entry", fixture: stageOwnershipAbandoned, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255, 7}
			neighbor := []byte{255, 0, 3}
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
			var input, original *filestore.StageDestination
			var file *os.File
			var before fs.FileInfo
			var receipt filestore.StagedFile
			switch tc.fixture {
			case stageOwnershipNil:
			case stageOwnershipZero:
				input = &filestore.StageDestination{}
			default:
				var err error
				original, err = filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600, ExpectedBytes: stageDestinationLength(t, uint64(len(payload)))})
				if err != nil {
					t.Fatal(err)
				}
				file, err = original.File()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = file.Close() })
				if n, err := file.Write(payload); err != nil || n != len(payload) {
					t.Fatalf("fixture write = (%d,%v), want %d", n, err, len(payload))
				}
				before, err = file.Stat()
				if err != nil {
					t.Fatal(err)
				}
				copied := *original
				input = original
				if tc.fixture == stageOwnershipCopiedLive || tc.fixture == stageOwnershipCopiedFinished {
					input = &copied
				}
				if tc.fixture == stageOwnershipFinished || tc.fixture == stageOwnershipCopiedFinished {
					receipt, err = filestore.FinishStageDestination(t.Context(), original)
					if err != nil {
						t.Fatal(err)
					}
				}
				if tc.fixture == stageOwnershipAbandoned {
					if err := filestore.AbandonStageDestination(original); err != nil {
						t.Fatal(err)
					}
				}
			}
			ctx := t.Context()
			if tc.cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			validation := input.Validate()
			gotFile, fileErr := input.File()
			gotReceipt, finishErr := filestore.FinishStageDestination(ctx, input)
			abandonErr := filestore.AbandonStageDestination(input)
			if !errors.Is(validation, tc.wantErr) || !errors.Is(fileErr, tc.wantErr) || !errors.Is(finishErr, tc.wantErr) || !errors.Is(abandonErr, tc.wantErr) || gotFile != nil || gotReceipt != (filestore.StagedFile{}) {
				t.Fatalf("invalid custody = (%v,%v,%v,%v,%v,%+v), want four typed refusals and zero capabilities", validation, fileErr, finishErr, abandonErr, gotFile, gotReceipt)
			}
			if errors.Is(finishErr, context.Canceled) != tc.cancel {
				t.Fatalf("finish cancellation = %v, want cancellation retained %t", finishErr, tc.cancel)
			}
			if file != nil {
				info, err := file.Stat()
				if tc.wantOriginalLive {
					gotOriginal, gotErr := original.File()
					if err != nil || gotErr != nil || gotOriginal != file || !os.SameFile(info, before) || info.Size() != int64(len(payload)) {
						t.Fatalf("original custody = (%v,%v,%v,%v), want live exact handle", info, err, gotOriginal, gotErr)
					}
				} else if !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("settled original = %v, want native closed", err)
				}
			}
			wantNames := 1
			if tc.wantStage {
				wantNames++
				info, err := root.Lstat("stage")
				if err != nil {
					t.Fatal(err)
				}
				gotBytes, err := os.ReadFile(filepath.Join(directory, "stage"))
				if err != nil || !bytes.Equal(gotBytes, payload) || !os.SameFile(info, before) || info.Mode() != before.Mode() || !info.ModTime().Equal(before.ModTime()) {
					t.Fatalf("retained stage = (%v,%v,%v), want exact original bytes and metadata", info, gotBytes, err)
				}
			} else if _, err := root.Lstat("stage"); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("absent stage = %v, want native not exist", err)
			}
			gotNeighbor, err := os.ReadFile(filepath.Join(directory, "neighbor"))
			if err != nil || !bytes.Equal(gotNeighbor, neighbor) {
				t.Fatalf("neighbor = (%v,%v), want %v", gotNeighbor, err, neighbor)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != wantNames {
				t.Fatalf("namespace = (%v,%v), want %d entries", entries, err, wantNames)
			}
			if tc.wantOriginalLive {
				if err := filestore.AbandonStageDestination(original); err != nil {
					t.Fatal(err)
				}
			}
			if receipt != (filestore.StagedFile{}) {
				if err := filestore.Discard(t.Context(), receipt); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

type settledStageMutation uint8

const (
	settledStageUnchanged settledStageMutation = iota
	settledStageTruncated
	settledStageAppended
	settledStagePermission
	settledStageForeign
	settledStageSymlink
	settledStageMissing
)

func TestStageDestinationCommitRefusesPostFinishMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		mutation            settledStageMutation
		wantErr, wantNative error
		wantForeign         bool
	}{
		{name: "exact finished receipt activates original inode"},
		{name: "one byte below finished extent cannot activate", mutation: settledStageTruncated, wantErr: core.ErrFilestoreSize},
		{name: "one byte above finished extent cannot activate", mutation: settledStageAppended, wantErr: core.ErrFilestoreSize},
		{name: "changed permissions cannot borrow original agreement", mutation: settledStagePermission, wantErr: core.ErrFilestoreActivation},
		{name: "same-byte foreign inode cannot borrow finished receipt", mutation: settledStageForeign, wantErr: core.ErrFilestoreActivationIndeterminate, wantForeign: true},
		{name: "symlink to original cannot borrow finished receipt", mutation: settledStageSymlink, wantErr: core.ErrFilestoreActivationIndeterminate, wantForeign: true},
		{name: "missing finished entry cannot fabricate activation", mutation: settledStageMissing, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			payload := []byte{0, 255}
			neighbor := []byte{31, 0, 255}
			if err := os.WriteFile(filepath.Join(directory, "neighbor"), neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
			plan := filestore.ActivationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Target: mustRelativePath(t, "target"), ExpectedBytes: stageDestinationLength(t, uint64(len(payload))), Mode: 0o600, Install: filestore.InstallCreate}
			destination, err := filestore.OpenStageDestination(t.Context(), plan.StageDestination())
			if err != nil {
				t.Fatal(err)
			}
			file, err := destination.File()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = file.Close() })
			if n, err := file.Write(payload); err != nil || n != len(payload) {
				t.Fatalf("native write = (%d,%v), want %d", n, err, len(payload))
			}
			original, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			staged, err := filestore.FinishStageDestination(t.Context(), destination)
			if err != nil || staged.Validate() != nil || staged.BytesWritten() != plan.ExpectedBytes {
				t.Fatalf("producer = (%+v,%v), want exact validated receipt", staged, err)
			}
			request, err := plan.CommitRequest(staged)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.mutation {
			case settledStageUnchanged:
			case settledStageTruncated, settledStageAppended:
				changed, err := root.OpenFile("stage", os.O_WRONLY, 0)
				if err != nil {
					t.Fatal(err)
				}
				size := int64(len(payload) - 1)
				if tc.mutation == settledStageAppended {
					size = int64(len(payload) + 1)
				}
				changeErr := changed.Truncate(size)
				closeErr := changed.Close()
				if changeErr != nil || closeErr != nil {
					t.Fatalf("native extent mutation = (%v,%v), want nil", changeErr, closeErr)
				}
			case settledStagePermission:
				if err := root.Chmod("stage", 0o640); err != nil {
					t.Fatal(err)
				}
			case settledStageForeign, settledStageSymlink, settledStageMissing:
				if err := root.Rename("stage", "archive"); err != nil {
					t.Fatal(err)
				}
				if tc.mutation == settledStageForeign {
					if err := os.WriteFile(filepath.Join(directory, "stage"), payload, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if tc.mutation == settledStageSymlink {
					if err := root.Symlink("archive", "stage"); err != nil {
						t.Fatal(err)
					}
				}
			default:
				t.Fatalf("mutation = %d, want declared case", tc.mutation)
			}
			want, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			current, currentErr := root.Lstat("stage")
			if tc.mutation == settledStageMissing {
				if !errors.Is(currentErr, fs.ErrNotExist) {
					t.Fatalf("missing mutation = %v, want native absence", currentErr)
				}
			} else {
				if currentErr != nil {
					t.Fatal(currentErr)
				}
				changed := !os.SameFile(original, current) || current.Mode() != original.Mode() || current.Size() != original.Size()
				if changed != (tc.mutation != settledStageUnchanged) {
					t.Fatalf("fixture changed agreement = %t, want %t", changed, tc.mutation != settledStageUnchanged)
				}
			}
			gotErr := filestore.Commit(t.Context(), request)
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("Commit = %v, want %v with native %v", gotErr, tc.wantErr, tc.wantNative)
			}
			if tc.wantErr == nil {
				for i := range want {
					if want[i].name == "stage" {
						want[i].name = "target"
					}
				}
				installed, err := root.Lstat("target")
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(original, installed) || !original.ModTime().Equal(installed.ModTime()) {
					t.Fatalf("activated inode = %v, want original %v", installed, original)
				}
			} else if current != nil {
				retained, err := root.Lstat("stage")
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(current, retained) || !current.ModTime().Equal(retained.ModTime()) {
					t.Fatalf("refused entry = %v, want unchanged %v", retained, current)
				}
			}
			gotNamespace, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(gotNamespace) != len(want) {
				t.Fatalf("namespace = %+v, want %+v", gotNamespace, want)
			}
			for i, got := range gotNamespace {
				if got.name != want[i].name || got.mode != want[i].mode || got.target != want[i].target || !bytes.Equal(got.data, want[i].data) {
					t.Fatalf("entry %d = %+v, want %+v", i, got, want[i])
				}
			}
			if tc.wantErr != nil {
				discardErr := filestore.Discard(t.Context(), staged)
				if tc.wantForeign {
					if !errors.Is(discardErr, core.ErrFilestoreCleanup) || !errors.Is(discardErr, core.ErrFilestoreConflict) {
						t.Fatalf("foreign discard = %v, want cleanup/conflict", discardErr)
					}
				} else if discardErr != nil {
					t.Fatalf("owned discard = %v, want nil", discardErr)
				}
			}
		})
	}
}
