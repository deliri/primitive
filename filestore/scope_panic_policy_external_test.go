package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func scopePanicFixture(t testing.TB, data []byte) (core.AbsolutePath, core.RelativePath) {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "subject"), data, 0600); err != nil {
		t.Fatal(err)
	}
	parent, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseRelativePath("subject")
	if err != nil {
		t.Fatal(err)
	}
	return parent, path
}

func TestScopePanicPropagationClosesNativeRootAndReaderForIterator(t *testing.T) {
	t.Parallel()
	parent, path := scopePanicFixture(t, []byte("ab"))
	var root *os.Root
	var reader io.ReadSeeker
	t.Cleanup(func() {
		if reader, ok := reader.(io.Closer); ok {
			_ = reader.Close()
		}
		if root != nil {
			_ = root.Close()
		}
	})
	sequence := func(yield func(byte, error) bool) {
		result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: parent, PanicPolicy: filestore.ScopePanicPropagate, Use: func(ctx context.Context, borrowed *os.Root) error {
			root = borrowed
			scope, err := filestore.WithReadScope(ctx, filestore.ReadScopeRequest{Location: filestore.Location{Root: root, Path: path}, PanicPolicy: filestore.ScopePanicPropagate, Use: func(_ context.Context, source io.ReadSeeker) error {
				reader = source
				var buffer [1]byte
				for {
					n, err := source.Read(buffer[:])
					if n > 0 && !yield(buffer[0], nil) {
						return nil
					}
					if err == io.EOF {
						return nil
					}
					if err != nil {
						return err
					}
				}
			}})
			return errors.Join(err, scope.OperationError(), scope.CleanupError())
		}})
		if err := errors.Join(err, result.OperationError, result.CleanupError); err != nil {
			yield(0, err)
		}
	}
	var observations int
	func() {
		defer func() {
			if got := recover(); got != io.ErrClosedPipe {
				t.Errorf("consumer panic=%v,want exact original identity", got)
			}
		}()
		for value, err := range iter.Seq2[byte, error](sequence) {
			if err != nil || value != 'a' {
				t.Fatalf("Native byte=%d,%v,want first exact byte", value, err)
			}
			observations++
			panic(io.ErrClosedPipe)
		}
	}()
	if observations != 1 || root == nil || reader == nil {
		t.Fatalf("borrowed prefix=%d,root=%v,reader=%v", observations, root, reader)
	}
	var data [1]byte
	if _, err := reader.Read(data[:]); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("reader survived iterator panic: %v", err)
	}
	if file, err := root.Open("."); !errors.Is(err, fs.ErrClosed) {
		if file != nil {
			_ = file.Close()
		}
		t.Fatalf("root survived iterator panic: %v", err)
	}
}

func TestScopePanicPolicyExhaustiveDomainAndForeignReadAdmission(t *testing.T) {
	t.Parallel()
	parent, path := scopePanicFixture(t, []byte("data"))
	result, err := filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: parent, Use: func(ctx context.Context, root *os.Root) error {
		for raw := 0; raw < 256; raw++ {
			policy := filestore.ScopePanicPolicy(raw)
			valid := raw == 0 || raw == 1
			if policy.IsValid() != valid || (policy.Validate() == nil) != valid {
				t.Fatalf("panic policy %d changed closed membership", raw)
			}
			if !valid && (!errors.Is(policy.Validate(), core.ErrFilestoreContract) || policy.String() != core.UnknownEnumDiagnostic) {
				t.Fatalf("foreign panic policy %d=%v,%q", raw, policy.Validate(), policy.String())
			}
			if valid {
				if policy.String() == "" || policy.String() == core.UnknownEnumDiagnostic {
					t.Fatal("valid panic policy has no diagnostic")
				}
				continue
			}
			called := false
			scope, err := filestore.WithReadScope(ctx, filestore.ReadScopeRequest{Location: filestore.Location{Root: root, Path: path}, PanicPolicy: policy, Use: func(context.Context, io.ReadSeeker) error { called = true; return nil }})
			if !errors.Is(err, core.ErrFilestoreContract) || scope.Validate() == nil || called {
				t.Fatalf("foreign read policy=%d,called=%t,scope=%+v,error=%v", raw, called, scope, err)
			}
			rootCalled := false
			rootScope, err := filestore.WithRootScope(ctx, filestore.RootScopeRequest{Directory: parent, PanicPolicy: policy, Use: func(context.Context, *os.Root) error { rootCalled = true; return nil }})
			if !errors.Is(err, core.ErrFilestoreContract) || rootScope.Validate() == nil || rootCalled {
				t.Fatalf("foreign root policy=%d,called=%t,scope=%+v,error=%v", raw, rootCalled, rootScope, err)
			}
		}
		return nil
	}})
	if err := errors.Join(err, result.OperationError, result.CleanupError); err != nil {
		t.Fatal(err)
	}
	if filestore.ScopePanicSanitize.String() == filestore.ScopePanicPropagate.String() {
		t.Fatal("panic policies share a diagnostic")
	}
}

