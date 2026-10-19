package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestNativeFileHandleLayerTriad(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"synchronize and close", "closed sync keeps native cause", "repeated close keeps native cause", "cancelled sync leaves ownership", "nil context leaves ownership", "absent handle refuses"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if mode == "absent handle refuses" {
				request := filestore.FileHandleRequest{}
				for _, err := range []error{request.Validate(), filestore.CloseFile(request), filestore.SyncFile(t.Context(), request)} {
					if !errors.Is(err, core.ErrFilestoreContract) {
						t.Fatalf("absent handle=%v, want contract refusal", err)
					}
				}
				entries, err := os.ReadDir(directory)
				if err != nil || len(entries) != 0 {
					t.Fatalf("refused handle entries=(%v,%v), want none", entries, err)
				}
				return
			}
			file, err := os.CreateTemp(directory, "native")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
					t.Error(err)
				}
			}()
			body := []byte("native file custody")
			if _, err := file.Write(body); err != nil {
				t.Fatal(err)
			}
			request := filestore.FileHandleRequest{File: file}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "closed sync keeps native cause":
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				err := filestore.SyncFile(t.Context(), request)
				if !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrFilestoreDestination) || errors.Is(err, core.ErrFilestoreContract) {
					t.Fatalf("closed sync=%v, want native closed and destination identities", err)
				}
				return
			case "repeated close keeps native cause":
				if err := filestore.CloseFile(request); err != nil {
					t.Fatal(err)
				}
				err := filestore.CloseFile(request)
				if !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrFilestoreCleanup) {
					t.Fatalf("repeated close=%v, want native closed and cleanup identities", err)
				}
				return
			case "cancelled sync leaves ownership":
				ctx, cancel := context.WithCancelCause(t.Context())
				cancel(fs.ErrPermission)
				err := filestore.SyncFile(ctx, request)
				if !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) {
					t.Fatalf("cancelled sync=%v, want cancellation and its native cause", err)
				}
			case "nil context leaves ownership":
				if err := filestore.SyncFile(nil, request); !errors.Is(err, core.ErrNilContext) {
					t.Fatalf("nil context sync=%v, want ErrNilContext", err)
				}
			case "synchronize and close":
				if err := filestore.SyncFile(t.Context(), request); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unknown native file case %q", mode)
			}
			if _, err := file.Stat(); err != nil {
				t.Fatalf("sync retained ownership=%v, want live handle", err)
			}
			if err := filestore.CloseFile(request); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("closed file=%v, want native ErrClosed", err)
			}
			got, err := os.ReadFile(file.Name())
			if err != nil || !bytes.Equal(got, body) {
				t.Fatalf("retained bytes=(%q,%v), want %q", got, err, body)
			}
		})
	}
}

func TestNativeRootHandleLayerTriad(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"close acquired root", "repeat native closure", "absent root refuses"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if mode == "absent root refuses" {
				request := filestore.RootHandleRequest{}
				if err := filestore.CloseRoot(request); !errors.Is(err, core.ErrFilestoreContract) {
					t.Fatalf("absent root=%v, want contract refusal", err)
				}
				return
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			}()
			child, err := root.OpenFile("child", os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := child.Close(); err != nil {
					t.Error(err)
				}
			}()
			request := filestore.RootHandleRequest{Root: root}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
			if err := filestore.CloseRoot(request); err != nil {
				t.Fatal(err)
			}
			if mode == "repeat native closure" {
				if err := filestore.CloseRoot(request); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := root.Stat("child"); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("closed root=%v, want native ErrClosed", err)
			}
			if n, err := child.Write([]byte("still owned")); err != nil || n != 11 {
				t.Fatalf("child after root closure=(%d,%v), want its independent native lifetime", n, err)
			}
		})
	}
}
