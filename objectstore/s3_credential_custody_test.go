package objectstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func s3CredentialBudget(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("credential test budget=%d error=%v, want positive count", value, err)
	}
	return count
}

func TestS3CredentialFileCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                      string
		mode                      os.FileMode
		budgetDelta               int
		zero, directory, overflow bool
		want                      error
	}{
		{name: "owner writable exact budget", mode: 0600},
		{name: "owner read only exact budget", mode: 0400},
		{name: "one spare byte still requires native EOF", mode: 0600, budgetDelta: 1},
		{name: "one byte over declared budget", mode: 0600, budgetDelta: -1, want: core.ErrObjectStoreSize},
		{name: "group read authority refused", mode: 0640, want: core.ErrObjectStoreContract},
		{name: "group write authority refused", mode: 0620, want: core.ErrObjectStoreContract},
		{name: "other read authority refused", mode: 0604, want: core.ErrObjectStoreContract},
		{name: "other write authority refused", mode: 0602, want: core.ErrObjectStoreContract},
		{name: "public read authority refused", mode: 0644, want: core.ErrObjectStoreContract},
		{name: "neutral absent budget has no default", mode: 0600, zero: true, want: core.ErrObjectStoreContract},
		{name: "unrepresentable sentinel refuses before effects", mode: 0600, overflow: true, want: core.ErrNumericOverflow},
		{name: "directory is not a regular credential file", mode: 0600, directory: true, want: core.ErrFilestoreSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			payload := []byte("sealed")
			r := s3FileFixture(t, dir, payload)
			r.CredentialMaximumBytes = s3CredentialBudget(t, uint64(len(s3CredentialFixtureJSON(t))+tc.budgetDelta))
			if tc.zero {
				r.CredentialMaximumBytes = core.ByteCount{}
			}
			if tc.overflow {
				r.CredentialMaximumBytes = s3CredentialBudget(t, math.MaxInt64)
			}
			if err := os.Chmod(r.CredentialFile.String(), tc.mode); err != nil {
				t.Fatal(err)
			}
			if tc.directory {
				if err := os.Remove(r.CredentialFile.String()); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(r.CredentialFile.String(), 0700); err != nil {
					t.Fatal(err)
				}
			}
			var calls atomic.Int32
			client := s3LocalProvider(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, request.Body)
				w.WriteHeader(http.StatusOK)
			}))
			source := &s3CountedSource{reader: bytes.NewReader(payload)}
			r.Source = source
			got, gotErr := UploadS3File(t.Context(), client, r)
			if tc.want == nil {
				if gotErr != nil || got.Validate() != nil || got.Commitment() != CommitmentConfirmed || got.Bytes() != r.Integrity.Length || got.SHA256() != r.Integrity.SHA256 || calls.Load() != 1 || source.Len() != 0 {
					t.Fatalf("credential custody upload=(%v,%v) calls=%d unread=%d, want exact confirmed upload", got, gotErr, calls.Load(), source.Len())
				}
				return
			}
			if got != (Transfer{}) || !errors.Is(gotErr, tc.want) || !errors.Is(gotErr, core.ErrObjectStoreContract) || calls.Load() != 0 || source.reads.Load() != 0 {
				t.Fatalf("credential custody refusal=(%v,%v) calls=%d reads=%d, want zero/%v/no effects", got, gotErr, calls.Load(), source.reads.Load(), tc.want)
			}
		})
	}
}

func TestS3CredentialBudgetNativeCounterLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		budget    uint64
		wantLimit int64
		want      error
	}{
		{name: "minimum budget reserves sentinel", budget: 1, wantLimit: 2},
		{name: "last representable sentinel", budget: math.MaxInt64 - 1, wantLimit: math.MaxInt64},
		{name: "sentinel addition overflow", budget: math.MaxInt64, want: core.ErrNumericOverflow},
		{name: "unsigned budget exceeds native counter", budget: math.MaxInt64 + 1, want: core.ErrNumericOverflow},
		{name: "maximum unsigned budget refused", budget: math.MaxUint64, want: core.ErrNumericOverflow},
		{name: "neutral missing budget refused", want: core.ErrPrimitiveContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var budget core.ByteCount
			if tc.budget > 0 {
				budget = s3CredentialBudget(t, tc.budget)
			}
			limit, err := s3CredentialReadLimit(budget)
			if (tc.want == nil && (err != nil || limit != tc.wantLimit)) || (tc.want != nil && (!errors.Is(err, tc.want) || limit != 0)) {
				t.Fatalf("credential read limit=(%d,%v), want (%d,%v)", limit, err, tc.wantLimit, tc.want)
			}
		})
	}
}

