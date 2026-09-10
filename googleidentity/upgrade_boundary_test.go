package googleidentity

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const googleFormerAudienceBytes = 1000
const googleFormerTokenBytes = 16 << 10
const googleFormerHeaderBytes = 4 << 10
const googleFormerIdentityTextBytes = 1024

func TestGoogleExtentAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		audience string
		command  []byte
		wantErr  error
	}{
		{name: "audience_beyond_former_extent", audience: strings.Repeat("a", googleFormerAudienceBytes+1)},
		{name: "audience_large_unicode", audience: strings.Repeat("界", 4096)},
		{name: "token_beyond_former_extent", command: bytes.Repeat([]byte("a"), googleFormerTokenBytes+1)},
		{name: "command_many_windows_crlf", command: append(bytes.Repeat([]byte("a"), 4*googleFormerTokenBytes), '\r', '\n')},
		{name: "large_audience_invalid_utf8", audience: strings.Repeat("a", googleFormerAudienceBytes) + "\xff", wantErr: core.ErrGoogleIdentityContract},
		{name: "large_token_embedded_space", command: append(bytes.Repeat([]byte("a"), googleFormerTokenBytes), ' ', 'b'), wantErr: core.ErrGoogleIdentityContract},
		{name: "neutral_empty_command", wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.audience != "" {
				got, err := ParseAudience(tc.audience)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("audience error=%v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (Audience{}) {
						t.Fatalf("refused audience=%v, want zero", got)
					}
					return
				}
				if got.String() != tc.audience {
					t.Fatalf("audience length=%d, want exact %d bytes", len(got.String()), len(tc.audience))
				}
				return
			}
			got, err := ParseGoogleCloudCommandOutput(tc.command)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("command error=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Token{}) {
					t.Fatalf("refused command=%v, want zero", got)
				}
				return
			}
			value, err := got.BearerValue()
			raw := strings.TrimSuffix(strings.TrimSuffix(string(tc.command), "\n"), "\r")
			if err != nil || value != bearerPrefix+raw {
				t.Fatalf("command disclosure equal=%t error=%v, want exact admitted bytes", value == bearerPrefix+raw, err)
			}
		})
	}
}
func TestGoogleAccessJSONExtentLayerTriad(t *testing.T) {
	t.Parallel()
	want := googleAccessTokenResponse{AccessToken: googleTestToken, TokenType: googleAccessTokenTypeBearer, ExpiresIn: 300}
	canonical, err := core.MarshalCanonicalJSONDocument(want)
	if err != nil {
		t.Fatal(err)
	}
	gap := bytes.Repeat([]byte(" "), 128<<10)
	for _, tc := range []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "canonical", input: canonical},
		{name: "large_prefix", input: append(bytes.Clone(gap), canonical...)},
		{name: "large_suffix", input: append(bytes.Clone(canonical), gap...)},
		{name: "large_trailing_document", input: append(append(bytes.Clone(canonical), gap...), canonical...), wantErr: core.ErrGoogleIdentityContract},
		{name: "neutral_whitespace_only", input: gap, wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeGoogleAccessTokenResponse(tc.input)
			desired := want
			if tc.wantErr != nil {
				desired = googleAccessTokenResponse{}
			}
			if !errors.Is(err, tc.wantErr) || got != desired {
				t.Fatalf("access response=%v error=%v, want %v and %v", got, err, desired, tc.wantErr)
			}
		})
	}
}
func TestGoogleHeaderExtentLayerTriad(t *testing.T) {
	t.Parallel()
	header := googleCloudJWTHeader{Algorithm: googleCloudSigningAlgorithmRS256, KeyID: verifierTestKeyID}
	data, err := core.MarshalCanonicalJSONDocument(header)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "canonical_header", input: data},
		{name: "header_beyond_former_extent", input: append(bytes.Clone(data), bytes.Repeat([]byte(" "), googleFormerHeaderBytes)...)},
		{name: "large_header_still_rejects_trailing_document", input: append(append(bytes.Clone(data), bytes.Repeat([]byte(" "), googleFormerHeaderBytes)...), data...), wantErr: core.ErrGoogleIdentityContract},
		{name: "neutral_absent_header", wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token := base64.RawURLEncoding.EncodeToString(tc.input) + ".e30.signature"
			if err := validateGoogleCloudJWTHeader(token); !errors.Is(err, tc.wantErr) {
				t.Fatalf("header error=%v, want %v", err, tc.wantErr)
			}
		})
	}
}
func TestGoogleExactAudienceVerificationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		audience string
		foreign  bool
		wantErr  error
	}{
		{name: "ordinary_exact_audience", audience: verifierTestAudience},
		{name: "surrounding_space_is_audience_data", audience: " " + verifierTestAudience + " "},
		{name: "foreign_padded_audience_is_not_trimmed", audience: " " + verifierTestAudience + " ", foreign: true, wantErr: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				p := newVerifierTestProvider(t, nil)
				claims := verifierClaims()
				claims.Audience = tc.audience
				signed := p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, claims, false)
				expectedAudience := tc.audience
				if tc.foreign {
					expectedAudience = verifierTestAudience
				}
				got, err := p.verifier(t, expectedAudience).Verify(t.Context(), signed)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("verification error=%v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != (GoogleCloudVerifiedIdentity{}) || p.calls.Load() != 0 {
						t.Fatalf("refused identity=%v certificate calls=%d, want zero", got, p.calls.Load())
					}
					return
				}
				if got.Audience != tc.audience || got.Subject != claims.Subject || p.calls.Load() != 1 {
					t.Fatalf("identity=%v calls=%d, want exact signed audience and one authority read", got, p.calls.Load())
				}
			})
		})
	}
}

type googleBrokenContext struct{ context.Context }

func TestGoogleServiceAccountContextLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ctx     func(*testing.T) context.Context
		wantErr error
	}{
		{name: "usable_context_preserves_missing_file", ctx: func(t *testing.T) context.Context { return t.Context() }, wantErr: fs.ErrNotExist},
		{name: "absent_context", ctx: func(*testing.T) context.Context { return nil }, wantErr: core.ErrNilContext},
		{name: "typed_nil_context", ctx: func(*testing.T) context.Context { return (*googleBrokenContext)(nil) }, wantErr: core.ErrContextObservation},
		{name: "cancelled_context_before_file_open", ctx: func(t *testing.T) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			client, err := exchange.NewStandardClient()
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseAbsolutePath(filepath.Join(dir, "absent-credential.json"))
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewServiceAccountSource(client, path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := source.Acquire(tc.ctx(t), serviceAccountFixtureRequest(t))
			if !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrGoogleIdentityContract) || got != (Token{}) {
				t.Fatalf("service account token=%v error=%v, want zero and %v", got, err, tc.wantErr)
			}
		})
	}
}