func FuzzScopePanicPolicyConservesNativeBytesAndCloseOwnership(f *testing.F) {
	for _, policy := range []uint8{0, 1, 2, 255} {
		for mode := uint8(0); mode < 4; mode++ {
			f.Add([]byte{0, 255, 'a'}, policy, mode)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, rawPolicy, mode uint8) {
		parent, path := scopePanicFixture(t, data)
		policy := filestore.ScopePanicPolicy(rawPolicy)
		mode %= 4
		var root *os.Root
		var source io.ReadSeeker
		t.Cleanup(func() {
			if closer, ok := source.(io.Closer); ok {
				_ = closer.Close()
			}
			if root != nil {
				_ = root.Close()
			}
		})
		var result filestore.RootScopeResult
		var resultErr error
		var recovered any
		rootCalls, readCalls := 0, 0
		func() {
			defer func() { recovered = recover() }()
			result, resultErr = filestore.WithRootScope(t.Context(), filestore.RootScopeRequest{Directory: parent, PanicPolicy: policy, Use: func(ctx context.Context, borrowed *os.Root) error {
				rootCalls++
				root = borrowed
				if mode == 1 {
					panic(io.ErrClosedPipe)
				}
				scope, err := filestore.WithReadScope(ctx, filestore.ReadScopeRequest{Location: filestore.Location{Root: root, Path: path}, PanicPolicy: policy, Use: func(_ context.Context, reader io.ReadSeeker) error {
					readCalls++
					source = reader
					got, err := io.ReadAll(reader)
					if err != nil {
						return err
					}
					if !bytes.Equal(got, data) {
						t.Fatalf("Native bytes changed: %x,want %x", got, data)
					}
					if mode == 2 {
						panic(io.ErrClosedPipe)
					}
					if mode == 3 {
						return io.ErrShortWrite
					}
					return nil
				}})
				return errors.Join(err, scope.OperationError(), scope.CleanupError())
			}})
		}()
		if !policy.IsValid() {
			if rootCalls != 0 || readCalls != 0 || recovered != nil || !errors.Is(resultErr, core.ErrFilestoreContract) || result.Validate() == nil {
				t.Fatalf("foreign policy borrowed a Native object: %d/%d,%v,%v", rootCalls, readCalls, recovered, resultErr)
			}
			return
		}
		wantReads := 1
		if mode == 1 {
			wantReads = 0
		}
		if rootCalls != 1 || readCalls != wantReads {
			t.Fatalf("Native callbacks=%d/%d,mode=%d", rootCalls, readCalls, mode)
		}
		panics := mode == 1 || mode == 2
		if panics && policy == filestore.ScopePanicPropagate {
			if recovered != io.ErrClosedPipe || result.Validate() == nil {
				t.Fatalf("propagating scope panic=%v,receipt=%+v", recovered, result)
			}
		} else {
			if recovered != nil || resultErr != nil || result.Validate() != nil || result.CleanupError != nil {
				t.Fatalf("settled Native scope=%+v,%v,panic=%v", result, resultErr, recovered)
			}
			want := error(nil)
			if panics {
				want = core.ErrFilestoreCallbackPanic
			} else if mode == 3 {
				want = io.ErrShortWrite
			}
			if !errors.Is(result.OperationError, want) || errors.Is(result.OperationError, io.ErrClosedPipe) {
				t.Fatalf("Native callback outcome=%v,want %v with no sanitized payload", result.OperationError, want)
			}
		}
		if source != nil {
			var buffer [1]byte
			if _, err := source.Read(buffer[:]); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("reader escaped Native closure: %v", err)
			}
		}
		if root != nil {
			file, err := root.Open(".")
			if file != nil {
				_ = file.Close()
			}
			if !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("root escaped Native closure: %v", err)
			}
		}
	})
}
