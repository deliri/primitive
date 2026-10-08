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
	"github.com/deliri/primitive/v2026/temporal"
)

func TestScratchReplayScopeResetsNativeExtentAndClosesAfterConsumerStops(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		failure       error
		panicConsumer bool
	}{
		{name: "complete replay"},
		{name: "consumer refusal", failure: io.ErrUnexpectedEOF},
		{name: "consumer panic", failure: core.ErrFilestoreContract, panicConsumer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("replay")
			if err != nil {
				t.Fatal(err)
			}
			rootScope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
				var borrowed filestore.ScratchReplayFile
				calls := 0
				scope, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Use: func(ctx context.Context, file filestore.ScratchReplayFile) error {
					borrowed = file
					calls++
					for _, data := range [][]byte{{0, 255, 31, 14}, {17}} {
						if err := filestore.ResetScratch(ctx, filestore.ScratchResetRequest{File: file}); err != nil {
							return err
						}
						if _, err := file.Write(data); err != nil {
							return err
						}
						if err := filestore.Rewind(ctx, filestore.RewindRequest{Source: file}); err != nil {
							return err
						}
						body, err := io.ReadAll(file)
						if err != nil {
							return err
						}
						if !bytes.Equal(body, data) {
							t.Fatalf("replayed native bytes = %v, want exact %v without prior tail", body, data)
						}
					}
					if tc.panicConsumer {
						panic(io.ErrShortWrite)
					}
					return tc.failure
				}})
				if err != nil || scope.Validate() != nil || !errors.Is(scope.OperationError(), tc.failure) || scope.CleanupError() != nil || calls != 1 || borrowed == nil {
					t.Fatalf("native replay scope = %+v/%v/%d calls, want closed lifetime and %v", scope, err, calls, tc.failure)
				}
				if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("borrowed reader after scope = %v, want closed native handle", err)
				}
				if _, err := borrowed.Write([]byte{19}); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("borrowed writer after scope = %v, want closed native handle", err)
				}
				return nil
			}})
			if err := errors.Join(err, rootScope.Validate(), rootScope.OperationError, rootScope.CleanupError); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestScratchReplayScopeRefusesInvalidOrOccupiedCoordinatesBeforeConsumer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                   string
		nilContext, canceled, nilUse, occupied bool
		want                                   error
	}{
		{name: "nil parent", nilContext: true, want: core.ErrNilContext},
		{name: "canceled parent", canceled: true, want: context.Canceled},
		{name: "missing consumer", nilUse: true, want: core.ErrFilestoreContract},
		{name: "occupied native coordinate", occupied: true, want: core.ErrFilestoreConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath("replay")
			if err != nil {
				t.Fatal(err)
			}
			rootScope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(parent context.Context, root *os.Root) error {
				location := filestore.Location{Root: root, Path: path}
				if tc.occupied {
					written, err := filestore.WithScratchWriterScope(parent, filestore.ScratchWriterScopeRequest{Scratch: filestore.ScratchRequest{Location: location, Mode: 0o600}, Use: func(_ context.Context, writer io.Writer) error { _, err := writer.Write([]byte{31}); return err }})
					if err := errors.Join(err, written.Validate(), written.OperationError(), written.CleanupError()); err != nil {
						return err
					}
				}
				ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: parent})
				if err != nil {
					return err
				}
				defer cancel(nil)
				if tc.canceled {
					cancel(context.Canceled)
				}
				if tc.nilContext {
					ctx = nil
				}
				calls := 0
				request := filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: location, Mode: 0o600}, Use: func(context.Context, filestore.ScratchReplayFile) error { calls++; return nil }}
				if tc.nilUse {
					request.Use = nil
				}
				scope, err := filestore.WithScratchReplayScope(ctx, request)
				if !errors.Is(err, tc.want) || !errors.Is(scope.Validate(), core.ErrFilestoreContract) || calls != 0 {
					t.Fatalf("scope refusal = %+v/%v/%d calls, want unavailable and %v before consumer", scope, err, calls, tc.want)
				}
				absolute, err := directory.JoinRelative(path)
				if err != nil {
					return err
				}
				observation, err := filestore.Inspect(parent, absolute)
				if err != nil {
					return err
				}
				kind, err := observation.Kind()
				if err != nil {
					return err
				}
				wantKind := filestore.PathKindAbsent
				if tc.occupied {
					wantKind = filestore.PathKindRegularFile
				}
				if kind != wantKind {
					t.Fatalf("native coordinate after refusal = %v, want %v", kind, wantKind)
				}
				if tc.occupied {
					var body bytes.Buffer
					if _, err := filestore.Read(parent, filestore.ReadRequest{Location: location, Destination: &body}); err != nil {
						return err
					}
					if !bytes.Equal(body.Bytes(), []byte{31}) {
						t.Fatalf("occupied bytes = %v, want preserved source", body.Bytes())
					}
				}
				return nil
			}})
			if err := errors.Join(err, rootScope.Validate(), rootScope.OperationError, rootScope.CleanupError); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func FuzzScratchReplayConservesReplacementBytes(f *testing.F) {
	f.Add([]byte{0, 255, 31}, uint16(1), false)
	f.Add([]byte{}, uint16(0), false)
	f.Add([]byte{31, 14}, uint16(0), true)
	f.Fuzz(func(t *testing.T, data []byte, extent uint16, refuse bool) {
		// This bounds the independent fixture oracle, not production streams.
		if len(data) > 4096 {
			return
		}
		directory, err := core.ParseAbsolutePath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		path, err := core.ParseRelativePath("replay")
		if err != nil {
			t.Fatal(err)
		}
		rootScope, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(ctx context.Context, root *os.Root) error {
			var borrowed filestore.ScratchReplayFile
			scope, err := filestore.WithScratchReplayScope(ctx, filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600}, Use: func(ctx context.Context, file filestore.ScratchReplayFile) error {
				borrowed = file
				shorter := int(extent) % (len(data) + 1)
				for _, expected := range [][]byte{data, data[:shorter]} {
					if err := filestore.ResetScratch(ctx, filestore.ScratchResetRequest{File: file}); err != nil {
						return err
					}
					written, err := file.Write(expected)
					if err != nil {
						return err
					}
					if written != len(expected) {
						t.Fatalf("native accepted bytes = %d, want %d", written, len(expected))
					}
					if err := filestore.Rewind(ctx, filestore.RewindRequest{Source: file}); err != nil {
						return err
					}
					actual, err := io.ReadAll(file)
					if err != nil {
						return err
					}
					if !bytes.Equal(actual, expected) {
						t.Fatalf("replacement bytes = %v, want exact %v", actual, expected)
					}
				}
				if refuse {
					return io.ErrUnexpectedEOF
				}
				return nil
			}})
			var want error
			if refuse {
				want = io.ErrUnexpectedEOF
			}
			if err != nil || scope.Validate() != nil || scope.CleanupError() != nil || !errors.Is(scope.OperationError(), want) || borrowed == nil {
				t.Fatalf("replacement scope = %+v/%v, want settled native lifetime and %v", scope, err, want)
			}
			if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("replacement handle after return = %v, want closed", err)
			}
			return nil
		}})
		if err := errors.Join(err, rootScope.Validate(), rootScope.OperationError, rootScope.CleanupError); err != nil {
			t.Fatal(err)
		}
	})
}
