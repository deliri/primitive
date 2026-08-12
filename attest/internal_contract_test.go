package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func scanAttestExternalJSONReceivers(root string) ([]string, error) {
	set := token.NewFileSet()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var receivers []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(
			set,
			filepath.Join(root, entry.Name()),
			nil,
			parser.SkipObjectResolution,
		)
		if parseErr != nil {
			return nil, parseErr
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.FuncDecl)
			if !ok || declaration.Name.Name != "UnmarshalJSON" || declaration.Recv == nil {
				return true
			}
			receiver := attestReceiverName(declaration.Recv.List[0].Type)
			if ast.IsExported(receiver) {
				receivers = append(receivers, receiver)
			}
			return false
		})
	}
	slices.Sort(receivers)
	return receivers, nil
}

func attestReceiverName(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.StarExpr:
		return attestReceiverName(value.X)
	case *ast.Ident:
		return value.Name
	case *ast.IndexExpr:
		return attestReceiverName(value.X)
	case *ast.IndexListExpr:
		return attestReceiverName(value.X)
	default:
		return ""
	}
}

func TestDomainTokenInternalFixedStorageBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup   func(testing.TB) domainToken
		wantErr error
		name    string
	}{
		{name: "one byte canonical token validates", setup: domainTokenFixture("a")},
		{name: "maximum canonical token validates", setup: domainTokenFixture(strings.Repeat("a", SigningDomainMaximumBytes))},
		{name: "zero token rejects", setup: func(testing.TB) domainToken { return domainToken{} }, wantErr: core.ErrAttestContract},
		{name: "negative length rejects", setup: forgedDomainTokenLengthFixture(-1), wantErr: core.ErrAttestContract},
		{name: "maximum plus one length rejects", setup: forgedDomainTokenLengthFixture(SigningDomainMaximumBytes + 1), wantErr: core.ErrAttestContract},
		{name: "leading hyphen storage rejects", setup: forgedDomainTokenTextFixture("-a"), wantErr: core.ErrAttestContract},
		{name: "trailing hyphen storage rejects", setup: forgedDomainTokenTextFixture("a-"), wantErr: core.ErrAttestContract},
		{name: "adjacent hyphen storage rejects", setup: forgedDomainTokenTextFixture("a--b"), wantErr: core.ErrAttestContract},
		{name: "uppercase storage rejects", setup: forgedDomainTokenTextFixture("A"), wantErr: core.ErrAttestContract},
		{name: "nonzero trailing storage rejects", setup: forgedDomainTokenTrailingFixture, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.setup(t).Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("domainToken.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestTrustedKeysInternalFixedStorageBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup   func(testing.TB) TrustedKeys
		wantErr error
		name    string
	}{
		{name: "one key closed storage validates", setup: internalTrustedKeysFixture},
		{name: "zero storage rejects", setup: func(testing.TB) TrustedKeys { return TrustedKeys{} }, wantErr: core.ErrAttestContract},
		{name: "negative count rejects", setup: forgedTrustedCountFixture(-1), wantErr: core.ErrAttestContract},
		{name: "maximum plus one count rejects", setup: forgedTrustedCountFixture(TrustedKeyMaximumCount + 1), wantErr: core.ErrAttestContract},
		{name: "zero key inside count rejects", setup: forgedTrustedZeroKeyFixture, wantErr: core.ErrAttestContract},
		{name: "duplicate keys inside count reject", setup: forgedTrustedDuplicateFixture, wantErr: core.ErrAttestContract},
		{name: "populated trailing storage rejects", setup: forgedTrustedTrailingFixture, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.setup(t).Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("TrustedKeys.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestCheckedUint16FromIntInternalCompleteBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		input   int
		want    uint16
	}{
		{name: "minimum int rejects", input: math.MinInt, wantErr: core.ErrAttestContract},
		{name: "one below zero rejects", input: -1, wantErr: core.ErrAttestContract},
		{name: "zero converts", input: 0, want: 0},
		{name: "one converts", input: 1, want: 1},
		{name: "maximum minus one converts", input: math.MaxUint16 - 1, want: math.MaxUint16 - 1},
		{name: "exact maximum converts", input: math.MaxUint16, want: math.MaxUint16},
		{name: "maximum plus one rejects", input: math.MaxUint16 + 1, wantErr: core.ErrAttestContract},
		{name: "maximum int rejects", input: math.MaxInt, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := checkedUint16FromInt(tc.input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("checkedUint16FromInt() error = %v, want %v", gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("checkedUint16FromInt() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestAttestationFrameInternalFixedExtentBoundaryMatrix ratchets the signed
// frame against its fixed backing array. The declared extent must be exactly
// the extent a maximum-domain frame occupies: a constant that drifts low makes
// append reallocate off the array and bytes slice out of range, and a constant
// that drifts high stops proving the layout is closed. It also proves an
// unusable token yields no frame rather than one silently bound to an empty
// domain, which would collapse domain separation.
func TestAttestationFrameInternalFixedExtentBoundaryMatrix(t *testing.T) {
	t.Parallel()

	signer := internalPublicKeyFixture(t, "internal-frame-signer")
	maximumText := strings.Repeat("a", SigningDomainMaximumBytes)
	cases := []struct {
		setup      func(testing.TB) canonicalFacts[internalTestDomain]
		wantErr    error
		name       string
		wantExtent int
	}{
		{
			name:       "maximum domain fills the declared extent exactly",
			setup:      internalCanonicalFactsFixture(maximumText),
			wantExtent: attestationFrameMaximum,
		},
		{
			name:       "maximum minus one domain is one byte shorter",
			setup:      internalCanonicalFactsFixture(maximumText[:SigningDomainMaximumBytes-1]),
			wantExtent: attestationFrameMaximum - 1,
		},
		{
			name:       "one byte domain leaves the domain slack unused",
			setup:      internalCanonicalFactsFixture("a"),
			wantExtent: attestationFrameMaximum - SigningDomainMaximumBytes + 1,
		},
		{
			name:    "unusable token yields no frame",
			setup:   internalForgedTokenFactsFixture,
			wantErr: core.ErrAttestContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotFrame, gotErr := newAttestationFrame(tc.setup(t), signer)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("newAttestationFrame() error = %v, want %v", gotErr, tc.wantErr)
			}
			if got := len(gotFrame.bytes()); got != tc.wantExtent {
				t.Fatalf("len(attestationFrame.bytes()) = %d, want %d", got, tc.wantExtent)
			}
		})
	}
}

type internalTestDomain struct {
	text string
}

func (d internalTestDomain) Validate() error {
	if !validDomainText([]byte(d.text)) {
		return contractError(errors.New(domainCanonicalErrorText))
	}
	return nil
}

func (d internalTestDomain) MarshalText() ([]byte, error) {
	return []byte(d.text), nil
}

func (internalTestDomain) ParseCanonicalText(text []byte) (internalTestDomain, error) {
	return internalTestDomain{text: string(text)}, nil
}

// TestCanonicalDomainRoundTripEnforcesSelfReference drives the real generic
// domain seam rather than assembling canonicalFacts by hand. canonicalDomain
// projects a domain to its token through Validate and MarshalText, and
// parseCanonicalDomain reconstructs it through ParseCanonicalText and then
// re-projects it, refusing any domain whose reconstruction does not reproduce
// the exact token. That re-projection is the whole point of the self-referential
// SigningDomain constraint: without it a protocol owner could sign under one
// domain text and verify under another. The table pressures both sides of the
// admitted alphabet and both length boundaries.
func TestCanonicalDomainRoundTripEnforcesSelfReference(t *testing.T) {
	t.Parallel()

	maximumText := strings.Repeat("a", SigningDomainMaximumBytes)
	cases := []struct {
		wantErr   error
		name      string
		text      string
		wantCanon bool
	}{
		{name: "single admitted byte is the shortest canonical domain", text: "a", wantCanon: true},
		{name: "single admitted digit is canonical", text: "0", wantCanon: true},
		{name: "interior hyphen is canonical", text: "a-b", wantCanon: true},
		{name: "multiple separated hyphens are canonical", text: "a-b-c-d", wantCanon: true},
		{name: "digits and letters mix canonically", text: "sha256-v1", wantCanon: true},
		{name: "exact maximum length is canonical", text: maximumText, wantCanon: true},
		{
			name:    "one byte above maximum length is refused",
			text:    maximumText + "a",
			wantErr: core.ErrAttestContract,
		},
		{name: "empty text is refused", text: "", wantErr: core.ErrAttestContract},
		{name: "leading hyphen is refused", text: "-a", wantErr: core.ErrAttestContract},
		{name: "trailing hyphen is refused", text: "a-", wantErr: core.ErrAttestContract},
		{name: "consecutive hyphens are refused", text: "a--b", wantErr: core.ErrAttestContract},
		{name: "lone hyphen is refused", text: "-", wantErr: core.ErrAttestContract},
		{name: "uppercase byte is outside the admitted alphabet", text: "A", wantErr: core.ErrAttestContract},
		{name: "underscore is outside the admitted alphabet", text: "a_b", wantErr: core.ErrAttestContract},
		{name: "dot is outside the admitted alphabet", text: "a.b", wantErr: core.ErrAttestContract},
		{name: "space is outside the admitted alphabet", text: "a b", wantErr: core.ErrAttestContract},
		{name: "interior NUL is outside the admitted alphabet", text: "a\x00b", wantErr: core.ErrAttestContract},
		{name: "non-ASCII byte is outside the admitted alphabet", text: "aéb", wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			domain := internalTestDomain{text: tc.text}
			token, gotErr := canonicalDomain(domain)
			if !tc.wantCanon {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("canonicalDomain(%q) error = %v, want %v", tc.text, gotErr, tc.wantErr)
				}
				if token != (domainToken{}) {
					t.Fatalf("canonicalDomain(%q) token = %v, want the zero token", tc.text, token)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("canonicalDomain(%q) error = %v, want nil", tc.text, gotErr)
			}
			if got := string(token.bytes()); got != tc.text {
				t.Fatalf("canonicalDomain(%q) token bytes = %q, want %q", tc.text, got, tc.text)
			}

			got, err := parseCanonicalDomain[internalTestDomain](token)
			if err != nil {
				t.Fatalf("parseCanonicalDomain(%q) error = %v, want nil", tc.text, err)
			}
			if got != domain {
				t.Fatalf("parseCanonicalDomain(%q) = %v, want %v", tc.text, got, domain)
			}

			// The reconstructed domain must re-project to the identical token,
			// byte for byte and padding included, or a verifier could accept a
			// domain the signer never committed to.
			reprojected, err := canonicalDomain(got)
			if err != nil {
				t.Fatalf("canonicalDomain(reconstructed %q) error = %v, want nil", tc.text, err)
			}
			if reprojected != token {
				t.Fatalf("re-projected token for %q = %v, want %v", tc.text, reprojected, token)
			}
		})
	}
}

// TestParseCanonicalDomainRefusesUnfaithfulReconstruction proves the
// re-projection guard is load-bearing rather than decorative. This domain's
// ParseCanonicalText deliberately reconstructs a different value than the token
// names, which is exactly the mistake the self-referential constraint exists to
// catch. Deleting the projected-token comparison in parseCanonicalDomain turns
// this red.
func TestParseCanonicalDomainRefusesUnfaithfulReconstruction(t *testing.T) {
	t.Parallel()

	token, err := canonicalDomain(unfaithfulTestDomain{text: "signed"})
	if err != nil {
		t.Fatalf("canonicalDomain() error = %v, want nil", err)
	}

	got, gotErr := parseCanonicalDomain[unfaithfulTestDomain](token)
	if !errors.Is(gotErr, core.ErrAttestContract) {
		t.Fatalf("parseCanonicalDomain(unfaithful) error = %v, want %v", gotErr, core.ErrAttestContract)
	}
	if got != (unfaithfulTestDomain{}) {
		t.Fatalf("parseCanonicalDomain(unfaithful) = %v, want the zero domain", got)
	}
}

// unfaithfulTestDomain reconstructs a domain that is individually valid but is
// not the one the canonical text names.
type unfaithfulTestDomain struct {
	text string
}

func (d unfaithfulTestDomain) Validate() error {
	if !validDomainText([]byte(d.text)) {
		return contractError(errors.New(domainCanonicalErrorText))
	}
	return nil
}

func (d unfaithfulTestDomain) MarshalText() ([]byte, error) {
	return []byte(d.text), nil
}

func (unfaithfulTestDomain) ParseCanonicalText([]byte) (unfaithfulTestDomain, error) {
	return unfaithfulTestDomain{text: "substituted"}, nil
}

func internalCanonicalFactsFixture(text string) func(testing.TB) canonicalFacts[internalTestDomain] {
	return func(t testing.TB) canonicalFacts[internalTestDomain] {
		t.Helper()
		token, err := newDomainToken([]byte(text))
		if err != nil {
			t.Fatalf("newDomainToken() error = %v, want nil", err)
		}
		facts := internalForgedTokenFactsFixture(t)
		facts.domain = internalTestDomain{text: text}
		facts.token = token
		return facts
	}
}

// internalForgedTokenFactsFixture carries a valid length and digest with the
// zero token, so the frame's rejection is attributable to the token alone.
func internalForgedTokenFactsFixture(t testing.TB) canonicalFacts[internalTestDomain] {
	t.Helper()
	length, err := core.NewByteCount(CanonicalBodyMaximumBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return canonicalFacts[internalTestDomain]{
		length: length,
		digest: core.NewSHA256Digest(sha256.Sum256([]byte("internal-frame-body"))),
	}
}

func domainTokenFixture(text string) func(testing.TB) domainToken {
	return func(t testing.TB) domainToken {
		t.Helper()
		token, err := newDomainToken([]byte(text))
		if err != nil {
			t.Fatalf("newDomainToken() error = %v, want nil", err)
		}
		return token
	}
}

func forgedDomainTokenLengthFixture(length int) func(testing.TB) domainToken {
	return func(testing.TB) domainToken {
		return domainToken{length: length}
	}
}

func forgedDomainTokenTextFixture(text string) func(testing.TB) domainToken {
	return func(testing.TB) domainToken {
		var token domainToken
		copy(token.text[:], text)
		token.length = len(text)
		return token
	}
}

func forgedDomainTokenTrailingFixture(t testing.TB) domainToken {
	token := forgedDomainTokenTextFixture("a")(t)
	token.text[1] = 'x'
	return token
}

func internalTrustedKeysFixture(t testing.TB) TrustedKeys {
	t.Helper()
	key := internalPublicKeyFixture(t, "internal-trusted")
	return TrustedKeys{keys: [TrustedKeyMaximumCount]core.Ed25519PublicKey{key}, count: 1}
}

func forgedTrustedCountFixture(count int) func(testing.TB) TrustedKeys {
	return func(testing.TB) TrustedKeys {
		return TrustedKeys{count: count}
	}
}

func forgedTrustedZeroKeyFixture(testing.TB) TrustedKeys {
	return TrustedKeys{count: 1}
}

func forgedTrustedDuplicateFixture(t testing.TB) TrustedKeys {
	t.Helper()
	key := internalPublicKeyFixture(t, "internal-duplicate")
	return TrustedKeys{
		keys:  [TrustedKeyMaximumCount]core.Ed25519PublicKey{key, key},
		count: 2,
	}
}

func forgedTrustedTrailingFixture(t testing.TB) TrustedKeys {
	t.Helper()
	first := internalPublicKeyFixture(t, "internal-first")
	trailing := internalPublicKeyFixture(t, "internal-trailing")
	return TrustedKeys{
		keys:  [TrustedKeyMaximumCount]core.Ed25519PublicKey{first, trailing},
		count: 1,
	}
}

func internalPublicKeyFixture(t testing.TB, label string) core.Ed25519PublicKey {
	t.Helper()
	seed := sha256.Sum256([]byte(label))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey, err := core.NewEd25519PublicKey(privateKey.Public().(ed25519.PublicKey))
	clear(privateKey)
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	return publicKey
}
