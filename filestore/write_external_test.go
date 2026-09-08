package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// These are resolved one-shot outcomes. Interrupted receipt custody is pressured
// by the real identity-swap recovery fuzz target and stage namespace tables.
func TestDurableWriterLayerTriadCreateReplaceAndNeutralEffects(t *testing.T) {
	t.Parallel()
	type fault uint8
	const (
		none fault = iota
		closedSource
		nilSource
		closedRoot
		canceled
		nilContext
		temporaryFile
		temporaryDirectory
		targetDirectory
		targetNonemptyDirectory
		missingParent
	)
	binary := []byte{0, 255, 7, 1}
	original := []byte{19, 0, 255, 88, 5}
	for _, tc := range []struct {
		name                string
		fault               fault
		payload             []byte
		initial             []byte
		occupied            bool
		install             filestore.InstallMode
		maximum             uint64
		want                []byte
		wantErr, wantNative error
	}{
		{name: "exact ceiling creates opaque binary content", payload: binary, install: filestore.InstallCreate, maximum: 4, want: binary},
		{name: "spare ceiling cannot fabricate bytes", payload: binary, install: filestore.InstallCreate, maximum: 5, want: binary},
		{name: "empty create publishes a real empty file", install: filestore.InstallCreate, maximum: 1},
		{name: "replace of absent target still publishes", payload: binary, install: filestore.InstallReplace, maximum: 4, want: binary},
		{name: "replace cannot leave an old suffix", payload: binary, occupied: true, initial: original, install: filestore.InstallReplace, maximum: 4, want: binary},
		{name: "empty replacement consumes the whole prior extent", occupied: true, initial: original, install: filestore.InstallReplace, maximum: 1},
		{name: "create conflict preserves occupied binary bytes", payload: binary, occupied: true, initial: original, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "existing empty file is occupied rather than absent", payload: binary, occupied: true, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "overflow cannot replace existing content", payload: binary, occupied: true, initial: original, install: filestore.InstallReplace, maximum: 3, wantErr: core.ErrFilestoreSize},
		{name: "overflow cannot publish an absent target", payload: binary, install: filestore.InstallCreate, maximum: 3, wantErr: core.ErrFilestoreSize},
		{name: "closed Go source leaves no empty publication", fault: closedSource, payload: binary, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrClosed},
		{name: "nil source refuses before staging", fault: nilSource, install: filestore.InstallCreate, maximum: 1, wantErr: core.ErrFilestoreContract},
		{name: "closed root retains native activation refusal", fault: closedRoot, payload: binary, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrClosed},
		{name: "cancellation preserves an existing target", fault: canceled, payload: binary, occupied: true, initial: original, install: filestore.InstallReplace, maximum: 4, wantErr: context.Canceled},
		{name: "nil context cannot acquire stage custody", fault: nilContext, payload: binary, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrNilContext},
		{name: "occupied temporary cannot be truncated", fault: temporaryFile, payload: binary, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "temporary directory retains its child", fault: temporaryDirectory, payload: binary, install: filestore.InstallCreate, maximum: 4, wantErr: core.ErrFilestoreConflict, wantNative: fs.ErrExist},
		{name: "empty target directory refuses and stage is cleaned", fault: targetDirectory, payload: binary, install: filestore.InstallReplace, maximum: 4, wantErr: core.ErrFilestoreActivation},
		{name: "nonempty target directory retains its child", fault: targetNonemptyDirectory, payload: binary, install: filestore.InstallReplace, maximum: 4, wantErr: core.ErrFilestoreActivation},
		{name: "missing shared parent refuses before staging", fault: missingParent, payload: binary, install: filestore.InstallReplace, maximum: 4, wantErr: core.ErrFilestoreActivation, wantNative: fs.ErrNotExist},
		{name: "unset install cannot choose replacement", payload: binary, occupied: true, initial: original, maximum: 4, wantErr: core.ErrFilestoreContract},
		{name: "future install cannot choose replacement", payload: binary, occupied: true, initial: original, install: filestore.InstallMode(255), maximum: 4, wantErr: core.ErrFilestoreContract},
		{name: "zero ceiling refuses before consuming bytes", payload: binary, install: filestore.InstallCreate, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			if err := root.WriteFile("neighbor", original, 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.occupied {
				if err := root.WriteFile("target", tc.initial, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			target := "target"
			temporary := "stage"
			switch tc.fault {
			case temporaryFile:
				if err := root.WriteFile("stage", original, 0o600); err != nil {
					t.Fatal(err)
				}
			case temporaryDirectory:
				if err := root.Mkdir("stage", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := root.WriteFile("stage/child", original, 0o600); err != nil {
					t.Fatal(err)
				}
			case targetDirectory, targetNonemptyDirectory:
				if err := root.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if tc.fault == targetNonemptyDirectory {
					if err := root.WriteFile("target/child", original, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			case missingParent:
				target = filepath.Join("missing", "target")
				temporary = filepath.Join("missing", "stage")
			}
			wantNative := tc.wantNative
			if tc.fault == targetDirectory || tc.fault == targetNonemptyDirectory {
				if err := root.WriteFile("native-stage", binary, 0o600); err != nil {
					t.Fatal(err)
				}
				nativeErr := root.Rename("native-stage", target)
				var native *os.LinkError
				if !errors.As(nativeErr, &native) {
					t.Fatalf("native refusal = %v, want LinkError", nativeErr)
				}
				wantNative = native.Err
				if err := root.Remove("native-stage"); err != nil {
					t.Fatal(err)
				}
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
			reader := bytes.NewReader(tc.payload)
			var source io.Reader = reader
			if tc.fault == closedSource {
				file, err := os.Open(filepath.Join(directory, "neighbor"))
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				source = file
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
				maximum = mustByteCount(t, tc.maximum)
			}
			request := filestore.WriteRequest{Source: source, Location: filestore.Location{Root: root, Path: mustRelativePath(t, target)}, Temporary: mustRelativePath(t, temporary), Mode: 0o600, Install: tc.install, MaximumBytes: maximum}
			gotRecovery, gotErr := filestore.Write(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) || wantNative != nil && !errors.Is(gotErr, wantNative) || gotRecovery != (filestore.CommitRequest{}) {
				t.Fatalf("Write = (%v,%v), want zero recovery and %v/native %v", gotRecovery, gotErr, tc.wantErr, wantNative)
			}
			for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize, core.ErrFilestoreActivationIndeterminate} {
				if errors.Is(gotErr, class) != (class == tc.wantErr) {
					t.Fatalf("Write error classes = %v, want exactly %v", gotErr, tc.wantErr)
				}
			}
			if tc.wantErr == nil || errors.Is(tc.wantErr, core.ErrFilestoreConflict) && tc.fault == none {
				if reader.Len() != 0 {
					t.Fatalf("remaining source = %d, want complete consumption before activation", reader.Len())
				}
			}
			if tc.fault == missingParent || tc.fault == temporaryFile || tc.fault == temporaryDirectory || tc.fault == closedRoot || tc.fault == canceled || tc.fault == nilContext || tc.maximum == 0 || !tc.install.IsValid() {
				if reader.Len() != len(tc.payload) {
					t.Fatalf("refused source remaining = %d, want %d untouched", reader.Len(), len(tc.payload))
				}
			}
			after, err := removalFixtureSnapshot(directory)
			if err != nil {
				t.Fatal(err)
			}
			wantEntries := len(before)
			if tc.wantErr == nil && !tc.occupied {
				wantEntries++
			}
			if len(after) != wantEntries {
				t.Fatalf("namespace = %v, want %d exact entries", after, wantEntries)
			}
			for i, entry := range before {
				info, err := os.Lstat(filepath.Join(directory, entry.name))
				if err != nil {
					t.Fatal(err)
				}
				if tc.wantErr == nil && entry.name == target {
					continue
				}
				if !os.SameFile(infos[i], info) || info.Mode() != infos[i].Mode() || info.Size() != infos[i].Size() || !info.ModTime().Equal(infos[i].ModTime()) {
					t.Fatalf("entry %s custody = %v, want %v", entry.name, info, infos[i])
				}
				index := -1
				for j, candidate := range after {
					if candidate.name == entry.name {
						index = j
						break
					}
				}
				if index < 0 || after[index].mode != entry.mode || after[index].target != entry.target || !bytes.Equal(after[index].data, entry.data) {
					t.Fatalf("entry %s changed, want %+v", entry.name, entry)
				}
			}
			if tc.wantErr == nil {
				data, err := os.ReadFile(filepath.Join(directory, target))
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Lstat(filepath.Join(directory, target))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, tc.want) || !info.Mode().IsRegular() || info.Size() != int64(len(tc.want)) {
					t.Fatalf("published file = (%v,%v), want exact %v", data, info, tc.want)
				}
			}
			// A resolved zero handoff must itself refuse recovery, with no namespace
			// effect. This catches residual valid fields hidden behind a failed Validate.
			recoverErr := filestore.Recover(t.Context(), gotRecovery)
			if !errors.Is(recoverErr, core.ErrFilestoreContract) || errors.Is(recoverErr, core.ErrFilestoreActivation) {
				t.Fatalf("resolved recovery = %v, want pure contract refusal", recoverErr)
			}
			final, err := removalFixtureSnapshot(directory)
			if err != nil || len(final) != len(after) {
				t.Fatalf("zero recovery namespace = (%v,%v), want %v", final, err, after)
			}
			for i, entry := range after {
				if final[i].name != entry.name || final[i].mode != entry.mode || final[i].target != entry.target || !bytes.Equal(final[i].data, entry.data) {
					t.Fatalf("zero recovery changed entry %d", i)
				}
			}
		})
	}
}