// This reader exposes actual accepted bytes and a late native error; it does
// not stand in for filesystem metadata or a provider's durability claim.
type s3CredentialProbe struct {
	source io.Reader
	read   int
}

type s3CredentialHeldProbe struct {
	filestore.InspectedReadSource
	read int
}

func (r *s3CredentialHeldProbe) Read(p []byte) (int, error) {
	n, err := r.InspectedReadSource.Read(p)
	r.read += n
	return n, err
}

func TestS3CredentialObservedOversizeReadsNoBytes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := s3FileFixture(t, dir, []byte("sealed"))
	location, err := filestore.OpenParent(t.Context(), r.CredentialFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Error(err)
		}
	}()
	var consumed int
	var got s3FileCredentials
	defer got.destroy()
	scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, held filestore.InspectedRead) error {
		probe := &s3CredentialHeldProbe{InspectedReadSource: held.Source}
		held.Source = probe
		var decodeErr error
		got, decodeErr = decodeS3InspectedCredentials(ctx, held, s3CredentialBudget(t, uint64(len(s3CredentialFixtureJSON(t))-1)))
		consumed = probe.read
		return decodeErr
	}})
	if err != nil || !errors.Is(scope.OperationError(), core.ErrObjectStoreSize) || scope.CleanupError() != nil || consumed != 0 || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 {
		t.Fatalf("oversized credential scope=(%v,%v,%v) read=%d sizes=(%d,%d), want size refusal without reading credentials", err, scope.OperationError(), scope.CleanupError(), consumed, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
	}
}

func (r *s3CredentialProbe) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.read += n
	return n, err
}

func TestS3CredentialBoundedDecodeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                   string
		extra                  int
		budgetDelta            int
		empty, zero, nilSource bool
		tail                   error
		want                   error
	}{
		{name: "exact budget ends at native EOF"},
		{name: "budget above extent preserves exact pair", budgetDelta: 1},
		{name: "complete JSON exceeds budget by one", budgetDelta: -1, want: core.ErrObjectStoreSize},
		{name: "one trailing whitespace exceeds budget", extra: 1, want: core.ErrObjectStoreSize},
		{name: "many trailing windows cannot evade sentinel", extra: 65537, want: core.ErrObjectStoreSize},
		{name: "late read failure preserves native cause", tail: io.ErrClosedPipe, budgetDelta: 1, want: io.ErrClosedPipe},
		{name: "wrapped EOF is not successful completion", tail: errors.Join(io.EOF, io.ErrUnexpectedEOF), budgetDelta: 1, want: io.ErrUnexpectedEOF},
		{name: "empty file cannot invent credentials", empty: true, want: core.ErrObjectStoreContract},
		{name: "zero budget consumes no bytes", zero: true, want: core.ErrPrimitiveContract},
		{name: "nil source refuses before decoder", nilSource: true, want: core.ErrObjectStoreSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			canonical := s3CredentialFixtureJSON(t)
			data := append(bytes.Clone(canonical), bytes.Repeat([]byte(" "), tc.extra)...)
			if tc.empty {
				data = nil
			}
			var reader io.Reader = bytes.NewReader(data)
			if tc.tail != nil {
				reader = io.MultiReader(reader, s3FileFailingSource{cause: tc.tail})
			}
			probe := &s3CredentialProbe{source: reader}
			var source io.Reader = probe
			if tc.nilSource {
				source = nil
			}
			budget := s3CredentialBudget(t, uint64(len(canonical)+tc.budgetDelta))
			if tc.zero {
				budget = core.ByteCount{}
			}
			got, err := decodeS3BoundedCredentials(t.Context(), source, budget)
			defer got.destroy()
			if tc.want == nil {
				if err != nil || string(got.AccessKeyID.value) != s3FixtureAccess || string(got.SecretAccessKey.value) != s3FixtureSecret || probe.read != len(data) {
					t.Fatalf("bounded credential error=%v sizes=(%d,%d) read=%d, want exact pair and %d bytes", err, len(got.AccessKeyID.value), len(got.SecretAccessKey.value), probe.read, len(data))
				}
				return
			}
			if !errors.Is(err, tc.want) || !errors.Is(err, core.ErrObjectStoreContract) || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 || probe.read > len(canonical)+tc.budgetDelta+1 || (tc.zero && probe.read != 0) {
				t.Fatalf("bounded refusal error=%v sizes=(%d,%d) read=%d, want zero/%v and bounded consumption", err, len(got.AccessKeyID.value), len(got.SecretAccessKey.value), probe.read, tc.want)
			}
		})
	}
}

