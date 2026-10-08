package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestFileScopesRefuseInvalidIngressAndAcquisitionWithoutCallingConsumer(t *testing.T) {
	t.Parallel()
	for _, write := range []bool{false, true} {
		for _, tc := range []struct {
			name                                                                string
			nilContext, canceled, nilUse, nilRoot, zeroPath, acquisitionFailure bool
			want                                                                error
		}{
			{name: "nil context", nilContext: true, want: core.ErrNilContext},
			{name: "canceled context", canceled: true, want: context.Canceled},
			{name: "missing consumer", nilUse: true, want: core.ErrFilestoreContract},
			{name: "missing root", nilRoot: true, want: core.ErrFilestoreContract},
			{name: "missing relative coordinate", zeroPath: true, want: core.ErrFilestoreContract},
			{name: "native acquisition refusal", acquisitionFailure: true},
		} {
			name := "read/" + tc.name
			if write {
				name = "write/" + tc.name
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				directory, err := core.ParseAbsolutePath(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				path, err := core.ParseRelativePath("subject")
				if err != nil {
					t.Fatal(err)
				}
				temporary, err := core.ParseRelativePath("temporary")
				if err != nil {
					t.Fatal(err)
				}
				result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: directory, Use: func(parent context.Context, root *os.Root) error {
					location := filestore.Location{Root: root, Path: path}
					present := (!write && !tc.acquisitionFailure) || (write && tc.acquisitionFailure)
					if present {
						if _, err := filestore.Write(parent, filestore.WriteRequest{Location: location, Temporary: temporary, Source: bytes.NewReader([]byte{31}), Mode: 0o600, Install: filestore.InstallCreate}); err != nil {
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
					if tc.nilRoot {
						location.Root = nil
					}
					if tc.zeroPath {
						location.Path = core.RelativePath{}
					}
					calls := 0
					var observed filestore.FileScopeResult
					var refusal error
					want := tc.want
					if write {
						request := filestore.ScratchWriterScopeRequest{Scratch: filestore.ScratchRequest{Location: location, Mode: 0o600}, Use: func(context.Context, io.Writer) error { calls++; return nil }}
						if tc.nilUse {
							request.Use = nil
						}
						observed, refusal = filestore.WithScratchWriterScope(ctx, request)
						if tc.acquisitionFailure {
							want = core.ErrFilestoreConflict
						}
					} else {
						request := filestore.ReadScopeRequest{Location: location, Use: func(context.Context, io.ReadSeeker) error { calls++; return nil }}
						if tc.nilUse {
							request.Use = nil
						}
						observed, refusal = filestore.WithReadScope(ctx, request)
						if tc.acquisitionFailure {
							want = fs.ErrNotExist
						}
					}
					if !errors.Is(refusal, want) || !errors.Is(observed.Validate(), core.ErrFilestoreContract) || calls != 0 {
						t.Fatalf("file scope admission=(%+v,%v,%d calls),want unavailable result,%v and zero calls", observed, refusal, calls, want)
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
					if present {
						wantKind = filestore.PathKindRegularFile
					}
					if kind != wantKind {
						t.Fatalf("namespace after refusal=%v,want %v", kind, wantKind)
					}
					if present {
						var body bytes.Buffer
						if _, err := filestore.Read(parent, filestore.ReadRequest{Location: filestore.Location{Root: root, Path: path}, Destination: &body}); err != nil {
							return err
						}
						if !bytes.Equal(body.Bytes(), []byte{31}) {
							t.Fatalf("source after refusal=%v,want unchanged native bytes", body.Bytes())
						}
					}
					return nil
				}})
				if err != nil {
					t.Fatal(err)
				}
				if err := errors.Join(result.Validate(), result.OperationError, result.CleanupError); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
