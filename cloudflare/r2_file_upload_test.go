package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
)

// r2FileDisposition is how the local R2 stand-in answers one object.
type r2FileDisposition uint8

const (
	r2FileStore r2FileDisposition = iota + 1
	// r2FileRefusePut answers the PUT with 403.
	r2FileRefusePut
	// r2FileTamperRead serves one changed byte on the read-back.
	r2FileTamperRead
	// r2FileTruncateRead serves the stored bytes without their last byte.
	r2FileTruncateRead
)

// localR2 is a LOCAL stand-in for R2 over real TLS and the real Exchange
// client, verifying each SigV4 signature. It is not the live service.
type localR2 struct {
	t           *testing.T
	disposition r2FileDisposition
	mu          sync.Mutex
	stored      []byte
	present     bool
	requests    atomic.Int64
	createOnly  atomic.Bool
}

func (r *localR2) serve(w http.ResponseWriter, request *http.Request) {
	r.requests.Add(1)
	if !verifyR2TestSignature(request, []byte("r2-secret-key")) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	switch request.Method {
	case http.MethodPut:
		if request.Header.Get(core.CloudflareR2IfNoneMatchHeader) == core.CloudflareR2CreateOnlyValue {
			r.createOnly.Store(true)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.disposition == r2FileRefusePut {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.present && r.createOnly.Load() {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		r.stored, r.present = body, true
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		if !r.present {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		served := bytes.Clone(r.stored)
		switch r.disposition {
		case r2FileTamperRead:
			served[0] ^= 0xff
		case r2FileTruncateRead:
			served = served[:len(served)-1]
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(served); err != nil {
			r.t.Errorf("serve object: %v", err)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// r2FileFixture is one sealed source, its expected integrity, a credential
// file in the calling test's own directory, and a local provider.
type r2FileFixture struct {
	provider *localR2
	client   R2Client
	request  R2FileUpload
}

// r2CredentialText is the strict credential document the fixture writes.
const r2CredentialText = `{"access_key_id":"r2-access-key","secret_access_key":"r2-secret-key"}`

func newR2FileFixture(t *testing.T, directory string, disposition r2FileDisposition) r2FileFixture {
	t.Helper()
	provider := &localR2{t: t, disposition: disposition}
	client, err := NewR2Client(testExchange(t, provider.serve))
	if err != nil {
		t.Fatal(err)
	}
	sealed := []byte(strings.Repeat("sealed archive bytes ", 512))
	digest := core.NewDigestWriter()
	if _, err := digest.Write(sealed); err != nil {
		t.Fatal(err)
	}
	sha, extent, err := digest.Seal()
	if err != nil {
		t.Fatal(err)
	}
	credential := filepath.Join(directory, "r2.json")
	if err := os.WriteFile(credential, []byte(r2CredentialText), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(credential)
	if err != nil {
		t.Fatal(err)
	}
	intent := testR2Intent(t, exchange.MethodPut, "anvil/archive.tar.gz")
	account, err := ParseAccountID(strings.Repeat("a", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	return r2FileFixture{provider: provider, client: client, request: R2FileUpload{
		CredentialFile: path, Account: account, Jurisdiction: R2JurisdictionDefault,
		Bucket: intent.Bucket, Key: intent.Key, Source: bytes.NewReader(sealed),
		Expected:    filestore.ContentIndexEntry{Digest: sha, Extent: extent},
		ContentType: core.HTTPMediaTypeOctetStream(), SignedAt: intent.SignedAt, Expires: intent.Expires, Policy: testPolicy(),
	}}
}

// UploadR2File proves a sealed file is retained exactly or returns zero
// proof. Over real TLS against a LOCAL R2 stand-in (not live R2): an exact
// create-only store verifies by signed read-back; a refused PUT, a
// create-only conflict, a read-back that differs in a byte or in length, a
// source that is short, long or has the right length but other bytes all
// return zero proof with their own cause. Every credential file that is not
// a private, strict two-member document is refused before any request.
func TestUploadR2FileLocalProviderLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive/an exact create-only store verifies by read-back", func(t *testing.T) {
		t.Parallel()
		f := newR2FileFixture(t, t.TempDir(), r2FileStore)
		got, err := UploadR2File(t.Context(), f.client, f.request)
		if err != nil || got.Validate() != nil || got.Digest() != f.request.Expected.Digest || got.Extent() != f.request.Expected.Extent || got.Bucket() != f.request.Bucket || got.Key() != f.request.Key {
			t.Fatalf("UploadR2File = (%+v, %v), want proof of the sealed digest and extent", got, err)
		}
		if !f.provider.createOnly.Load() || f.provider.requests.Load() != 2 {
			t.Fatalf("create-only %t, requests %d, want a create-only PUT and one GET", f.provider.createOnly.Load(), f.provider.requests.Load())
		}
	})
	for _, hostile := range []struct {
		name        string
		disposition r2FileDisposition
		source      func(sealed []byte) []byte
		preexisting bool
		want        error
	}{
		{name: "a refused PUT", disposition: r2FileRefusePut, want: core.ErrExchangeAuthentication},
		{name: "a create-only conflict", disposition: r2FileStore, preexisting: true, want: core.ErrExchangeRejected},
		{name: "a read-back with one byte changed", disposition: r2FileTamperRead, want: core.ErrCloudflareVerification},
		{name: "a read-back one byte short", disposition: r2FileTruncateRead, want: core.ErrCloudflareVerification},
		{name: "a source with other bytes of the same length", disposition: r2FileStore, source: func(sealed []byte) []byte {
			changed := bytes.Clone(sealed)
			changed[len(changed)-1] ^= 0xff
			return changed
		}, want: core.ErrCloudflareVerification},
		{name: "a source one byte short", disposition: r2FileStore, source: func(sealed []byte) []byte { return sealed[:len(sealed)-1] }, want: core.ErrExchangeRequest},
		{name: "a source one byte long", disposition: r2FileStore, source: func(sealed []byte) []byte { return append(bytes.Clone(sealed), 'x') }, want: core.ErrExchangeRequest},
	} {
		t.Run("negative/"+hostile.name+" returns zero proof", func(t *testing.T) {
			t.Parallel()
			f := newR2FileFixture(t, t.TempDir(), hostile.disposition)
			if hostile.preexisting {
				f.provider.stored, f.provider.present = []byte("already retained"), true
			}
			if hostile.source != nil {
				sealed := []byte(strings.Repeat("sealed archive bytes ", 512))
				f.request.Source = bytes.NewReader(hostile.source(sealed))
			}
			got, err := UploadR2File(t.Context(), f.client, f.request)
			if err == nil || got != (R2VerifiedObject{}) || !errors.Is(err, hostile.want) {
				t.Fatalf("UploadR2File = (%+v, %v), want zero proof and %v", got, err, hostile.want)
			}
			if hostile.preexisting && string(f.provider.stored) != "already retained" {
				t.Fatalf("existing object = %q, want it untouched", f.provider.stored)
			}
		})
	}
	// Whitespace pads a valid document past the file budget, so only the
	// budget can refuse it.
	oversized := r2CredentialText + strings.Repeat(" ", r2CredentialFileMaximumBytes+1-len(r2CredentialText))
	for _, credential := range []struct {
		name, text string
		mode       os.FileMode
		// link plants the file as a symbolic link to a private, valid copy.
		link bool
	}{
		{name: "group-readable", text: r2CredentialText, mode: 0o640},
		{name: "one byte past its budget", text: oversized, mode: 0o600},
		// The link itself is private where the platform allows it; the bytes
		// read are the group-readable target's, which must decide.
		{name: "reached through a private link to a group-readable file", text: r2CredentialText, mode: 0o640, link: true},
		{name: "carrying an unknown member", text: `{"access_key_id":"r2-access-key","secret_access_key":"r2-secret-key","region":"auto"}`, mode: 0o600},
		{name: "carrying a session token", text: `{"access_key_id":"r2-access-key","secret_access_key":"r2-secret-key","session_token":"t"}`, mode: 0o600},
		{name: "missing its secret", text: `{"access_key_id":"r2-access-key"}`, mode: 0o600},
		{name: "malformed", text: `{"access_key_id":`, mode: 0o600},
	} {
		t.Run("negative/a credential file "+credential.name+" is refused before any request", func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			f := newR2FileFixture(t, directory, r2FileStore)
			path := filepath.Join(directory, "r2.json")
			if credential.link {
				path = filepath.Join(directory, "r2-target.json")
				if err := os.Remove(filepath.Join(directory, "r2.json")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("r2-target.json", filepath.Join(directory, "r2.json")); err != nil {
					t.Fatal(err)
				}
				// Where the platform allows it (macOS), the link itself is made
				// private, so only the target's own mode can refuse it.
				err := unix.Fchmodat(unix.AT_FDCWD, filepath.Join(directory, "r2.json"), 0o600, unix.AT_SYMLINK_NOFOLLOW)
				if err != nil && !errors.Is(err, unix.EOPNOTSUPP) && !errors.Is(err, unix.ENOTSUP) {
					t.Fatalf("private link mode: %v", err)
				}
			}
			if err := os.WriteFile(path, []byte(credential.text), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, credential.mode); err != nil {
				t.Fatal(err)
			}
			got, err := UploadR2File(t.Context(), f.client, f.request)
			if !errors.Is(err, core.ErrCloudflareAuthentication) || got != (R2VerifiedObject{}) || f.provider.requests.Load() != 0 {
				t.Fatalf("UploadR2File = (%+v, %v), requests %d, want zero proof, %v and no request", got, err, f.provider.requests.Load(), core.ErrCloudflareAuthentication)
			}
		})
	}
	t.Run("neutral/a credential file exactly at its budget verifies", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		f := newR2FileFixture(t, directory, r2FileStore)
		padded := r2CredentialText + strings.Repeat(" ", r2CredentialFileMaximumBytes-len(r2CredentialText))
		if err := os.WriteFile(filepath.Join(directory, "r2.json"), []byte(padded), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := UploadR2File(t.Context(), f.client, f.request)
		if err != nil || got.Validate() != nil {
			t.Fatalf("UploadR2File with a %d-byte credential file = (%+v, %v), want verified proof", len(padded), got, err)
		}
	})
	t.Run("neutral/a cancelled upload sends nothing and proves nothing", func(t *testing.T) {
		t.Parallel()
		f := newR2FileFixture(t, t.TempDir(), r2FileStore)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, err := UploadR2File(ctx, f.client, f.request)
		if err == nil || got != (R2VerifiedObject{}) || f.provider.requests.Load() != 0 {
			t.Fatalf("UploadR2File(cancelled) = (%+v, %v), requests %d, want zero proof and no request", got, err, f.provider.requests.Load())
		}
	})
}

// R2 accepts one PUT of at most 5 MiB less than 5 GiB. A write of exactly that
// many bytes is admitted; one byte more, and the former 5 GiB ceiling, are
// refused as binding violations before any request, by both the PUT door and
// the sealed-file door.
func TestR2SingleUploadBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	grant, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodPut, "anvil/archive.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		layer, name string
		bytes       uint64
		admitted    bool
	}{
		{layer: "positive", name: "exactly 5 MiB less than 5 GiB", bytes: (5*1024 - 5) * 1024 * 1024, admitted: true},
		{layer: "negative", name: "one byte past the ceiling", bytes: (5*1024-5)*1024*1024 + 1},
		{layer: "negative", name: "the former 5 GiB ceiling", bytes: 5 * 1024 * 1024 * 1024},
		{layer: "neutral", name: "an empty object", bytes: 0, admitted: true},
	} {
		t.Run(row.layer+"/"+row.name, func(t *testing.T) {
			t.Parallel()
			length, err := core.NewByteLength(row.bytes)
			if err != nil {
				t.Fatal(err)
			}
			err = R2WriteRequest{Source: bytes.NewReader(nil), Grant: grant, Bytes: length}.Validate()
			if row.admitted && err != nil || !row.admitted && !errors.Is(err, core.ErrCloudflareBinding) {
				t.Fatalf("PUT of %d bytes: Validate = %v, want admitted=%t", row.bytes, err, row.admitted)
			}
			f := newR2FileFixture(t, t.TempDir(), r2FileStore)
			f.request.Expected.Extent = length
			if row.bytes == 0 {
				// A zero extent only ever pairs with the empty digest.
				f.request.Expected.Digest = core.SHA256Of(nil)
			}
			err = f.request.Validate()
			if row.admitted && err != nil || !row.admitted && !errors.Is(err, core.ErrCloudflareBinding) {
				t.Fatalf("sealed file of %d bytes: Validate = %v, want admitted=%t", row.bytes, err, row.admitted)
			}
		})
	}
}

// oversizedCredential yields a valid document padded with whitespace to four
// budgets, as a file that grew after its inspection would, counting what was
// consumed. Only the budget can refuse it.
type oversizedCredential struct{ consumed int }

func (e *oversizedCredential) Read(data []byte) (int, error) {
	if e.consumed >= 4*r2CredentialFileMaximumBytes {
		return 0, io.EOF
	}
	for i := range data {
		data[i] = ' '
		if at := e.consumed + i; at < len(r2CredentialText) {
			data[i] = r2CredentialText[at]
		}
	}
	e.consumed += len(data)
	return len(data), nil
}

// The bytes consumed from a credential file are bounded by its budget, not by
// what an earlier inspection saw: a source four budgets long is refused after
// consuming at most one byte past the budget.
func TestR2CredentialReadIsBoundedLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("negative/a source far past its budget is refused within it", func(t *testing.T) {
		t.Parallel()
		source := &oversizedCredential{}
		got, err := decodeBoundedR2Credentials(source)
		if !errors.Is(err, core.ErrCloudflareAuthentication) || got.Validate() == nil || source.consumed > r2CredentialFileMaximumBytes+1 {
			t.Fatalf("decode = (%v, %v) after consuming %d bytes, want %v within %d bytes", got.Validate(), err, source.consumed, core.ErrCloudflareAuthentication, r2CredentialFileMaximumBytes+1)
		}
	})
	t.Run("positive/a document within its budget decodes", func(t *testing.T) {
		t.Parallel()
		got, err := decodeBoundedR2Credentials(strings.NewReader(r2CredentialText))
		if err != nil || got.Validate() != nil {
			t.Fatalf("decode = (%v, %v), want valid credentials", got.Validate(), err)
		}
		if err := got.Close(); err != nil {
			t.Fatalf("close credentials: %v", err)
		}
	})
}
