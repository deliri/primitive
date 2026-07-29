package core

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// governanceRepositoryRoot is the parent of the core package directory. The
// canonical document path is root-relative, so the test resolves it exactly
// once here instead of embedding a second copy of the path text.
const governanceRepositoryRoot = ".."

// bitFlipReader streams source while inverting one bit at an absolute byte
// offset. It mutates content without changing length and holds no document in
// memory, so a digest violation is provable against the real 50 KiB document
// under the same O(1) budget production must satisfy.
type bitFlipReader struct {
	source io.Reader
	offset int64
	read   int64
	mask   byte
}

func (r *bitFlipReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	for index := range n {
		if r.read+int64(index) == r.offset {
			p[index] ^= r.mask
		}
	}
	r.read += int64(n)
	return n, err
}

// singleByteReader yields at most one byte per Read. A verifier that assumes a
// full buffer per call, or that counts calls rather than bytes, fails here
// while passing against a well-behaved file.
type singleByteReader struct {
	source io.Reader
}

func (r singleByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return r.source.Read(p[:1])
}

// failingReader yields prefix bytes and then a non-EOF failure, proving a read
// fault is reported as a source failure rather than silently shortening the
// document into a length violation.
type failingReader struct {
	failure error
	prefix  io.Reader
}

func (r failingReader) Read(p []byte) (int, error) {
	n, err := r.prefix.Read(p)
	if errors.Is(err, io.EOF) {
		return n, r.failure
	}
	return n, err
}

// countingReader records how many bytes a consumer drew from it. Appended after
// the canonical document, it converts "Verify must not read the whole excess"
// from a comment into a measured bound.
type countingReader struct {
	source io.Reader
	read   int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.read += int64(n)
	return n, err
}

func openCanonicalGovernanceDocument(t *testing.T, contract GovernanceDocumentContract) *os.File {
	t.Helper()

	path := filepath.Join(governanceRepositoryRoot, filepath.FromSlash(contract.Path.String()))
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v, want nil", contract.Path.String(), err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close canonical governance document error = %v, want nil", closeErr)
		}
	})
	return file
}

func canonicalGovernanceContractForTest(t *testing.T) GovernanceDocumentContract {
	t.Helper()

	contract, err := GovernanceDocumentTestingProtocol.Contract()
	if err != nil {
		t.Fatalf("GovernanceDocumentTestingProtocol.Contract() error = %v, want nil", err)
	}
	return contract
}

func TestGovernanceDocumentContractAdmitsOnlyTheCanonicalDocument(t *testing.T) {
	t.Parallel()

	contract := canonicalGovernanceContractForTest(t)
	if err := contract.Verify(openCanonicalGovernanceDocument(t, contract)); err != nil {
		t.Fatalf("GovernanceDocumentContract.Verify(canonical document) error = %v, want nil", err)
	}
}

// TestGovernanceDocumentContractRejectsMutatedDocumentStreams attacks the real
// repository document rather than a synthetic stand-in: every case below is the
// canonical 50 KiB file altered in exactly one way that a length-only or
// digest-only check would admit.
func TestGovernanceDocumentContractRejectsMutatedDocumentStreams(t *testing.T) {
	t.Parallel()

	contract := canonicalGovernanceContractForTest(t)
	canonicalBytes, err := contract.Bytes.Int64()
	if err != nil {
		t.Fatalf("GovernanceDocumentContract.Bytes.Int64() error = %v, want nil", err)
	}
	readFailure := errors.New("governance document read fault")

	cases := []struct {
		document func(*testing.T) io.Reader
		name     string
		want     ErrorIdentity
	}{
		{
			name: "canonical stream delivered one byte per read is admitted",
			document: func(t *testing.T) io.Reader {
				return singleByteReader{source: openCanonicalGovernanceDocument(t, contract)}
			},
		},
		{
			name: "empty document cannot satisfy a nonzero canonical length",
			document: func(*testing.T) io.Reader {
				return bytes.NewReader(nil)
			},
			want: ErrGovernanceDocumentLength,
		},
		{
			name: "document truncated by one byte is rejected",
			document: func(t *testing.T) io.Reader {
				return io.LimitReader(openCanonicalGovernanceDocument(t, contract), canonicalBytes-1)
			},
			want: ErrGovernanceDocumentLength,
		},
		{
			name: "document extended by one trailing byte is rejected",
			document: func(t *testing.T) io.Reader {
				return io.MultiReader(openCanonicalGovernanceDocument(t, contract), bytes.NewReader([]byte{'\n'}))
			},
			want: ErrGovernanceDocumentLength,
		},
		{
			name: "document extended far beyond the canonical length is rejected",
			document: func(t *testing.T) io.Reader {
				return io.MultiReader(
					openCanonicalGovernanceDocument(t, contract),
					bytes.NewReader(bytes.Repeat([]byte{'x'}, 1<<20)),
				)
			},
			want: ErrGovernanceDocumentLength,
		},
		{
			name: "first byte bit flip is rejected at canonical length",
			document: func(t *testing.T) io.Reader {
				return &bitFlipReader{source: openCanonicalGovernanceDocument(t, contract), offset: 0, mask: 0x01}
			},
			want: ErrGovernanceDocumentDigest,
		},
		{
			name: "interior byte bit flip is rejected at canonical length",
			document: func(t *testing.T) io.Reader {
				return &bitFlipReader{source: openCanonicalGovernanceDocument(t, contract), offset: canonicalBytes / 2, mask: 0x80}
			},
			want: ErrGovernanceDocumentDigest,
		},
		{
			name: "final byte bit flip is rejected at canonical length",
			document: func(t *testing.T) io.Reader {
				return &bitFlipReader{source: openCanonicalGovernanceDocument(t, contract), offset: canonicalBytes - 1, mask: 0x20}
			},
			want: ErrGovernanceDocumentDigest,
		},
		{
			name: "read fault is a source failure, not a short document",
			document: func(t *testing.T) io.Reader {
				return failingReader{
					prefix:  io.LimitReader(openCanonicalGovernanceDocument(t, contract), canonicalBytes/2),
					failure: readFailure,
				}
			},
			want: ErrGovernanceDocumentSource,
		},
		{
			name: "nil document source is rejected",
			document: func(*testing.T) io.Reader {
				return nil
			},
			want: ErrGovernanceDocumentSource,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := contract.Verify(tc.document(t))
			if tc.want == ErrUnknown {
				if gotErr != nil {
					t.Fatalf("GovernanceDocumentContract.Verify() error = %v, want nil", gotErr)
				}
				return
			}
			requireExactGovernanceIdentity(t, gotErr, tc.want)
		})
	}
}

