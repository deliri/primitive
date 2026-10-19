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

func TestAppendObservationOwnsRotationFailureAndExplicitClosure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		setup   func(*testing.T, string, *filestore.AppendPathRotationRequest) context.Context
		want    error
		custody filestore.AppendCustody
	}{
		{name: "successful path rotation holds incoming custody", custody: filestore.AppendCustodyHeld},
		{name: "occupied target settles outgoing and retains native conflict", want: core.ErrFilestoreConflict, custody: filestore.AppendCustodyReleased, setup: func(t *testing.T, dir string, request *filestore.AppendPathRotationRequest) context.Context {
			if err := os.WriteFile(filepath.Join(dir, "incoming"), []byte("occupant"), 0o600); err != nil {
				t.Fatal(err)
			}
			return t.Context()
		}},
		{name: "missing parent keeps outgoing and records acquisition failure", want: fs.ErrNotExist, custody: filestore.AppendCustodyHeld, setup: func(t *testing.T, dir string, request *filestore.AppendPathRotationRequest) context.Context {
			path, err := core.ParseAbsolutePath(filepath.Join(dir, "missing", "incoming"))
			if err != nil {
				t.Fatal(err)
			}
			request.Path = path
			return t.Context()
		}},
		{name: "cancellation retains standard and native causes without releasing custody", want: io.ErrUnexpectedEOF, custody: filestore.AppendCustodyHeld, setup: func(t *testing.T, _ string, _ *filestore.AppendPathRotationRequest) context.Context {
			ctx, cancel := context.WithCancelCause(t.Context())
			cancel(io.ErrUnexpectedEOF)
			return ctx
		}},
		{name: "invalid mode keeps custody and records contract refusal", want: core.ErrFilestoreContract, custody: filestore.AppendCustodyHeld, setup: func(t *testing.T, _ string, request *filestore.AppendPathRotationRequest) context.Context {
			request.Mode = fs.ModeDir
			return t.Context()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			destination, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "outgoing")}, Mode: 0o600, Append: filestore.AppendCreate})
			if closeErr := root.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := destination.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := destination.Write([]byte("outgoing")); err != nil {
				t.Fatal(err)
			}
			request := filestore.AppendPathRotationRequest{Path: mustAppendAbsolute(t, filepath.Join(dir, "incoming")), Mode: 0o600, Append: filestore.AppendCreate}
			ctx := t.Context()
			if tc.setup != nil {
				ctx = tc.setup(t, dir, &request)
			}
			err = destination.RotatePath(ctx, request)
			if !errors.Is(err, tc.want) {
				t.Fatalf("rotation = %v, want %v", err, tc.want)
			}
			observation, observeErr := destination.ObserveAppend()
			if observeErr != nil || observation.Custody != tc.custody || !errors.Is(observation.RotationError, tc.want) {
				t.Fatalf("native append observation = (%+v, %v), want custody %s and cause %v", observation, observeErr, tc.custody, tc.want)
			}
			if err := observation.Validate(); err != nil {
				t.Fatal(err)
			}
			if errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(observation.RotationError, context.Canceled) {
				t.Fatalf("rotation cause = %v, want standard cancellation too", observation.RotationError)
			}
			if tc.custody == filestore.AppendCustodyReleased {
				if _, err := destination.Write([]byte("zombie")); !errors.Is(err, core.ErrFilestoreContract) {
					t.Fatalf("settled write = %v, want contract refusal", err)
				}
			} else if _, err := destination.Write([]byte("tail")); err != nil {
				t.Fatal(err)
			}
			before, err := destination.AcceptedBytes()
			if err != nil {
				t.Fatal(err)
			}
			if err := destination.Close(); err != nil {
				t.Fatal(err)
			}
			observation, err = destination.ObserveAppend()
			if err != nil || observation.Custody != filestore.AppendCustodyReleased || observation.RotationError != nil {
				t.Fatalf("explicit closure observation = (%+v, %v), want released and no pending rotation failure", observation, err)
			}
			after, err := destination.AcceptedBytes()
			if err != nil || after != before {
				t.Fatalf("historical acceptance after closure = (%v,%v), want %v", after, err, before)
			}
			outgoing, err := os.ReadFile(filepath.Join(dir, "outgoing"))
			if err != nil {
				t.Fatal(err)
			}
			wantOutgoing := []byte("outgoing")
			if tc.want != nil && tc.custody == filestore.AppendCustodyHeld {
				wantOutgoing = append(wantOutgoing, []byte("tail")...)
			}
			if !bytes.Equal(outgoing, wantOutgoing) {
				t.Fatalf("outgoing bytes = %q, want %q", outgoing, wantOutgoing)
			}
			if tc.want == nil {
				incoming, err := os.ReadFile(filepath.Join(dir, "incoming"))
				if err != nil || string(incoming) != "tail" {
					t.Fatalf("incoming bytes = (%q,%v), want tail", incoming, err)
				}
			}
		})
	}
}

