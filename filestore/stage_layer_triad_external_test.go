package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Stage owns only its exclusive temporary. Every refusal must leave an exact
// zero receipt and preserve all pre-existing namespace owners.
func TestStagingEffectLayerTriad(t *testing.T) {
	t.Parallel()
	type fault uint8
	const (
		none fault = iota
		terminalSource
		occupiedFile
		occupiedDirectory
		missingParent
		closedRoot
		canceled
		nilContext
		nilSource
	)
	for _, tc := range []struct {
		name                string
		size, maximum       int
		fault               fault
		wantErr, wantNative error
	}{
		{name: "fragmented exact buffer crossing retains every byte", size: (32 << 10) + 3, maximum: (32 << 10) + 3},
		{name: "spare capacity cannot invent a suffix", size: (32 << 10) + 3, maximum: (32 << 10) + 4},
		{name: "one byte beyond capacity leaves no partial stage", size: (32 << 10) + 3, maximum: (32 << 10) + 2, wantErr: core.ErrFilestoreSize},
		{name: "empty input produces a real zero extent receipt", maximum: 1},
		{name: "terminal source error after two buffers cleans the entire stage", size: 1 << 16, maximum: (1 << 16) + 1, fault: terminalSource, wantErr: core.ErrFilestoreSource, wantNative: io.ErrUnexpectedEOF},
		{name: "occupied file is not truncated", size: 3, maximum: 3, fault: occupiedFile, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "occupied directory retains its child", size: 3, maximum: 3, fault: occupiedDirectory, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "absent parent is not created implicitly", size: 3, maximum: 3, fault: missingParent, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
		{name: "closed root refuses without consuming source", size: 3, maximum: 3, fault: closedRoot, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
		{name: "cancellation precedes exclusive creation", size: 3, maximum: 3, fault: canceled, wantErr: context.Canceled},
		{name: "nil context cannot acquire custody", size: 3, maximum: 3, fault: nilContext, wantErr: core.ErrNilContext},
		{name: "nil source cannot publish an empty receipt", maximum: 1, fault: nilSource, wantErr: core.ErrFilestoreContract},
		{name: "zero maximum refuses before source consumption", size: 3, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			neighbor := []byte{255, 0, 127, 1}
			if err := root.WriteFile("neighbor", neighbor, 0o600); err != nil {
				t.Fatal(err)
			}
			temporary := "stage"
			switch tc.fault {
			case occupiedFile:
				if err := root.WriteFile(temporary, neighbor, 0o600); err != nil {
					t.Fatal(err)
				}
			case occupiedDirectory:
				if err := root.Mkdir(temporary, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := root.WriteFile(filepath.Join(temporary, "child"), neighbor, 0o600); err != nil {
					t.Fatal(err)
				}
			case missingParent:
				temporary = filepath.Join("missing", temporary)
			}
			before, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			infos := make([]fs.FileInfo, len(before))
			for i, entry := range before {
				infos[i], err = os.Lstat(filepath.Join(directory, entry.name))
				if err != nil {
					t.Fatal(err)
				}
			}
			payload := deterministicPayload(tc.size)
			reader := bytes.NewReader(payload)
			var source io.Reader = iotest.HalfReader(reader)
			if tc.fault == terminalSource {
				source = io.MultiReader(source, iotest.ErrReader(io.ErrUnexpectedEOF))
			}
			if tc.fault == nilSource {
				source = nil
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.fault == canceled {
				cancel()
			}
			if tc.fault == nilContext {
				ctx = nil
			}
			if tc.fault == closedRoot {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var maximum core.ByteCount
			if tc.maximum != 0 {
				maximum = mustByteCount(t, uint64(tc.maximum))
			}
			path := mustRelativePath(t, temporary)
			got, gotErr := filestore.Stage(ctx, filestore.StageRequest{Source: source, Temporary: filestore.Location{Root: root, Path: path}, Mode: 0o600, MaximumBytes: maximum})
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("Stage = (%v,%v), want %v and native %v", got, gotErr, tc.wantErr, tc.wantNative)
			}
			for _, class := range []error{core.ErrFilestoreContract, core.ErrFilestoreActivation, core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreConflict, core.ErrFilestoreCleanup, core.ErrFilestoreSize, core.ErrFilestoreActivationIndeterminate} {
				if errors.Is(gotErr, class) != errors.Is(tc.wantErr, class) {
					t.Fatalf("Stage error = %v, want exactly %v", gotErr, tc.wantErr)
				}
			}
			if tc.wantErr != nil && got != (filestore.StagedFile{}) {
				t.Fatalf("refused receipt = %+v, want exact zero", got)
			}
			wantUnread := 0
			if tc.fault >= occupiedFile || tc.maximum == 0 {
				wantUnread = len(payload)
			}
			if reader.Len() != wantUnread {
				t.Fatalf("source unread = %d, want %d", reader.Len(), wantUnread)
			}
			after, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			wantEntries := len(before)
			if tc.wantErr == nil {
				wantEntries++
			}
			if len(after) != wantEntries {
				t.Fatalf("namespace = %v, want %d entries", after, wantEntries)
			}
			for i, entry := range before {
				index := slices.IndexFunc(after, func(candidate removalFixtureEntry) bool { return candidate.name == entry.name })
				if index < 0 || after[index].mode != entry.mode || after[index].target != entry.target || !bytes.Equal(after[index].data, entry.data) {
					t.Fatalf("retained entry = %v, want %+v", after, entry)
				}
				info, err := os.Lstat(filepath.Join(directory, entry.name))
				if err != nil || !os.SameFile(infos[i], info) || info.Mode() != infos[i].Mode() || info.Size() != infos[i].Size() || !info.ModTime().Equal(infos[i].ModTime()) {
					t.Fatalf("retained custody = (%v,%v), want %v", info, err, infos[i])
				}
			}
			if tc.wantErr == nil {
				info, err := os.Lstat(filepath.Join(directory, temporary))
				data, readErr := os.ReadFile(filepath.Join(directory, temporary))
				if got.Validate() != nil || got.Path() != path || got.BytesWritten().Uint64() != uint64(len(payload)) || err != nil || readErr != nil || !info.Mode().IsRegular() || info.Size() != int64(len(payload)) || !bytes.Equal(data, payload) {
					t.Fatalf("stage = (%v,%v,%v,%v), want exact path, extent, regular bytes", got, info, err, readErr)
				}
				native := filepath.Join(t.TempDir(), "mode")
				if err := os.WriteFile(native, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(native, 0o600); err != nil {
					t.Fatal(err)
				}
				nativeInfo, err := os.Stat(native)
				if err != nil || info.Mode().Perm() != nativeInfo.Mode().Perm() {
					t.Fatalf("stage mode = %v, want native (%v,%v)", info.Mode(), nativeInfo, err)
				}
			}
		})
	}
}
