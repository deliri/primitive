package receipt

import (
	"bytes"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestWatermarkScopeStrictJSONHostileMatrix presses the nested scope directly.
// The scope reaches durable bytes only inside a watermark, so a defect in its
// own strict decode is otherwise visible only when the outer decode happens to
// exercise it.
func TestWatermarkScopeStrictJSONHostileMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 230)
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	canonical, err := json.Marshal(scope)
	if err != nil {
		t.Fatalf("json.Marshal(Scope) error = %v, want nil", err)
	}
	if len(canonical) > scopeCanonicalJSONMaximumBytes {
		t.Fatalf("canonical scope extent = %d, want at most %d",
			len(canonical), scopeCanonicalJSONMaximumBytes)
	}

	var decoded Scope
	if err := json.Unmarshal(canonical, &decoded); err != nil || decoded != scope {
		t.Fatalf("json.Unmarshal(canonical scope) = (%v, %v), want exact scope and nil", decoded, err)
	}
	reordered, err := json.Marshal(struct {
		Offering *core.Offering   `json:"offering"`
		Account  *AccountIdentity `json:"account_identity"`
	}{Offering: &fixture.offering, Account: &fixture.account})
	if err != nil {
		t.Fatalf("json.Marshal(reordered scope) error = %v, want nil", err)
	}
	for _, data := range [][]byte{
		canonical,
		reordered,
		append(append([]byte(" \n\t"), canonical...), '\r'),
	} {
		var got Scope
		if gotErr := got.UnmarshalJSON(data); gotErr != nil || got != scope {
			t.Fatalf("Scope.UnmarshalJSON(%q) = (%v, %v), want exact scope and nil", data, got, gotErr)
		}
	}

	hostileCases := []struct {
		name string
		data []byte
	}{
		{name: "empty input is rejected", data: nil},
		{name: "null is rejected", data: []byte("null")},
		{name: "truncation is rejected", data: canonical[:len(canonical)-1]},
		{name: "trailing document is rejected", data: append(append([]byte{}, canonical...), []byte("{}")...)},
		{name: "wrong top-level type is rejected", data: []byte("[]")},
		{name: "unknown member is rejected", data: bytes.Replace(canonical, []byte(`"account_identity":`), []byte(`"unknown":0,"account_identity":`), 1)},
		{name: "duplicate account is rejected", data: bytes.Replace(canonical, []byte(`"account_identity":`), []byte(`"account_identity":"`+fixture.account.String()+`","account_identity":`), 1)},
		{name: "missing offering is rejected", data: []byte(`{"account_identity":"` + fixture.account.String() + `"}`)},
		{name: "missing account is rejected", data: []byte(`{"offering":"` + fixture.offering.String() + `"}`)},
		{name: "zero account text is rejected", data: []byte(`{"account_identity":"` + strings.Repeat("0", LifecycleIdentityHexBytes) + `","offering":"` + fixture.offering.String() + `"}`)},
		{name: "uppercase account hex is rejected", data: []byte(`{"account_identity":"` + strings.ToUpper(fixture.account.String()) + `","offering":"` + fixture.offering.String() + `"}`)},
		{name: "account of the wrong width is rejected", data: []byte(`{"account_identity":"00","offering":"` + fixture.offering.String() + `"}`)},
		{name: "numeric account is rejected", data: []byte(`{"account_identity":1,"offering":"` + fixture.offering.String() + `"}`)},
		{name: "invalid UTF-8 is rejected", data: []byte{'{', '"', 0xff, '"', ':', '1', '}'}},
		{name: "nesting bomb is rejected", data: []byte(`{"account_identity":{"nested":{"deeper":{}}}}`)},
		{name: "one above byte bound is rejected", data: bytes.Repeat([]byte{' '}, scopeCanonicalJSONMaximumBytes+scopeJSONWhitespaceAllowance+1)},
	}
	for _, tc := range hostileCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := scope
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || receiver != scope {
				t.Fatalf("Scope.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and JSON rejection",
					tc.data, receiver, gotErr)
			}
		})
	}

	if gotErr := (*Scope)(nil).UnmarshalJSON(canonical); !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("nil Scope.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
	}
	for _, incomplete := range []Scope{
		{}, {Account: fixture.account}, {Offering: fixture.offering},
	} {
		if _, gotErr := incomplete.MarshalJSON(); !errors.Is(gotErr, core.ErrReceiptContract) {
			t.Fatalf("incomplete Scope.MarshalJSON() error = %v, want %v",
				gotErr, core.ErrReceiptContract)
		}
	}
}

