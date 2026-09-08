package filestore

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The source returns its prefix in one completed Read, then either returns a
// native failure, panics with that same identity, or ends cleanly. No WriteTo
// shortcut can bypass the terminal call, including the one-byte ceiling probe.
type unwindStageSource struct {
	prefix         []byte
	terminal       error
	beforeTerminal func() error
	panicAtEnd     bool
	calls          int
}

func (s *unwindStageSource) Read(p []byte) (int, error) {
	s.calls++
	if len(s.prefix) > 0 {
		n := copy(p, s.prefix)
		s.prefix = s.prefix[n:]
		return n, nil
	}
	if s.beforeTerminal != nil {
		if err := s.beforeTerminal(); err != nil {
			return 0, err
		}
	}
	if s.panicAtEnd {
		panic(s.terminal)
	}
	if s.terminal != nil {
		return 0, s.terminal
	}
	return 0, io.EOF
}

func TestStageCallerUnwindCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		prefix        []byte
		maximum       uint64
		panicAtEnd    bool
		returnFailure bool
		replaceName   bool
		wantErr       error
		wantCalls     int
	}{
		{name: "empty completed source retains its real empty receipt", maximum: 1, wantCalls: 1},
		{name: "exact binary ceiling survives successful return", prefix: []byte{0, 255}, maximum: 2, wantCalls: 2},
		{name: "native refusal removes only its partial file", prefix: []byte{0}, maximum: 2, returnFailure: true, wantErr: core.ErrFilestoreSource, wantCalls: 2},
		{name: "panic before first byte cannot orphan a temporary", maximum: 2, panicAtEnd: true, wantCalls: 1},
		{name: "panic after acknowledged prefix cannot orphan partial bytes", prefix: []byte{0}, maximum: 2, panicAtEnd: true, wantCalls: 2},
		{name: "ceiling probe panic cannot orphan a complete but unsealed file", prefix: []byte{0, 255}, maximum: 2, panicAtEnd: true, wantCalls: 2},
		{name: "panic cleanup cannot remove a replacement inode", prefix: []byte{0, 255}, maximum: 2, panicAtEnd: true, replaceName: true, wantCalls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
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
			path, err := core.ParseRelativePath("stage")
			if err != nil {
				t.Fatal(err)
			}
			maximum, err := core.NewByteCount(tc.maximum)
			if err != nil {
				t.Fatal(err)
			}
			file, err := root.OpenFile(path.String(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			// Red-test cleanup closes the exact leaked handle. It does not turn that
			// cleanup into production evidence: the assertion below runs first.
			t.Cleanup(func() {
				if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
					t.Error(err)
				}
			})
			before, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			native := &fs.PathError{Op: "read", Path: "caller-source", Err: fs.ErrPermission}
			source := &unwindStageSource{prefix: bytes.Clone(tc.prefix), panicAtEnd: tc.panicAtEnd}
			if tc.panicAtEnd || tc.returnFailure {
				source.terminal = native
			}
			foreign := []byte{31, 0, 255, 9}
			var mutationErr error
			if tc.replaceName {
				source.beforeTerminal = func() error {
					// Retain the original inode so reuse cannot make a replacement appear owned.
					if err := root.Rename(path.String(), "retained-original"); err != nil {
						mutationErr = err
						return err
					}
					mutationErr = os.WriteFile(directory+"/"+path.String(), foreign, 0o600)
					return mutationErr
				}
			}
			request := StageRequest{Source: source, Temporary: Location{Root: root, Path: path}, Mode: 0o600, MaximumBytes: maximum}
			var got StagedFile
			var gotErr error
			var gotPanic any
			returned := false
			func() {
				defer func() { gotPanic = recover() }()
				got, gotErr = finishStage(t.Context(), request, file, before)
				returned = true
			}()
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			if source.calls != tc.wantCalls {
				t.Errorf("source calls = %d, want %d", source.calls, tc.wantCalls)
			}
			if tc.panicAtEnd {
				if gotPanic != native || returned || got != (StagedFile{}) || gotErr != nil {
					t.Errorf("unwind = (%v,%t,%+v,%v), want original panic and no returned receipt", gotPanic, returned, got, gotErr)
				}
			} else if gotPanic != nil || !returned || (gotErr == nil) != (tc.wantErr == nil) || tc.wantErr != nil && (!errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, native)) {
				t.Errorf("return = (%v,%t,%+v,%v), want returned %v with original native cause", gotPanic, returned, got, gotErr, tc.wantErr)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Errorf("owned file Stat = %v, want closed before control returns or unwinds", err)
			}
			if tc.panicAtEnd || tc.returnFailure {
				if got != (StagedFile{}) {
					t.Errorf("refused receipt = %+v, want zero", got)
				}
				wantEntries := 0
				if tc.replaceName {
					wantEntries = 2
					gotForeign, err := os.ReadFile(directory + "/" + path.String())
					if err != nil || !bytes.Equal(gotForeign, foreign) {
						t.Errorf("foreign bytes = (%v,%v), want %v", gotForeign, err, foreign)
					}
					retained, err := root.Lstat("retained-original")
					if err != nil {
						t.Fatal(err)
					}
					gotOriginal, err := os.ReadFile(directory + "/retained-original")
					if err != nil || !os.SameFile(before, retained) || !bytes.Equal(gotOriginal, tc.prefix) {
						t.Errorf("retained original = (%v,%v,%v), want original inode and %v", retained, gotOriginal, err, tc.prefix)
					}
				} else if _, err := root.Lstat(path.String()); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("abandoned path = %v, want absent", err)
				}
				entries, err := os.ReadDir(directory)
				if err != nil || len(entries) != wantEntries {
					t.Errorf("entries = (%v,%v), want %d", entries, err, wantEntries)
				}
				return
			}
			gotBytes, err := os.ReadFile(directory + "/" + path.String())
			if err != nil || !bytes.Equal(gotBytes, tc.prefix) || got.Validate() != nil || got.BytesWritten().Uint64() != uint64(len(tc.prefix)) {
				t.Fatalf("completed stage = (%+v,%v,%v), want exact %v", got, gotBytes, err, tc.prefix)
			}
			if err := Discard(t.Context(), got); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("discarded entries = (%v,%v), want empty", entries, err)
			}
		})
	}
}
