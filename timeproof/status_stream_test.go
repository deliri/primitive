package timeproof

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestStatusTextAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	status, err := RefusalStatusRejection.rfcValue()
	if err != nil {
		t.Fatalf("RefusalStatus.rfcValue() error = %v, want nil", err)
	}
	many := make([]string, 1024)
	for index := range many {
		many[index] = "refused"
	}
	cases := []struct {
		name    string
		text    []byte
		wantErr error
	}{
		{name: "one UTF8 status text", text: encodeStatusText(t, "refused"), wantErr: core.ErrTimeProofRefused},
		{name: "one empty string still supplies one entry", text: encodeStatusText(t, ""), wantErr: core.ErrTimeProofRefused},
		{name: "Unicode status text", text: encodeStatusText(t, "拒否 🌍"), wantErr: core.ErrTimeProofRefused},
		{name: "ninth text is not a protocol violation", text: encodeStatusText(t, many[:9]...), wantErr: core.ErrTimeProofRefused},
		{name: "thousand text entries remain typed refusal", text: encodeStatusText(t, many...), wantErr: core.ErrTimeProofRefused},
		{name: "absent optional status text", wantErr: core.ErrTimeProofRefused},
		{name: "present empty sequence is malformed", text: encodeStatusText(t), wantErr: core.ErrTimeProofInvalid},
		{name: "invalid UTF8 byte", text: encodeSequence(derTagged(byte(asn1.TagUTF8String), []byte{0xff})), wantErr: core.ErrTimeProofInvalid},
		{name: "truncated UTF8 rune", text: encodeSequence(derTagged(byte(asn1.TagUTF8String), []byte{0xe2, 0x82})), wantErr: core.ErrTimeProofInvalid},
		{name: "overlong UTF8 encoding", text: encodeSequence(derTagged(byte(asn1.TagUTF8String), []byte{0xc0, 0x80})), wantErr: core.ErrTimeProofInvalid},
		{name: "UTF8 surrogate encoding", text: encodeSequence(derTagged(byte(asn1.TagUTF8String), []byte{0xed, 0xa0, 0x80})), wantErr: core.ErrTimeProofInvalid},
		{name: "non UTF8 ASN1 string type", text: encodeSequence(derTagged(byte(asn1.TagPrintableString), []byte("refused"))), wantErr: core.ErrTimeProofInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			response := encodeSequence(encodeSequence(encodeStatusInteger(t, status), tc.text))
			got, gotErr := Verify(VerifyRequest{Response: response, Request: fixture.request, ExpectedDigest: fixture.digest})
			if !errors.Is(gotErr, tc.wantErr) || !timestampHasNoProof(got) {
				t.Fatalf("Verify(status text) = (%+v, %v), want zero and %v", got, gotErr, tc.wantErr)
			}
			if errors.Is(gotErr, core.ErrTimeProofRefused) {
				var refusal Refusal
				if !errors.As(gotErr, &refusal) || refusal.Status() != RefusalStatusRejection || len(refusal.Codes()) != 0 {
					t.Fatalf("typed status refusal = %+v, want rejection without failure codes", refusal)
				}
			}
		})
	}
}

func TestDERHandoffsBorrowExactCallerBytes(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	token, conclusion, err := parseTimestampResponse(fixture.response)
	if err != nil || !conclusion.status.granted() || len(token) == 0 {
		t.Fatalf("parseTimestampResponse() = (%d bytes, %+v, %v), want nonempty granted token", len(token), conclusion, err)
	}
	contentType, explicit, err := parseContentInfo(token)
	if err != nil || !contentType.Equal(oidSignedData()) {
		t.Fatalf("parseContentInfo() = (%v, %v), want signed-data", contentType, err)
	}
	signed, err := parseSignedData(explicit)
	if err != nil {
		t.Fatalf("parseSignedData() error = %v, want nil", err)
	}
	tst, err := explicitOctets(signed.Content.Content)
	if err != nil || len(tst) == 0 {
		t.Fatalf("explicitOctets() = (%d bytes, %v), want nonempty and nil", len(tst), err)
	}
	cases := []struct {
		name  string
		value []byte
	}{
		{name: "CMS token source span", value: token},
		{name: "TSTInfo source span", value: tst},
		{name: "signature source span", value: signed.Signers[0].Signature},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			index := bytes.Index(fixture.response, tc.value)
			if len(tc.value) == 0 || index < 0 || &tc.value[0] != &fixture.response[index] {
				t.Fatalf("DER source offset/size = %d/%d, want a nonempty borrowed source span", index, len(tc.value))
			}
		})
	}

}

func BenchmarkValidateStatusText(b *testing.B) {
	b.ReportAllocs()
	for _, size := range []int{1024, 1 << 20} {
		b.Run("bytes_"+strconv.Itoa(size), func(b *testing.B) {
			value := strings.Repeat("x", size)
			encoded, err := asn1.MarshalWithParams(value, "utf8")
			if err != nil {
				b.Fatalf("ASN1 workload error = %v, want nil", err)
			}
			if err := validateStatusText(encoded); err != nil {
				b.Fatalf("status workload validation error = %v, want nil", err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				if err := validateStatusText(encoded); err != nil {
					b.Fatalf("validateStatusText() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestBorrowedDERDoesNotEscapeVerifiedCustody(t *testing.T) {
	t.Parallel()
	fixture := loadAuthenticFixture(t)
	response := bytes.Clone(fixture.response)
	got, err := Verify(VerifyRequest{Response: response, Request: fixture.request, ExpectedDigest: fixture.digest})
	if err != nil {
		t.Fatalf("Verify(owned source) error = %v, want nil", err)
	}
	clear(response)
	if !sameTimeproofEvidence(got.Evidence(), fixture.evidence) {
		t.Fatalf("retained response identity = %+v, want original identity after source mutation", got.Evidence())
	}
	encoded, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("verified MarshalJSON() error = %v, want nil after accessor mutation", err)
	}
	var replayed AuthoritativeTimestamp
	if err := replayed.Restore(RestoreRequest{Document: encoded, Response: fixture.response, ExpectedDigest: fixture.digest}); err != nil || replayed.Signer() != got.Signer() || !sameTimeproofEvidence(replayed.Evidence(), fixture.evidence) {
		t.Fatalf("verified replay = (%+v, %v), want independent authentic custody", replayed, err)
	}
}
