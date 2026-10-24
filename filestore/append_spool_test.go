package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func spoolLocation(t testing.TB, directory string) Location {
	t.Helper()
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("root close error = %v, want nil", err)
		}
	})
	path, err := core.ParseRelativePath("spool")
	if err != nil {
		t.Fatal(err)
	}
	return Location{Root: root, Path: path}
}

func TestAppendSpoolLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "neutral empty stream seals its actual empty digest"},
		{name: "positive binary data retains every byte", data: []byte{0, 255, 13, 10, 128}},
		{name: "positive transfer window boundary is exact", data: bytes.Repeat([]byte{31}, 32768)},
		{name: "positive one beyond transfer window is exact", data: bytes.Repeat([]byte{127}, 32769)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			var borrowed *AppendSpool
			var expired io.Reader
			result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
				borrowed = spool
				cut := min(len(tc.data), 32768)
				for _, part := range [][]byte{tc.data[:cut], tc.data[cut:]} {
					if n, err := spool.Write(part); err != nil || n != len(part) {
						t.Fatalf("write = (%d,%v), want (%d,nil)", n, err, len(part))
					}
				}
				content, err := spool.Seal(ctx, nil)
				if err != nil || content.Digest != core.SHA256Of(tc.data) || content.Extent.Uint64() != uint64(len(tc.data)) {
					t.Fatalf("seal = (%+v,%v), want native digest and %d bytes", content, err, len(tc.data))
				}
				for range 2 {
					if err := spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(_ context.Context, source io.Reader) error {
						expired = source
						got, err := io.ReadAll(source)
						if err != nil || !bytes.Equal(got, tc.data) {
							t.Fatalf("independent reader = (%x,%v), want exact input bytes", got, err)
						}
						return nil
					}}); err != nil {
						return err
					}
				}
				if n, err := spool.Write([]byte("late")); n != 0 || !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("sealed write = (%d,%v), want zero and closed", n, err)
				}
				if proof, err := spool.Seal(ctx, nil); proof != (ContentIndexEntry{}) || !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("repeated seal = (%+v,%v), want zero and closed", proof, err)
				}
				return nil
			}})
			proof, proofErr := result.Content()
			if err != nil || proofErr != nil || proof.Digest != core.SHA256Of(tc.data) || proof.Extent.Uint64() != uint64(len(tc.data)) {
				t.Fatalf("closed scope = (%+v,%v,%v), want joined native content", proof, err, proofErr)
			}
			if _, err := borrowed.Write(nil); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("expired owner error = %v, want closed", err)
			}
			if _, err := expired.Read(make([]byte, 1)); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("expired reader error = %v, want closed", err)
			}
			got, err := location.Root.ReadFile("spool")
			if err != nil || !bytes.Equal(got, tc.data) {
				t.Fatalf("retained SSD file = (%x,%v), want complete input and no deletion", got, err)
			}
		})
	}
}

func TestAppendSpoolWaitsTemporaryEOFWithoutBlockingWriter(t *testing.T) {
	t.Parallel()
	location := spoolLocation(t, t.TempDir())
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		result, err := WithAppendSpoolScope(ctx, AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
			done := make(chan error, 1)
			var got []byte
			go func() {
				done <- spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(_ context.Context, source io.Reader) error {
					var err error
					got, err = io.ReadAll(source)
					return err
				}})
			}()
			synctest.Wait()
			select {
			case err := <-done:
				t.Fatalf("temporary EOF reader returned %v, want waiting for producer", err)
			default:
			}
			if n, err := spool.Write([]byte("first")); err != nil || n != 5 {
				t.Fatalf("first write = (%d,%v), want (5,nil)", n, err)
			}
			synctest.Wait()
			select {
			case err := <-done:
				t.Fatalf("prefix EOF reader returned %v, want waiting for next write", err)
			default:
			}
			if n, err := spool.Write([]byte("second")); err != nil || n != 6 {
				t.Fatalf("second write = (%d,%v), want (6,nil)", n, err)
			}
			if _, err := spool.Seal(ctx, nil); err != nil {
				return err
			}
			if err := <-done; err != nil || string(got) != "firstsecond" {
				t.Fatalf("joined reader = (%q,%v), want firstsecond and nil", got, err)
			}
			return nil
		}})
		if err != nil || errors.Join(result.OperationError(), result.CleanupError()) != nil {
			t.Fatalf("spool scope = (%+v,%v), want successful native lifetime", result, err)
		}
	})
}