// TestGovernanceDocumentContractVerifyReadsBoundedExcess measures the O(1)
// claim: an oversized document must be rejected after exactly one byte beyond
// the canonical length, never by draining an attacker-sized stream.
func TestGovernanceDocumentContractVerifyReadsBoundedExcess(t *testing.T) {
	t.Parallel()

	contract := canonicalGovernanceContractForTest(t)
	excess := &countingReader{source: bytes.NewReader(bytes.Repeat([]byte{'x'}, 1<<20))}
	document := io.MultiReader(openCanonicalGovernanceDocument(t, contract), excess)

	if gotErr := contract.Verify(document); !errors.Is(gotErr, ErrGovernanceDocumentLength) {
		t.Fatalf("GovernanceDocumentContract.Verify(oversized document) error = %v, want %v", gotErr, ErrGovernanceDocumentLength)
	}
	if excess.read != 1 {
		t.Fatalf("excess bytes read = %d, want exactly 1 beyond the canonical length", excess.read)
	}
}

func governanceDocumentFailureIdentities() []ErrorIdentity {
	return []ErrorIdentity{
		ErrGovernanceDocumentSource,
		ErrGovernanceDocumentLength,
		ErrGovernanceDocumentDigest,
	}
}

// requireExactGovernanceIdentity pins one verdict and rules out the others. A
// governance rejection that matched more than one document identity would make
// the caller's remediation ambiguous, and one that matched none of them would
// be indistinguishable from any unrelated Primitive failure.
func requireExactGovernanceIdentity(t *testing.T, gotErr error, want ErrorIdentity) {
	t.Helper()

	if !errors.Is(gotErr, want) {
		t.Fatalf("governance error = %v, want %v", gotErr, want)
	}
	for _, other := range governanceDocumentFailureIdentities() {
		if other != want && errors.Is(gotErr, other) {
			t.Fatalf("governance error = %v, must not also match %v", gotErr, other)
		}
	}
	if !errors.Is(gotErr, ErrGovernanceContract) || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf(
			"governance error = %v, want descent through %v and %v",
			gotErr, ErrGovernanceContract, ErrPrimitiveContract,
		)
	}
}

// TestGovernanceDocumentContractVerifyRefusesNoncanonicalContracts proves the
// canonical document cannot be admitted by a contract that was tampered with:
// verification is decided by the canonical fields, never by the caller's copy.
func TestGovernanceDocumentContractVerifyRefusesNoncanonicalContracts(t *testing.T) {
	t.Parallel()

	canonical := canonicalGovernanceContractForTest(t)
	foreignDigest := NewSHA256Digest(sha256.Sum256([]byte("different governance document")))

	cases := []struct {
		name     string
		contract GovernanceDocumentContract
	}{
		{name: "zero contract cannot verify", contract: GovernanceDocumentContract{}},
		{name: "digest substitution cannot verify", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: foreignDigest, Bytes: canonical.Bytes,
		}},
		{name: "length substitution cannot verify", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: canonical.SHA256,
			Bytes: NewByteLength(canonical.Bytes.Uint64() - 1),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A tampered contract must fail as a contract violation, never be
			// laundered into a document-content verdict.
			requireExactGovernanceIdentity(t, tc.contract.Verify(openCanonicalGovernanceDocument(t, canonical)), ErrGovernanceContract)
		})
	}
}

