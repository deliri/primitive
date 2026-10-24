package filestore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppendSpoolAdmissionRefusesBeforeCallback(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"nil_callback", "nil_root", "zero_path", "root_path", "directory_mode", "existing_file", "existing_symlink", "canceled_context"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			called := false
			request := AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(context.Context, *AppendSpool) error {
				called = true
				return nil
			}}
			want := error(core.ErrFilestoreContract)
			switch outcome {
			case "nil_callback":
				request.Use = nil
			case "nil_root":
				request.Location.Root = nil
			case "zero_path":
				request.Location.Path = core.RelativePath{}
			case "root_path":
				var err error
				request.Location.Path, err = core.ParseRelativePath(".")
				if err != nil {
					t.Fatal(err)
				}
			case "directory_mode":
				request.Mode |= fs.ModeDir
			case "existing_file":
				if err := location.Root.WriteFile("spool", []byte("foreign"), 0o600); err != nil {
					t.Fatal(err)
				}
				want = fs.ErrExist
			case "existing_symlink":
				if err := location.Root.Symlink("missing", "spool"); err != nil {
					t.Fatal(err)
				}
				want = fs.ErrExist
			case "canceled_context":
				cancel(fs.ErrPermission)
				want = context.Canceled
			}
			result, err := WithAppendSpoolScope(ctx, request)
			if called || !errors.Is(err, want) || result.Validate() == nil {
				t.Fatalf("admission = (called=%t,%+v,%v), want no callback, zero result and %v", called, result, err, want)
			}
			if outcome == "existing_file" {
				got, err := location.Root.ReadFile("spool")
				if err != nil || string(got) != "foreign" {
					t.Fatalf("foreign bytes = (%q,%v), want unchanged", got, err)
				}
			}
		})
	}
}

func TestAppendSpoolReaderCancellationAndScopeJoin(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"reader_cancellation", "scope_closing", "consumer_refusal", "producer_abort"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)
				var readerResult error
				var accepted bool
				var done chan error
				result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(scope context.Context, spool *AppendSpool) error {
					done = make(chan error, 1)
					go func() {
						done <- spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(ctx context.Context, source io.Reader) error {
							accepted = true
							if outcome == "consumer_refusal" {
								return fs.ErrPermission
							}
							_, err := io.Copy(io.Discard, source)
							return err
						}})
					}()
					synctest.Wait()
					if !accepted {
						t.Fatalf("reader admission = %t, want accepted callback before scope closure", accepted)
					}
					if outcome == "scope_closing" {
						// The scope, not a timeout, wakes and joins the reader.
						return nil
					}
					if outcome == "reader_cancellation" {
						cancel(fs.ErrPermission)
					}
					var cause error
					if outcome == "producer_abort" {
						cause = fs.ErrPermission
					}
					_, sealErr := spool.Seal(scope, cause)
					if cause == nil && sealErr != nil {
						return sealErr
					}
					readerResult = <-done
					return nil
				}})
				if outcome == "scope_closing" {
					readerResult = <-done
				}
				if err != nil || result.Validate() != nil {
					t.Fatalf("scope native lifetime = (%+v,%v), want joined close attempt", result, err)
				}
				want := error(fs.ErrPermission)
				if outcome == "scope_closing" {
					want = fs.ErrClosed
				}
				if !errors.Is(readerResult, want) {
					t.Fatalf("reader result = %v, want preserved %v", readerResult, want)
				}
				if outcome == "reader_cancellation" && !errors.Is(readerResult, context.Canceled) {
					t.Fatalf("reader cancellation = %v, want context identity and cause", readerResult)
				}
				if outcome == "consumer_refusal" || outcome == "reader_cancellation" {
					proof, err := result.Content()
					if err != nil || proof.Digest != core.SHA256Of(nil) {
						t.Fatalf("independent producer proof = (%+v,%v), want successful empty production", proof, err)
					}
				} else if proof, err := result.Content(); proof != (ContentIndexEntry{}) || !errors.Is(err, core.ErrFilestoreSource) {
					t.Fatalf("unsealed or aborted proof = (%+v,%v), want zero and source refusal", proof, err)
				}
			})
		})
	}
}

