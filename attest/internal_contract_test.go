package attest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

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