// TestWatermarkNominalConstructorsAndValidationMatrix covers every rejection
// branch of the nominal closures and of Watermark's own validation, so a
// removed guard cannot hide behind an unrelated green field.
func TestWatermarkNominalConstructorsAndValidationMatrix(t *testing.T) {
	t.Parallel()

	fixture := newReceiptFixture(t, 61)
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	complete := watermarkFixture(t, scope, 4, "validation")

	if _, gotErr := NewCursorDigest(core.SHA256Digest{}); !errors.Is(gotErr, core.ErrReceiptContract) {
		t.Fatalf("NewCursorDigest(unset) error = %v, want %v", gotErr, core.ErrReceiptContract)
	}
	if _, gotErr := NewChainHash(core.SHA256Digest{}); !errors.Is(gotErr, core.ErrReceiptContract) {
		t.Fatalf("NewChainHash(unset) error = %v, want %v", gotErr, core.ErrReceiptContract)
	}
	zeroDigest, err := NewCursorDigest(core.NewSHA256Digest([sha256.Size]byte{}))
	if err != nil {
		t.Fatalf("NewCursorDigest(all-zero digest) error = %v, want nil", err)
	}
	if gotErr := zeroDigest.Validate(); gotErr != nil {
		t.Fatalf("CursorDigest(all-zero digest).Validate() error = %v, want nil", gotErr)
	}

	from := func(mutate func(*Watermark)) Watermark {
		got := complete
		mutate(&got)
		return got
	}
	cases := []struct {
		wantErr   error
		name      string
		watermark Watermark
	}{
		{name: "complete watermark is admitted", watermark: complete},
		{name: "wholly unset watermark is rejected", watermark: Watermark{}, wantErr: core.ErrReceiptContract},
		{name: "unset revision is rejected", watermark: from(func(v *Watermark) { v.Revision = RevisionUnknown }), wantErr: core.ErrReceiptContract},
		{name: "future revision is rejected", watermark: from(func(v *Watermark) { v.Revision = Revision(math.MaxUint8) }), wantErr: core.ErrReceiptContract},
		{name: "unset scope account is rejected", watermark: from(func(v *Watermark) { v.Scope.Account = AccountIdentity{} }), wantErr: core.ErrReceiptContract},
		{name: "unset scope offering is rejected", watermark: from(func(v *Watermark) { v.Scope.Offering = core.Offering{} }), wantErr: core.ErrReceiptContract},
		{name: "unset generation is rejected", watermark: from(func(v *Watermark) { v.Generation = Generation{} }), wantErr: core.ErrReceiptContract},
		{name: "unset cursor digest is rejected", watermark: from(func(v *Watermark) { v.CursorDigest = CursorDigest{} }), wantErr: core.ErrReceiptContract},
		{name: "unset chain hash is rejected", watermark: from(func(v *Watermark) { v.ChainHash = ChainHash{} }), wantErr: core.ErrReceiptContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if gotErr := tc.watermark.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Watermark.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}

	requestCases := []struct {
		wantErr error
		name    string
		request WatermarkRequest
	}{
		{name: "complete request closes a watermark", request: WatermarkRequest{Scope: scope, Generation: complete.Generation, CursorDigest: complete.CursorDigest, ChainHash: complete.ChainHash}},
		{name: "unset scope is rejected", request: WatermarkRequest{Generation: complete.Generation, CursorDigest: complete.CursorDigest, ChainHash: complete.ChainHash}, wantErr: core.ErrReceiptContract},
		{name: "unset generation is rejected", request: WatermarkRequest{Scope: scope, CursorDigest: complete.CursorDigest, ChainHash: complete.ChainHash}, wantErr: core.ErrReceiptContract},
		{name: "unset cursor is rejected", request: WatermarkRequest{Scope: scope, Generation: complete.Generation, ChainHash: complete.ChainHash}, wantErr: core.ErrReceiptContract},
		{name: "unset chain is rejected", request: WatermarkRequest{Scope: scope, Generation: complete.Generation, CursorDigest: complete.CursorDigest}, wantErr: core.ErrReceiptContract},
		{name: "wholly unset request is rejected", request: WatermarkRequest{}, wantErr: core.ErrReceiptContract},
	}
	for _, tc := range requestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := NewWatermark(tc.request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewWatermark() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && got != (Watermark{}) {
				t.Fatalf("NewWatermark(rejected) result = %v, want zero", got)
			}
		})
	}
}

// TestSealedRejectionDiagnosticsAreStableAndNonSensitive proves both typed
// rejections render the stable Core text and nothing else. A diagnostic that
// leaked the differing value would turn an authentication failure into an
// oracle for the caller's expected scope.
func TestSealedRejectionDiagnosticsAreStableAndNonSensitive(t *testing.T) {
	t.Parallel()

	cases := []struct {
		err  error
		name string
		want string
	}{
		{name: "scope mismatch", err: newScopeMismatch(ScopeFieldAccount), want: core.ErrReceiptScope.Error()},
		{name: "watermark conflict", err: conflictError(ConflictReasonCursorUnchanged), want: core.ErrReceiptConflict.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}