// TestGovernanceDocumentContractRejectsFieldMutations pins Validate to the
// canonical structure: every single-field deviation is a contract violation,
// including the two that a digest-only check would miss.
func TestGovernanceDocumentContractRejectsFieldMutations(t *testing.T) {
	t.Parallel()

	canonical := canonicalGovernanceContractForTest(t)
	foreignPath, err := ParseRelativePath("_docs/not_the_testing_protocol.md")
	if err != nil {
		t.Fatalf("ParseRelativePath(foreign governance path) error = %v, want nil", err)
	}
	foreignDigest := NewSHA256Digest(sha256.Sum256([]byte("different governance document")))
	canonicalBytes := canonical.Bytes.Uint64()

	cases := []struct {
		name     string
		contract GovernanceDocumentContract
	}{
		{name: "zero contract has no document identity", contract: GovernanceDocumentContract{}},
		{name: "unknown document identity cannot carry canonical fields", contract: GovernanceDocumentContract{
			Document: GovernanceDocumentUnknown, Path: canonical.Path, SHA256: canonical.SHA256, Bytes: canonical.Bytes,
		}},
		{name: "future document identity cannot carry canonical fields", contract: GovernanceDocumentContract{
			Document: governanceDocumentLimit, Path: canonical.Path, SHA256: canonical.SHA256, Bytes: canonical.Bytes,
		}},
		{name: "missing path is rejected", contract: GovernanceDocumentContract{
			Document: canonical.Document, SHA256: canonical.SHA256, Bytes: canonical.Bytes,
		}},
		{name: "different valid path cannot redirect doctrine", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: foreignPath, SHA256: canonical.SHA256, Bytes: canonical.Bytes,
		}},
		{name: "missing digest is rejected", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, Bytes: canonical.Bytes,
		}},
		{name: "different valid digest cannot redefine doctrine", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: foreignDigest, Bytes: canonical.Bytes,
		}},
		{name: "missing exact byte length is rejected", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: canonical.SHA256,
		}},
		{name: "one byte short cannot redefine the canonical document", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: canonical.SHA256,
			Bytes: NewByteLength(canonicalBytes - 1),
		}},
		{name: "one trailing byte cannot redefine the canonical document", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: canonical.SHA256,
			Bytes: NewByteLength(canonicalBytes + 1),
		}},
		{name: "maximum byte length cannot redefine the canonical document", contract: GovernanceDocumentContract{
			Document: canonical.Document, Path: canonical.Path, SHA256: canonical.SHA256,
			Bytes: NewByteLength(math.MaxUint64),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireExactGovernanceIdentity(t, tc.contract.Validate(), ErrGovernanceContract)
		})
	}
}

// TestGovernanceDocumentEnumDomainIsClosed exhausts the whole uint8 domain so a
// newly added identity cannot ship without a canonical contract behind it.
func TestGovernanceDocumentEnumDomainIsClosed(t *testing.T) {
	t.Parallel()

	canonical := canonicalGovernanceContractForTest(t)

	for raw := range math.MaxUint8 + 1 {
		document := GovernanceDocument(raw)
		gotErr := document.Validate()
		gotContract, gotContractErr := document.Contract()
		if document == GovernanceDocumentTestingProtocol {
			if gotErr != nil {
				t.Fatalf("GovernanceDocument(%d).Validate() error = %v, want nil", raw, gotErr)
			}
			if gotContractErr != nil || gotContract != canonical {
				t.Fatalf(
					"GovernanceDocument(%d).Contract() = (%+v, %v), want (%+v, nil)",
					raw, gotContract, gotContractErr, canonical,
				)
			}
			continue
		}
		if !errors.Is(gotErr, ErrGovernanceContract) {
			t.Fatalf("GovernanceDocument(%d).Validate() error = %v, want %v", raw, gotErr, ErrGovernanceContract)
		}
		if gotContract != (GovernanceDocumentContract{}) || !errors.Is(gotContractErr, ErrGovernanceContract) {
			t.Fatalf(
				"GovernanceDocument(%d).Contract() = (%+v, %v), want (zero, %v)",
				raw, gotContract, gotContractErr, ErrGovernanceContract,
			)
		}
	}
}

// TestGovernanceDocumentContractObservationIsStable pins the canonical value
// across repeated observation so a future memoization or lazy-parse change
// cannot leak a mutated contract to a later caller.
func TestGovernanceDocumentContractObservationIsStable(t *testing.T) {
	t.Parallel()

	first, firstErr := GovernanceDocumentTestingProtocol.Contract()
	second, secondErr := GovernanceDocumentTestingProtocol.Contract()
	if firstErr != nil || secondErr != nil {
		t.Fatalf("repeated Contract() errors = (%v, %v), want (nil, nil)", firstErr, secondErr)
	}
	if second != first {
		t.Fatalf("repeated Contract() = %+v, want unchanged %+v", second, first)
	}
	// Mutating a returned copy must not reach the canonical source.
	tampered := first
	tampered.Bytes = NewByteLength(0)
	third, thirdErr := GovernanceDocumentTestingProtocol.Contract()
	if thirdErr != nil || third != first {
		t.Fatalf("Contract() after caller mutation = (%+v, %v), want (%+v, nil)", third, thirdErr, first)
	}
}
