package timeproof

import (
	"encoding/asn1"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type tstInfoFixture struct {
	required [][]byte
	ordering []byte
	nonce    []byte
	tsa      []byte
}

func loadAuthenticTSTInfoFixture(t testing.TB) tstInfoFixture {
	t.Helper()

	fixture := loadAuthenticFixture(t)
	tokenDER, _, err := parseTimestampResponse(
		fixture.evidence.ResponseBytes(),
	)
	if err != nil {
		t.Fatalf("parseTimestampResponse(authentic) error = %v, want nil", err)
	}
	token, err := parseTimestampToken(tokenDER)
	if err != nil {
		t.Fatalf("parseTimestampToken(authentic) error = %v, want nil", err)
	}
	sequence, err := requireSequence(token.TSTDER)
	if err != nil {
		t.Fatalf("requireSequence(authentic TSTInfo) error = %v, want nil", err)
	}
	fields := sequence.Bytes
	parts := make([][]byte, 0, 8)
	for len(fields) != 0 {
		raw, remaining, parseErr := consumeRaw(fields)
		if parseErr != nil {
			t.Fatalf("consumeRaw(authentic TSTInfo) error = %v, want nil", parseErr)
		}
		parts = append(parts, append([]byte(nil), raw.FullBytes...))
		fields = remaining
	}
	if len(parts) != 8 {
		t.Fatalf("authentic TSTInfo field count = %d, want 8", len(parts))
	}
	return tstInfoFixture{
		required: parts[:5],
		ordering: parts[5],
		nonce:    parts[6],
		tsa:      parts[7],
	}
}

func encodeTSTInfo(parts ...[]byte) []byte {
	length := 0
	for _, part := range parts {
		length += len(part)
	}
	body := make([]byte, 0, length)
	for _, part := range parts {
		body = append(body, part...)
	}
	return derTagged(byte(asn1.TagSequence)|derConstructed, body)
}

func TestTSTInfoOptionalProtocolOrderLayerTriad(t *testing.T) {
	t.Parallel()

	fixture := loadAuthenticTSTInfoFixture(t)
	canonical := append(
		append(append([][]byte(nil), fixture.required...), fixture.ordering),
		fixture.nonce,
		fixture.tsa,
	)
	t.Run("positive authentic ordering nonce and TSA sequence is accepted", func(t *testing.T) {
		t.Parallel()

		got, gotErr := parseTSTInfo(encodeTSTInfo(canonical...))
		if gotErr != nil || got.Nonce == nil {
			t.Fatalf(
				"parseTSTInfo(canonical) nonce/error = (%v, %v), want (set, nil)",
				got.Nonce,
				gotErr,
			)
		}
	})

	falseOrdering, err := asn1.Marshal(false)
	if err != nil {
		t.Fatalf("asn1.Marshal(false) error = %v, want nil", err)
	}
	oneSecond, err := asn1.Marshal(1)
	if err != nil {
		t.Fatalf("asn1.Marshal(1) error = %v, want nil", err)
	}
	accuracy := derTagged(byte(asn1.TagSequence)|derConstructed, oneSecond)
	zeroMillis := derTagged(
		byte(asn1.TagSequence)|derConstructed,
		derTagged(0x80, []byte{0}),
	)
	zeroMicros := derTagged(
		byte(asn1.TagSequence)|derConstructed,
		derTagged(0x81, []byte{0}),
	)
	malformedTSA := derTagged(0xa0, []byte{0x05, 0x00})
	emptyTSA := derTagged(0xa0, nil)
	primitiveTSA := derTagged(0x80, []byte{0})
	unknownExtension := derTagged(0xa1, []byte{0x30, 0x00})
	nullField := []byte{0x05, 0x00}
	cases := []struct {
		name     string
		optional [][]byte
	}{
		{name: "nonce reordered before ordering", optional: [][]byte{fixture.nonce, fixture.ordering, fixture.tsa}},
		{name: "TSA reordered before nonce", optional: [][]byte{fixture.ordering, fixture.tsa, fixture.nonce}},
		{name: "accuracy reordered after ordering", optional: [][]byte{fixture.ordering, accuracy, fixture.nonce, fixture.tsa}},
		{name: "accuracy reordered after nonce", optional: [][]byte{fixture.ordering, fixture.nonce, accuracy, fixture.tsa}},
		{name: "ordering reordered after TSA", optional: [][]byte{fixture.nonce, fixture.tsa, fixture.ordering}},
		{name: "explicit default false ordering is noncanonical", optional: [][]byte{falseOrdering, fixture.nonce, fixture.tsa}},
		{name: "explicit zero millis violates one through 999", optional: [][]byte{zeroMillis, fixture.ordering, fixture.nonce, fixture.tsa}},
		{name: "explicit zero micros violates one through 999", optional: [][]byte{zeroMicros, fixture.ordering, fixture.nonce, fixture.tsa}},
		{name: "malformed TSA null content", optional: [][]byte{fixture.ordering, fixture.nonce, malformedTSA}},
		{name: "empty TSA content", optional: [][]byte{fixture.ordering, fixture.nonce, emptyTSA}},
		{name: "primitive TSA wrapper", optional: [][]byte{fixture.ordering, fixture.nonce, primitiveTSA}},
		{name: "duplicate ordering", optional: [][]byte{fixture.ordering, fixture.ordering, fixture.nonce, fixture.tsa}},
		{name: "duplicate nonce", optional: [][]byte{fixture.ordering, fixture.nonce, fixture.nonce, fixture.tsa}},
		{name: "duplicate TSA", optional: [][]byte{fixture.ordering, fixture.nonce, fixture.tsa, fixture.tsa}},
		{name: "unsupported extensions field", optional: [][]byte{fixture.ordering, fixture.nonce, fixture.tsa, unknownExtension}},
		{name: "unknown universal null field", optional: [][]byte{fixture.ordering, fixture.nonce, fixture.tsa, nullField}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parts := append(append([][]byte(nil), fixture.required...), tc.optional...)
			got, gotErr := parseTSTInfo(encodeTSTInfo(parts...))
			if !errors.Is(gotErr, core.ErrTimeProofInvalid) || got.Nonce != nil {
				t.Fatalf(
					"parseTSTInfo(hostile) nonce/error = (%v, %v), want (nil, %v)",
					got.Nonce,
					gotErr,
					core.ErrTimeProofInvalid,
				)
			}
		})
	}

	t.Run("neutral absent nonce makes no timestamp fact", func(t *testing.T) {
		t.Parallel()

		parts := append(append([][]byte(nil), fixture.required...), fixture.ordering)
		got, gotErr := parseTSTInfo(encodeTSTInfo(parts...))
		if !errors.Is(gotErr, core.ErrTimeProofInvalid) || got.Nonce != nil {
			t.Fatalf(
				"parseTSTInfo(without nonce) nonce/error = (%v, %v), want (nil, %v)",
				got.Nonce,
				gotErr,
				core.ErrTimeProofInvalid,
			)
		}
	})
}