func mustAppendAbsolute(t testing.TB, text string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(text)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAppendObservationRefusesCopiedAndUnsetOwners(t *testing.T) {
	t.Parallel()
	for _, destination := range []*filestore.AppendDestination{nil, {}} {
		observation, err := destination.ObserveAppend()
		if !errors.Is(err, core.ErrFilestoreContract) || observation != (filestore.AppendObservation{}) {
			t.Fatalf("unset observation = (%+v,%v), want zero and refusal", observation, err)
		}
	}
	for _, custody := range []filestore.AppendCustody{filestore.AppendCustodyUnknown, 255} {
		if err := custody.Validate(); !errors.Is(err, core.ErrFilestoreContract) || custody.IsValid() {
			t.Fatalf("unknown custody = %s/%v, want invalid", custody, err)
		}
	}
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	destination, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "owned")}, Mode: 0o600, Append: filestore.AppendCreate})
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	defer root.Close()
	copied := *destination
	if observation, err := copied.ObserveAppend(); !errors.Is(err, core.ErrFilestoreContract) || observation != (filestore.AppendObservation{}) {
		t.Fatalf("copied observation = (%+v,%v), want zero and refusal", observation, err)
	}
	if err := copied.RotatePath(t.Context(), filestore.AppendPathRotationRequest{Path: mustAppendAbsolute(t, filepath.Join(dir, "foreign")), Mode: 0o600, Append: filestore.AppendCreate}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("copied rotation = %v, want refusal", err)
	}
	if observation, err := destination.ObserveAppend(); err != nil || observation.Custody != filestore.AppendCustodyHeld || observation.RotationError != nil {
		t.Fatalf("original after copied request = (%+v,%v), want untouched held custody", observation, err)
	}
	if err := destination.Rotate(t.Context(), filestore.AppendRequest{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("rooted invalid rotation = %v, want refusal", err)
	}
	if observation, err := destination.ObserveAppend(); err != nil || observation.Custody != filestore.AppendCustodyHeld || !errors.Is(observation.RotationError, core.ErrFilestoreContract) {
		t.Fatalf("rooted rotation observation = (%+v,%v), want actual refusal with custody held", observation, err)
	}
}

func FuzzAppendRotationObservationMatchesNativeFiles(f *testing.F) {
	for _, selector := range []uint8{0, 1, 2, 3} {
		f.Add(selector, []byte("native outgoing seed"))
	}
	f.Add(uint8(0), []byte{})
	f.Fuzz(func(t *testing.T, selector uint8, payload []byte) {
		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()
		destination, err := filestore.OpenAppendDestination(t.Context(), filestore.AppendRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "outgoing")}, Mode: 0o600, Append: filestore.AppendCreate})
		if err != nil {
			t.Fatal(err)
		}
		defer destination.Close()
		if _, err := destination.Write(payload); err != nil {
			t.Fatal(err)
		}
		request := filestore.AppendPathRotationRequest{Path: mustAppendAbsolute(t, filepath.Join(dir, "incoming")), Mode: 0o600, Append: filestore.AppendCreate}
		ctx := t.Context()
		custody, want := filestore.AppendCustodyHeld, error(nil)
		switch selector % 4 {
		case 0:
		case 1:
			if err := os.WriteFile(filepath.Join(dir, "incoming"), []byte("occupant"), 0o600); err != nil {
				t.Fatal(err)
			}
			custody, want = filestore.AppendCustodyReleased, core.ErrFilestoreConflict
		case 2:
			request.Path = mustAppendAbsolute(t, filepath.Join(dir, "missing", "incoming"))
			want = fs.ErrNotExist
		case 3:
			var cancel context.CancelCauseFunc
			ctx, cancel = context.WithCancelCause(t.Context())
			cancel(io.ErrClosedPipe)
			want = io.ErrClosedPipe
		}
		err = destination.RotatePath(ctx, request)
		observation, observeErr := destination.ObserveAppend()
		if !errors.Is(err, want) || observeErr != nil || observation.Custody != custody || !errors.Is(observation.RotationError, want) {
			t.Fatalf("rotation=%v observation=(%+v,%v), want cause%v custody%s", err, observation, observeErr, want, custody)
		}
		if err := destination.Close(); err != nil {
			t.Fatal(err)
		}
		observation, err = destination.ObserveAppend()
		if err != nil || observation.Custody != filestore.AppendCustodyReleased || observation.RotationError != nil {
			t.Fatalf("closed observation=(%+v,%v), want released without pending refusal", observation, err)
		}
		actual, err := os.ReadFile(filepath.Join(dir, "outgoing"))
		if err != nil || !bytes.Equal(actual, payload) {
			t.Fatalf("outgoing=(%d bytes,%v), want exact%d input bytes", len(actual), err, len(payload))
		}
		if selector%4 == 0 {
			incoming, err := os.ReadFile(filepath.Join(dir, "incoming"))
			if err != nil || len(incoming) != 0 {
				t.Fatalf("incoming=(%d bytes,%v), want newly created empty file", len(incoming), err)
			}
		}
		if selector%4 == 1 {
			incoming, err := os.ReadFile(filepath.Join(dir, "incoming"))
			if err != nil || string(incoming) != "occupant" {
				t.Fatalf("occupant=(%q,%v), want untouched bytes", incoming, err)
			}
		}
	})
}
