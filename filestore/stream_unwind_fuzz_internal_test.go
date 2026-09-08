package filestore

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type unwindIngress uint8

const (
	unwindStage unwindIngress = iota
	unwindWrite
	unwindRead
	unwindIngressLimit
)

// Public execution, exact effects, retained neighboring bytes, and returned
// versus unwound receipts are checked together. A panic payload is never an
// ordinary returned native error. The internal triads separately prove closure
// of the precise handles owned by these public calls.
func FuzzStreamUnwindSemanticCustody(f *testing.F) {
	emitted, err := FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(unwindStage), emitted, uint16(len(emitted)), false, false)
	for _, seed := range []struct {
		door           unwindIngress
		payload        []byte
		maximum        uint16
		panics, effect bool
	}{
		{door: unwindStage, maximum: 1, panics: true},
		{door: unwindStage, payload: []byte{0, 255}, maximum: 2, panics: true},
		{door: unwindStage, payload: []byte{0, 255}, maximum: 1, panics: true},
		{door: unwindStage, maximum: 1},
		{door: unwindWrite, payload: []byte{0, 255}, maximum: 2, panics: true},
		{door: unwindWrite, payload: []byte{0, 255}, maximum: 2},
		{door: unwindRead, maximum: 1, panics: true},
		{door: unwindRead, payload: []byte{0, 255}, maximum: 2, panics: true},
		{door: unwindRead, payload: []byte{0, 255}, maximum: 2, panics: true, effect: true},
		{door: unwindRead, payload: []byte{0, 255}, maximum: 1},
	} {
		f.Add(uint8(seed.door), seed.payload, seed.maximum, seed.panics, seed.effect)
	}
	f.Fuzz(func(t *testing.T, rawDoor uint8, payload []byte, rawMaximum uint16, panics, effect bool) {
		door := unwindIngress(rawDoor) % unwindIngressLimit
		payload = payload[:min(len(payload), 4096)]
		ceiling := uint64(max(1, rawMaximum%4097))
		maximum, err := core.NewByteCount(ceiling)
		if err != nil {
			t.Fatal(err)
		}
		directory := t.TempDir()
		root, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := root.Close(); err != nil {
				t.Error(err)
			}
		})
		stagePath, err := core.ParseRelativePath("stage")
		if err != nil {
			t.Fatal(err)
		}
		targetPath, err := core.ParseRelativePath("target")
		if err != nil {
			t.Fatal(err)
		}
		unrelated := []byte{31, 0, 255, 9}
		if err := os.WriteFile(directory+"/unrelated", unrelated, 0o600); err != nil {
			t.Fatal(err)
		}
		wantTarget := unrelated
		if door == unwindRead {
			wantTarget = payload
		}
		if err := os.WriteFile(directory+"/target", wantTarget, 0o600); err != nil {
			t.Fatal(err)
		}
		before, err := root.Lstat(targetPath.String())
		if err != nil {
			t.Fatal(err)
		}
		native := &fs.PathError{Op: "stream", Path: "caller", Err: fs.ErrPermission}
		source := &unwindStageSource{prefix: bytes.Clone(payload), panicAtEnd: panics, terminal: nil}
		if panics {
			source.terminal = native
		}
		destination := &unwindReadDestination{panicAtWrite: panics, effectBeforePanic: effect, terminal: nil}
		if panics {
			destination.terminal = native
		}
		var gotStage StagedFile
		var gotCommit CommitRequest
		var gotCount core.ByteLength
		var gotErr error
		var gotPanic any
		returned := false
		func() {
			defer func() { gotPanic = recover() }()
			switch door {
			case unwindStage:
				gotStage, gotErr = Stage(t.Context(), StageRequest{Source: source, Temporary: Location{Root: root, Path: stagePath}, Mode: 0o600, MaximumBytes: maximum})
			case unwindWrite:
				gotCommit, gotErr = Write(t.Context(), WriteRequest{Source: source, Location: Location{Root: root, Path: targetPath}, Temporary: stagePath, Mode: 0o600, Install: InstallReplace, MaximumBytes: maximum})
			case unwindRead:
				gotCount, gotErr = Read(t.Context(), ReadRequest{Destination: destination, Location: Location{Root: root, Path: targetPath}, MaximumBytes: maximum})
			}
			returned = true
		}()
		wantPanic := panics && uint64(len(payload)) <= ceiling
		if door == unwindRead {
			wantPanic = panics && len(payload) > 0
		}
		var wantErr error
		if !wantPanic && uint64(len(payload)) > ceiling {
			wantErr = core.ErrFilestoreSize
		}
		if wantPanic {
			if gotPanic != native || returned || gotErr != nil || gotStage != (StagedFile{}) || gotCommit != (CommitRequest{}) || gotCount != (core.ByteLength{}) {
				t.Fatalf("unwind = (%v,%t,%v,%+v,%+v,%+v), want original panic without a returned receipt", gotPanic, returned, gotErr, gotStage, gotCommit, gotCount)
			}
		} else if gotPanic != nil || !returned || (gotErr == nil) != (wantErr == nil) || wantErr != nil && !errors.Is(gotErr, wantErr) {
			t.Fatalf("return = (%v,%t,%v), want returned %v", gotPanic, returned, gotErr, wantErr)
		}
		if door == unwindRead {
			wantBytes := payload[:min(uint64(len(payload)), ceiling)]
			if wantPanic && !effect {
				wantBytes = nil
			}
			if !bytes.Equal(destination.Bytes(), wantBytes) {
				t.Fatalf("physical destination = %v, want %v", destination.Bytes(), wantBytes)
			}
			if !wantPanic && gotCount.Uint64() != uint64(len(wantBytes)) {
				t.Fatalf("read count = %d, want %d", gotCount.Uint64(), len(wantBytes))
			}
		}
		if !wantPanic && wantErr == nil {
			switch door {
			case unwindStage:
				stagedBytes, err := os.ReadFile(directory + "/stage")
				if err != nil || !bytes.Equal(stagedBytes, payload) || gotStage.Validate() != nil || gotStage.BytesWritten().Uint64() != uint64(len(payload)) {
					t.Fatalf("stage = (%+v,%v,%v), want exact %v", gotStage, stagedBytes, err, payload)
				}
				if err := Discard(t.Context(), gotStage); err != nil {
					t.Fatal(err)
				}
			case unwindWrite:
				if gotCommit != (CommitRequest{}) {
					t.Fatalf("resolved commit = %+v, want zero", gotCommit)
				}
				wantTarget = payload
			}
		} else if gotStage != (StagedFile{}) || gotCommit != (CommitRequest{}) {
			t.Fatalf("refusal receipts = (%+v,%+v), want zero", gotStage, gotCommit)
		}
		retained, err := os.ReadFile(directory + "/target")
		if err != nil || !bytes.Equal(retained, wantTarget) {
			t.Fatalf("target bytes = (%v,%v), want %v", retained, err, wantTarget)
		}
		after, err := root.Lstat(targetPath.String())
		if err != nil {
			t.Fatal(err)
		}
		if door != unwindWrite || wantPanic || wantErr != nil {
			if !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("preserved target = %v, want original inode and metadata %v", after, before)
			}
		} else if !after.Mode().IsRegular() || after.Mode().Perm() != 0o600 || after.Size() != int64(len(payload)) {
			t.Fatalf("activated target = %v, want exact regular file", after)
		}
		retained, err = os.ReadFile(directory + "/unrelated")
		if err != nil || !bytes.Equal(retained, unrelated) {
			t.Fatalf("neighbor = (%v,%v), want %v", retained, err, unrelated)
		}
		if _, err := root.Lstat(stagePath.String()); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("temporary = %v, want absent after settlement", err)
		}
		entries, err := os.ReadDir(directory)
		if err != nil || len(entries) != 2 {
			t.Fatalf("entries = (%v,%v), want target and neighbor only", entries, err)
		}
	})
}