func TestAppendSpoolSlowIndependentReaderDoesNotHoldProducer(t *testing.T) {
	t.Parallel()
	location := spoolLocation(t, t.TempDir())
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var blocked bool
		var written int
		var writeErr, fastErr, slowErr error
		result, err := WithAppendSpoolScope(ctx, AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
			release := make(chan struct{})
			defer close(release)
			done := make(chan error, 2)
			for index := range 2 {
				go func() {
					done <- spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(ctx context.Context, source io.Reader) error {
						if index == 0 {
							select {
							case <-release:
							case <-ctx.Done():
								return context.Cause(ctx)
							}
						}
						got, err := io.ReadAll(source)
						if err == nil && string(got) != "complete" {
							return fs.ErrInvalid
						}
						return err
					}})
				}()
			}
			synctest.Wait()
			writeDone := make(chan struct{})
			go func() {
				defer close(writeDone)
				written, writeErr = spool.Write([]byte("complete"))
			}()
			synctest.Wait()
			select {
			case <-writeDone:
			default:
				blocked = true
				cancel()
				<-writeDone
			}
			if blocked || writeErr != nil {
				cancel()
				fastErr, slowErr = <-done, <-done
				return writeErr
			}
			if _, err := spool.Seal(ctx, nil); err != nil {
				cancel()
				fastErr, slowErr = <-done, <-done
				return err
			}
			synctest.Wait()
			fastErr = <-done
			// Scope cancellation releases the deliberately held reader, whose
			// callback must join before the scope can close its native file.
			cancel()
			slowErr = <-done
			return nil
		}})
		if blocked || written != 8 || writeErr != nil || fastErr != nil || !errors.Is(slowErr, context.Canceled) {
			t.Fatalf("producer/reader ownership = (blocked %t, write %d/%v, readers %v/%v), want independent full SSD write, fast success and joined slow cancellation", blocked, written, writeErr, fastErr, slowErr)
		}
		if err != nil || !errors.Is(result.OperationError(), context.Canceled) {
			t.Fatalf("canceled scope = (%+v,%v), want scope cancellation retained", result, err)
		}
	})
}

func TestAppendSpoolFailureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"unsealed", "producer_refusal", "callback_refusal", "callback_panic", "closed_writer", "closed_on_seal", "close_failure", "foreign_namespace"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			location := spoolLocation(t, t.TempDir())
			result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
				if outcome == "closed_writer" {
					if err := spool.file.Close(); err != nil {
						return err
					}
				}
				_, writeErr := spool.Write([]byte("owned"))
				if outcome == "closed_writer" {
					if !errors.Is(writeErr, fs.ErrClosed) {
						t.Fatalf("closed native writer = %v, want closed cause", writeErr)
					}
				} else if writeErr != nil {
					return writeErr
				}
				switch outcome {
				case "unsealed":
					return nil
				case "callback_refusal":
					return fs.ErrPermission
				case "callback_panic":
					panic(fs.ErrPermission)
				case "foreign_namespace":
					if err := location.Root.Rename("spool", "original"); err != nil {
						return err
					}
					if err := location.Root.WriteFile("spool", []byte("foreign"), 0o600); err != nil {
						return err
					}
				case "closed_on_seal":
					if err := spool.file.Close(); err != nil {
						return err
					}
				}
				var cause error
				if outcome == "producer_refusal" {
					cause = fs.ErrPermission
				}
				proof, sealErr := spool.Seal(ctx, cause)
				if outcome == "producer_refusal" || outcome == "closed_writer" || outcome == "closed_on_seal" {
					want := error(fs.ErrClosed)
					if outcome == "producer_refusal" {
						want = fs.ErrPermission
					}
					if proof != (ContentIndexEntry{}) || !errors.Is(sealErr, want) {
						t.Fatalf("refused seal = (%+v,%v), want zero and %v", proof, sealErr, want)
					}
					return nil
				}
				if sealErr != nil {
					return sealErr
				}
				if outcome == "close_failure" {
					return spool.file.Close()
				}
				return spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(_ context.Context, source io.Reader) error {
					got, err := io.ReadAll(source)
					if err != nil || string(got) != "owned" {
						t.Fatalf("held inode reader = (%q,%v), want owned original bytes", got, err)
					}
					return nil
				}})
			}})
			proof, proofErr := result.Content()
			if err != nil || result.Validate() != nil {
				t.Fatalf("native lifetime = (%+v,%v), want completed close attempt", result, err)
			}
			if outcome == "foreign_namespace" {
				if proofErr != nil || proof.Digest != core.SHA256Of([]byte("owned")) {
					t.Fatalf("original content = (%+v,%v), want owned digest", proof, proofErr)
				}
				got, err := location.Root.ReadFile("spool")
				if err != nil || string(got) != "foreign" {
					t.Fatalf("replacement = (%q,%v), want untouched foreign", got, err)
				}
			} else {
				want := error(fs.ErrClosed)
				switch outcome {
				case "unsealed":
					want = core.ErrFilestoreSource
				case "producer_refusal", "callback_refusal":
					want = fs.ErrPermission
				case "callback_panic":
					want = core.ErrFilestoreCallbackPanic
				case "close_failure":
					want = core.ErrFilestoreCleanup
				}
				if proof != (ContentIndexEntry{}) || !errors.Is(proofErr, want) {
					t.Fatalf("failed scope content = (%+v,%v), want zero and %v", proof, proofErr, want)
				}
			}
		})
	}
}