func TestS3CredentialHeldDescriptorIgnoresPathReplacement(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := s3FileFixture(t, dir, []byte("sealed"))
	location, err := filestore.OpenParent(t.Context(), r.CredentialFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Error(err)
		}
	}()
	var got s3FileCredentials
	defer got.destroy()
	var borrowed io.Reader
	scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, held filestore.InspectedRead) error {
		borrowed = held.Source
		if err := os.Rename(r.CredentialFile.String(), filepath.Join(dir, "original")); err != nil {
			return err
		}
		// A hostile new pathname names a public, oversized different document.
		foreign := strings.Replace(string(s3CredentialFixtureJSON(t)), s3FixtureSecret, "foreign-secret", 1) + strings.Repeat(" ", 65537)
		if err := os.WriteFile(r.CredentialFile.String(), []byte(foreign), 0644); err != nil {
			return err
		}
		var decodeErr error
		got, decodeErr = decodeS3InspectedCredentials(ctx, held, s3CredentialBudget(t, uint64(len(s3CredentialFixtureJSON(t)))))
		return decodeErr
	}})
	if err != nil || scope.OperationError() != nil || scope.CleanupError() != nil || string(got.AccessKeyID.value) != s3FixtureAccess || string(got.SecretAccessKey.value) != s3FixtureSecret {
		t.Fatalf("held credential scope=(%v,%v,%v) sizes=(%d,%d), want original private pair despite replacement", err, scope.OperationError(), scope.CleanupError(), len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
	}
	if borrowed == nil {
		t.Fatalf("borrowed credential source=%v, want a held descriptor", borrowed)
	}
	if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("expired credential descriptor read=%v, want native closed cause", err)
	}
}

func TestS3CredentialGrowthAfterInspectionCannotEscapeBudget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := s3FileFixture(t, dir, []byte("sealed"))
	maximum := s3CredentialBudget(t, uint64(len(s3CredentialFixtureJSON(t))))
	location, err := filestore.OpenParent(t.Context(), r.CredentialFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			t.Error(err)
		}
	}()
	var got s3FileCredentials
	defer got.destroy()
	var decodeErr error
	scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, held filestore.InspectedRead) error {
		appendFile, err := os.OpenFile(r.CredentialFile.String(), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		_, writeErr := io.WriteString(appendFile, strings.Repeat(" ", 65537))
		if err := errors.Join(writeErr, appendFile.Close()); err != nil {
			return err
		}
		got, decodeErr = decodeS3InspectedCredentials(ctx, held, maximum)
		return decodeErr
	}})
	if err != nil || !errors.Is(decodeErr, core.ErrObjectStoreSize) || !errors.Is(scope.OperationError(), core.ErrObjectStoreSize) || scope.CleanupError() != nil || len(got.AccessKeyID.value) != 0 || len(got.SecretAccessKey.value) != 0 {
		t.Fatalf("grown credential scope=(%v,%v,%v) decode=%v sizes=(%d,%d), want bounded size refusal and zero", err, scope.OperationError(), scope.CleanupError(), decodeErr, len(got.AccessKeyID.value), len(got.SecretAccessKey.value))
	}
}