func TestAppendSpoolReaderAndSealRefusals(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"nil_reader_callback", "nil_reader_context", "cancelled_reader_context", "callback_cancels_reader", "foreign_owner_identity", "nil_seal_context", "cancelled_seal_context"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			sealRefusal := outcome == "nil_seal_context" || outcome == "cancelled_seal_context"
			want := error(core.ErrFilestoreContract)
			if outcome == "nil_reader_context" || outcome == "nil_seal_context" {
				want = core.ErrNilContext
			} else if outcome == "cancelled_reader_context" || outcome == "callback_cancels_reader" || outcome == "cancelled_seal_context" {
				want = context.Canceled
			}
			result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
				operationCtx, cancel := context.WithCancelCause(ctx)
				defer cancel(nil)
				if outcome == "cancelled_reader_context" || outcome == "cancelled_seal_context" {
					cancel(fs.ErrPermission)
				}
				if outcome == "nil_reader_context" || outcome == "nil_seal_context" {
					operationCtx = nil
				}
				if sealRefusal {
					proof, err := spool.Seal(operationCtx, nil)
					if proof != (ContentIndexEntry{}) || !errors.Is(err, want) || (errors.Is(want, context.Canceled) && !errors.Is(err, fs.ErrPermission)) {
						t.Fatalf("refused seal = (%+v,%v), want zero proof and %v with its native cause", proof, err, want)
					}
					return nil
				}
				called := false
				request := AppendSpoolReadRequest{Use: func(context.Context, io.Reader) error {
					called = true
					cancel(fs.ErrPermission)
					return nil
				}}
				if outcome == "nil_reader_callback" {
					request.Use = nil
				}
				owner := spool
				if outcome == "foreign_owner_identity" {
					owner = &AppendSpool{self: spool}
				}
				err := owner.WithReader(operationCtx, request)
				if called != (outcome == "callback_cancels_reader") || !errors.Is(err, want) || (errors.Is(want, context.Canceled) && !errors.Is(err, fs.ErrPermission)) {
					t.Fatalf("reader refusal = (called %t,%v), want only accepted callback to run and %v with its native cause", called, err, want)
				}
				_, sealErr := spool.Seal(ctx, nil)
				return sealErr
			}})
			proof, proofErr := result.Content()
			if err != nil || result.Validate() != nil {
				t.Fatalf("scope lifetime = (%+v,%v), want native close observation", result, err)
			}
			if sealRefusal {
				if proof != (ContentIndexEntry{}) || !errors.Is(proofErr, want) {
					t.Fatalf("refused producer proof = (%+v,%v), want zero and %v", proof, proofErr, want)
				}
			} else if proofErr != nil || proof.Digest != core.SHA256Of(nil) || proof.Extent.Uint64() != 0 {
				t.Fatalf("independent empty producer proof = (%+v,%v), want native empty content despite reader refusal", proof, proofErr)
			}
		})
	}
}

func TestAppendSpoolZeroOwnersHaveNoNativeAuthority(t *testing.T) {
	t.Parallel()
	for _, owner := range []*AppendSpool{nil, {}} {
		written, writeErr := owner.Write([]byte{1})
		proof, sealErr := owner.Seal(t.Context(), nil)
		called := false
		readerErr := owner.WithReader(t.Context(), AppendSpoolReadRequest{Use: func(context.Context, io.Reader) error { called = true; return nil }})
		if written != 0 || proof != (ContentIndexEntry{}) || called || !errors.Is(writeErr, core.ErrFilestoreContract) || !errors.Is(sealErr, core.ErrFilestoreContract) || !errors.Is(readerErr, core.ErrFilestoreContract) {
			t.Fatalf("zero owner authority = (%d,%+v,called %t,%v/%v/%v), want zero projection and contract refusals", written, proof, called, writeErr, sealErr, readerErr)
		}
	}
}

func TestAppendSpoolCloseJoinsReadersBeforeNativeClose(t *testing.T) {
	t.Parallel()
	location := spoolLocation(t, t.TempDir())
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		accepted, release := make(chan struct{}), make(chan struct{})
		readerDone, scopeDone := make(chan error, 1), make(chan error, 1)
		var nativeBeforeReaderExit error
		go func() {
			result, err := WithAppendSpoolScope(ctx, AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
				go func() {
					readerDone <- spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(ctx context.Context, _ io.Reader) error {
						close(accepted)
						<-ctx.Done()
						_, nativeBeforeReaderExit = spool.file.Stat()
						<-release
						return ctx.Err()
					}})
				}()
				<-accepted
				return nil
			}})
			scopeDone <- errors.Join(err, result.OperationError(), result.CleanupError())
		}()
		synctest.Wait()
		returnedBeforeReaderExit := false
		var scopeErr error
		select {
		case scopeErr = <-scopeDone:
			returnedBeforeReaderExit = true
		default:
		}
		close(release)
		if !returnedBeforeReaderExit {
			scopeErr = <-scopeDone
		}
		readerErr := <-readerDone
		if returnedBeforeReaderExit || nativeBeforeReaderExit != nil || !errors.Is(readerErr, context.Canceled) || !errors.Is(scopeErr, core.ErrFilestoreSource) {
			t.Fatalf("scope close ownership = (early %t, native %v, reader %v, scope %v), want joined reader before native close and unsealed source refusal", returnedBeforeReaderExit, nativeBeforeReaderExit, readerErr, scopeErr)
		}
	})
}

type appendSpoolRegistrationPanic struct{ context.Context }

func (appendSpoolRegistrationPanic) Value(any) any { panic(fs.ErrPermission) }

func TestAppendSpoolContextSetupPrecedesFileAcquisition(t *testing.T) {
	t.Parallel()
	location := spoolLocation(t, t.TempDir())
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	var nativePanic error
	called := false
	func() {
		defer func() { nativePanic, _ = recover().(error) }()
		_, _ = WithAppendSpoolScope(appendSpoolRegistrationPanic{Context: parent}, AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(context.Context, *AppendSpool) error {
			called = true
			return nil
		}})
	}()
	_, pathErr := location.Root.Lstat("spool")
	if called || !errors.Is(nativePanic, fs.ErrPermission) || !errors.Is(pathErr, fs.ErrNotExist) {
		t.Fatalf("scope context registration = (called %t, panic %v, path %v), want preserved native panic before file acquisition", called, nativePanic, pathErr)
	}
}