func FuzzAppendSpoolNativeBytes(f *testing.F) {
	f.Add([]byte{}, uint16(1), false)
	f.Add([]byte{0, 255, 13, 10, 128}, uint16(2), false)
	f.Add(bytes.Repeat([]byte{127}, 32769), uint16(32768), false)
	f.Add([]byte("producer rejected"), uint16(3), true)
	f.Fuzz(func(t *testing.T, data []byte, width uint16, abort bool) {
		if len(data) > 1<<20 {
			return
		}
		location := spoolLocation(t, t.TempDir())
		result, err := WithAppendSpoolScope(t.Context(), AppendSpoolScopeRequest{Location: location, Mode: 0o600, Use: func(ctx context.Context, spool *AppendSpool) error {
			step := max(1, int(width))
			for start := 0; start < len(data); start += step {
				part := data[start:min(start+step, len(data))]
				if n, err := spool.Write(part); n != len(part) || err != nil {
					t.Fatalf("native chunk = (%d,%v), want exact %d bytes", n, err, len(part))
				}
			}
			var cause error
			if abort {
				cause = fs.ErrPermission
			}
			proof, sealErr := spool.Seal(ctx, cause)
			if abort {
				if proof != (ContentIndexEntry{}) || !errors.Is(sealErr, fs.ErrPermission) {
					t.Fatalf("aborted proof = (%+v,%v), want zero and native cause", proof, sealErr)
				}
			} else if sealErr != nil || proof.Digest != core.SHA256Of(data) || proof.Extent.Uint64() != uint64(len(data)) {
				t.Fatalf("sealed proof = (%+v,%v), want exact source facts", proof, sealErr)
			}
			return spool.WithReader(ctx, AppendSpoolReadRequest{Use: func(_ context.Context, source io.Reader) error {
				got, err := io.ReadAll(source)
				if !bytes.Equal(got, data) || abort && !errors.Is(err, fs.ErrPermission) || !abort && err != nil {
					t.Fatalf("native egress = (%x,%v), want input bytes and abort=%t", got, err, abort)
				}
				return nil
			}})
		}})
		proof, proofErr := result.Content()
		if result.Validate() != nil || result.CleanupError() != nil || abort && !errors.Is(result.OperationError(), fs.ErrPermission) || !abort && result.OperationError() != nil {
			t.Fatalf("native scope custody = (%v,%v,%v), want successful close and abort=%t operation facts", result.Validate(), result.OperationError(), result.CleanupError(), abort)
		}
		if err != nil || abort && (!errors.Is(proofErr, fs.ErrPermission) || proof != (ContentIndexEntry{})) || !abort && (proofErr != nil || proof.Digest != core.SHA256Of(data)) {
			t.Fatalf("native scope projection = (%+v,%v,%v), want abort=%t and exact content", proof, err, proofErr, abort)
		}
	})
}
