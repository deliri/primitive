package timeproof

import (
	"encoding/asn1"
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// authenticTokenDER returns the exact granted token the authentic response
// carries, so refusal surgery can prove the token/status exclusivity gate with
// real bytes rather than a fabricated payload.
func authenticTokenDER(t testing.TB) []byte {
	t.Helper()

	fixture := loadAuthenticFixture(t)
	tokenDER, conclusion, err := parseTimestampResponse(fixture.response)
	if err != nil {
		t.Fatalf("parseTimestampResponse(authentic) error = %v, want nil", err)
	}
	if conclusion.status != RefusalStatusGranted {
		t.Fatalf(
			"authentic conclusion status = %v, want %v",
			conclusion.status,
			RefusalStatusGranted,
		)
	}
	return tokenDER
}

func encodeStatusInteger(t testing.TB, value int) []byte {
	t.Helper()

	encoded, err := asn1.Marshal(value)
	if err != nil {
		t.Fatalf("asn1.Marshal(status %d) error = %v, want nil", value, err)
	}
	return encoded
}

// encodeFailInfo builds a PKIFailureInfo BIT STRING with an explicit declared
// bit length, so tests can attack the length boundary independently of content.
func encodeFailInfo(t testing.TB, bitLength int, bits ...int) []byte {
	t.Helper()

	raw := make([]byte, (bitLength+7)/8)
	for _, bit := range bits {
		if bit >= bitLength {
			t.Fatalf("failInfo bit %d, want below declared length %d", bit, bitLength)
		}
		raw[bit/8] |= 0x80 >> (bit % 8)
	}
	encoded, err := asn1.Marshal(asn1.BitString{Bytes: raw, BitLength: bitLength})
	if err != nil {
		t.Fatalf("asn1.Marshal(failInfo) error = %v, want nil", err)
	}
	return encoded
}

func encodeStatusText(t testing.TB, values ...string) []byte {
	t.Helper()

	body := make([]byte, 0, len(values))
	for _, value := range values {
		encoded, err := asn1.MarshalWithParams(value, "utf8")
		if err != nil {
			t.Fatalf("asn1.MarshalWithParams(%q) error = %v, want nil", value, err)
		}
		body = append(body, encoded...)
	}
	return derTagged(byte(asn1.TagSequence)|derConstructed, body)
}

func encodeSequence(parts ...[]byte) []byte {
	body := make([]byte, 0, len(parts))
	for _, part := range parts {
		body = append(body, part...)
	}
	return derTagged(byte(asn1.TagSequence)|derConstructed, body)
}

// verifyCraftedResponse drives the real Verify path with the authentic request
// binding and one crafted TimeStampResp.
func verifyCraftedResponse(t testing.TB, response []byte) error {
	t.Helper()

	fixture := loadAuthenticFixture(t)
	_, err := Verify(VerifyRequest{
		Response: response, Request: fixture.request,
		ExpectedDigest: fixture.digest,
	})
	return err
}

func TestAuthorityRefusalLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every non-granting status reaches the caller typed", func(t *testing.T) {
		t.Parallel()

		for status := RefusalStatusGranted; status < refusalStatusLimit; status++ {
			if status.granted() {
				continue
			}
			value, err := status.rfcValue()
			if err != nil {
				t.Fatalf("RefusalStatus(%v).rfcValue() error = %v, want nil", status, err)
			}
			response := encodeSequence(
				encodeSequence(encodeStatusInteger(t, value)),
			)
			gotErr := verifyCraftedResponse(t, response)
			var refusal Refusal
			if !errors.As(gotErr, &refusal) {
				t.Fatalf(
					"Verify(status %v) error = %v, want errors.As Refusal",
					status,
					gotErr,
				)
			}
			if refusal.Status() != status || len(refusal.Codes()) != 0 {
				t.Fatalf(
					"Verify(status %v) refusal status/codes = (%v, %v), want (%v, none)",
					status,
					refusal.Status(),
					refusal.Codes(),
					status,
				)
			}
			if !errors.Is(gotErr, core.ErrTimeProofRefused) ||
				!errors.Is(gotErr, core.ErrTimeProofContract) {
				t.Fatalf(
					"Verify(status %v) error = %v, want %v under %v",
					status,
					gotErr,
					core.ErrTimeProofRefused,
					core.ErrTimeProofContract,
				)
			}
		}
	})

	t.Run("positive every failure code reaches the caller typed", func(t *testing.T) {
		t.Parallel()

		rejection, err := RefusalStatusRejection.rfcValue()
		if err != nil {
			t.Fatalf("RefusalStatusRejection.rfcValue() error = %v, want nil", err)
		}
		maximumBit, err := refusalMaximumRFCBit()
		if err != nil {
			t.Fatalf("refusalMaximumRFCBit() error = %v, want nil", err)
		}
		for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
			bit, bitErr := code.rfcBit()
			if bitErr != nil {
				t.Fatalf("RefusalCode(%v).rfcBit() error = %v, want nil", code, bitErr)
			}
			response := encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, int(maximumBit)+1, int(bit)),
			))
			gotErr := verifyCraftedResponse(t, response)
			var refusal Refusal
			if !errors.As(gotErr, &refusal) {
				t.Fatalf(
					"Verify(code %v) error = %v, want errors.As Refusal",
					code,
					gotErr,
				)
			}
			if !slices.Equal(refusal.Codes(), []RefusalCode{code}) {
				t.Fatalf(
					"Verify(code %v) refusal codes = %v, want [%v]",
					code,
					refusal.Codes(),
					code,
				)
			}
		}
	})

	t.Run("positive every code set together projects ascending enum order", func(t *testing.T) {
		t.Parallel()

		rejection, err := RefusalStatusRejection.rfcValue()
		if err != nil {
			t.Fatalf("RefusalStatusRejection.rfcValue() error = %v, want nil", err)
		}
		maximumBit, err := refusalMaximumRFCBit()
		if err != nil {
			t.Fatalf("refusalMaximumRFCBit() error = %v, want nil", err)
		}
		bits := make([]int, 0, refusalMaximumCodeCount)
		want := make([]RefusalCode, 0, refusalMaximumCodeCount)
		for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
			bit, bitErr := code.rfcBit()
			if bitErr != nil {
				t.Fatalf("RefusalCode(%v).rfcBit() error = %v, want nil", code, bitErr)
			}
			bits = append(bits, int(bit))
			want = append(want, code)
		}
		response := encodeSequence(encodeSequence(
			encodeStatusInteger(t, rejection),
			encodeStatusText(t, "policy refused"),
			encodeFailInfo(t, int(maximumBit)+1, bits...),
		))
		gotErr := verifyCraftedResponse(t, response)
		var refusal Refusal
		if !errors.As(gotErr, &refusal) {
			t.Fatalf("Verify(all codes) error = %v, want errors.As Refusal", gotErr)
		}
		if !slices.Equal(refusal.Codes(), want) {
			t.Fatalf(
				"Verify(all codes) refusal codes = %v, want %v",
				refusal.Codes(),
				want,
			)
		}
	})

	t.Run("neutral granted status with no failure codes yields no refusal", func(t *testing.T) {
		t.Parallel()

		fixture := loadAuthenticFixture(t)
		got, gotErr := Verify(VerifyRequest{
			Response: fixture.response, Request: fixture.request,
			ExpectedDigest: fixture.digest,
		})
		var refusal Refusal
		if gotErr != nil || errors.As(gotErr, &refusal) || got.Validate() != nil {
			t.Fatalf(
				"Verify(authentic) timestamp/error = (%v, %v), want validated timestamp and no refusal",
				got,
				gotErr,
			)
		}
	})
}

func TestAuthorityRefusalHostileStatusInfoTable(t *testing.T) {
	t.Parallel()

	rejection, err := RefusalStatusRejection.rfcValue()
	if err != nil {
		t.Fatalf("RefusalStatusRejection.rfcValue() error = %v, want nil", err)
	}
	granted, err := RefusalStatusGranted.rfcValue()
	if err != nil {
		t.Fatalf("RefusalStatusGranted.rfcValue() error = %v, want nil", err)
	}
	maximumBit, err := refusalMaximumRFCBit()
	if err != nil {
		t.Fatalf("refusalMaximumRFCBit() error = %v, want nil", err)
	}
	ceiling := int(maximumBit) + 1
	token := authenticTokenDER(t)
	oversizedText := make([]string, refusalStatusTextCount+1)
	for index := range oversizedText {
		oversizedText[index] = "refused"
	}
	exactText := make([]string, refusalStatusTextCount)
	for index := range exactText {
		exactText[index] = "refused"
	}
	cases := []struct {
		name     string
		response []byte
	}{
		{
			name:     "status integer one above the closed set",
			response: encodeSequence(encodeSequence(encodeStatusInteger(t, 6))),
		},
		{
			name:     "status integer far above the closed set",
			response: encodeSequence(encodeSequence(encodeStatusInteger(t, 4096))),
		},
		{
			name:     "negative status integer",
			response: encodeSequence(encodeSequence(encodeStatusInteger(t, -1))),
		},
		{
			name:     "granted status carrying no token",
			response: encodeSequence(encodeSequence(encodeStatusInteger(t, granted))),
		},
		{
			name: "granted status carrying failure codes",
			response: encodeSequence(
				encodeSequence(
					encodeStatusInteger(t, granted),
					encodeFailInfo(t, ceiling, 0),
				),
				token,
			),
		},
		{
			name: "refusing status carrying a granted token",
			response: encodeSequence(
				encodeSequence(encodeStatusInteger(t, rejection)),
				token,
			),
		},
		{
			name: "failure bit string declaring zero bits",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, 0),
			)),
		},
		{
			name: "failure bit string one above the code ceiling",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling+1),
			)),
		},
		{
			name: "failure bit string present but all clear",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling),
			)),
		},
		{
			name: "failure bit string setting an unassigned low bit",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling, 1),
			)),
		},
		{
			name: "failure bit string setting an unassigned high bit",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling, 24),
			)),
		},
		{
			name: "failure bit string mixing assigned and unassigned bits",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling, 0, 3),
			)),
		},
		{
			name: "duplicate failure bit strings",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling, 0),
				encodeFailInfo(t, ceiling, 2),
			)),
		},
		{
			name: "duplicate status text sequences",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeStatusText(t, "refused"),
				encodeStatusText(t, "refused again"),
			)),
		},
		{
			name: "status text after failure codes",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeFailInfo(t, ceiling, 0),
				encodeStatusText(t, "refused"),
			)),
		},
		{
			name: "empty status text sequence",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeStatusText(t),
			)),
		},
		{
			name: "status text one above the element ceiling",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeStatusText(t, oversizedText...),
			)),
		},
		{
			name: "status text element that is not a UTF8String",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				encodeSequence(encodeStatusInteger(t, 1)),
			)),
		},
		{
			name: "unknown universal field inside the status info",
			response: encodeSequence(encodeSequence(
				encodeStatusInteger(t, rejection),
				[]byte{0x05, 0x00},
			)),
		},
		{
			name: "status field encoded as a bit string",
			response: encodeSequence(encodeSequence(
				encodeFailInfo(t, ceiling, 0),
			)),
		},
		{
			name:     "empty status info sequence",
			response: encodeSequence(encodeSequence()),
		},
		{
			name:     "status info that is not a sequence",
			response: encodeSequence(encodeStatusInteger(t, rejection)),
		},
		{
			name:     "response that is not a sequence",
			response: encodeStatusInteger(t, rejection),
		},
		{
			name: "trailing byte after a refusing response",
			response: append(
				encodeSequence(encodeSequence(encodeStatusInteger(t, rejection))),
				0,
			),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := verifyCraftedResponse(t, tc.response)
			if _, ok := errors.AsType[Refusal](gotErr); ok {
				t.Fatalf(
					"Verify(hostile status info) error = %v, want no typed refusal from malformed input",
					gotErr,
				)
			}
			if !errors.Is(gotErr, core.ErrTimeProofInvalid) {
				t.Fatalf(
					"Verify(hostile status info) error = %v, want %v",
					gotErr,
					core.ErrTimeProofInvalid,
				)
			}
		})
	}

	t.Run("exact status text element ceiling stays a typed refusal", func(t *testing.T) {
		t.Parallel()

		response := encodeSequence(encodeSequence(
			encodeStatusInteger(t, rejection),
			encodeStatusText(t, exactText...),
		))
		gotErr := verifyCraftedResponse(t, response)
		var refusal Refusal
		if !errors.As(gotErr, &refusal) ||
			refusal.Status() != RefusalStatusRejection {
			t.Fatalf(
				"Verify(exact text ceiling) error = %v, want Refusal with status %v",
				gotErr,
				RefusalStatusRejection,
			)
		}
	})

	t.Run("exact failure bit ceiling stays a typed refusal", func(t *testing.T) {
		t.Parallel()

		response := encodeSequence(encodeSequence(
			encodeStatusInteger(t, rejection),
			encodeFailInfo(t, ceiling, int(maximumBit)),
		))
		gotErr := verifyCraftedResponse(t, response)
		var refusal Refusal
		if !errors.As(gotErr, &refusal) ||
			!slices.Equal(refusal.Codes(), []RefusalCode{RefusalCodeSystemFailure}) {
			t.Fatalf(
				"Verify(exact bit ceiling) error = %v, want Refusal carrying %v",
				gotErr,
				RefusalCodeSystemFailure,
			)
		}
	})
}

func TestRefusalStatusClosedEnumMapping(t *testing.T) {
	t.Parallel()

	seen := make(map[int]RefusalStatus, refusalStatusLimit)
	for status := RefusalStatusGranted; status < refusalStatusLimit; status++ {
		if err := status.Validate(); err != nil || !status.IsValid() {
			t.Fatalf(
				"RefusalStatus(%d) validation = (%v, %t), want (nil, true)",
				status,
				err,
				status.IsValid(),
			)
		}
		var offWire core.OffWireEnum = status
		offWire.OffWireEnum()
		if status.String() == "" {
			t.Fatalf("RefusalStatus(%d).String() = %q, want a canonical token", status, "")
		}
		value, err := status.rfcValue()
		if err != nil {
			t.Fatalf("RefusalStatus(%d).rfcValue() error = %v, want nil", status, err)
		}
		if prior, ok := seen[value]; ok {
			t.Fatalf("RFC status %d maps to %v and %v, want one owner", value, prior, status)
		}
		seen[value] = status
		got, err := refusalStatusFromRFC(value)
		if err != nil || got != status {
			t.Fatalf(
				"refusalStatusFromRFC(%d) = (%v, %v), want (%v, nil)",
				value,
				got,
				err,
				status,
			)
		}
	}

	cases := []struct {
		name   string
		status RefusalStatus
	}{
		{name: "zero status is unknown", status: RefusalStatusUnknown},
		{name: "limit sentinel is not a member", status: refusalStatusLimit},
		{name: "one above the limit sentinel", status: refusalStatusLimit + 1},
		{name: "far above the limit sentinel", status: RefusalStatus(200)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.status.Validate(); !errors.Is(err, core.ErrTimeProofContract) {
				t.Fatalf(
					"RefusalStatus(%d).Validate() error = %v, want %v",
					tc.status,
					err,
					core.ErrTimeProofContract,
				)
			}
			if tc.status.IsValid() {
				t.Fatalf("RefusalStatus(%d).IsValid() = true, want false", tc.status)
			}
			if got := tc.status.String(); got != "" {
				t.Fatalf("RefusalStatus(%d).String() = %q, want empty", tc.status, got)
			}
			if _, err := tc.status.rfcValue(); !errors.Is(err, core.ErrTimeProofContract) {
				t.Fatalf(
					"RefusalStatus(%d).rfcValue() error = %v, want %v",
					tc.status,
					err,
					core.ErrTimeProofContract,
				)
			}
		})
	}

	for _, value := range []int{-4096, -1, 6, 7, 4096} {
		if _, err := refusalStatusFromRFC(value); !errors.Is(err, core.ErrTimeProofInvalid) {
			t.Fatalf(
				"refusalStatusFromRFC(%d) error = %v, want %v",
				value,
				err,
				core.ErrTimeProofInvalid,
			)
		}
	}
}

func TestRefusalCodeClosedEnumMapping(t *testing.T) {
	t.Parallel()

	seen := make(map[uint8]RefusalCode, refusalMaximumCodeCount)
	count := 0
	for code := RefusalCodeBadAlgorithm; code < refusalCodeLimit; code++ {
		count++
		if err := code.Validate(); err != nil || !code.IsValid() {
			t.Fatalf(
				"RefusalCode(%d) validation = (%v, %t), want (nil, true)",
				code,
				err,
				code.IsValid(),
			)
		}
		var offWire core.OffWireEnum = code
		offWire.OffWireEnum()
		if code.String() == "" {
			t.Fatalf("RefusalCode(%d).String() = %q, want a canonical token", code, "")
		}
		bit, err := code.rfcBit()
		if err != nil {
			t.Fatalf("RefusalCode(%d).rfcBit() error = %v, want nil", code, err)
		}
		if prior, ok := seen[bit]; ok {
			t.Fatalf("RFC bit %d maps to %v and %v, want one owner", bit, prior, code)
		}
		seen[bit] = code
		got, err := refusalCodeFromRFCBit(int(bit))
		if err != nil || got != code {
			t.Fatalf(
				"refusalCodeFromRFCBit(%d) = (%v, %v), want (%v, nil)",
				bit,
				got,
				err,
				code,
			)
		}
	}
	if count != refusalMaximumCodeCount {
		t.Fatalf(
			"closed refusal code count = %d, want %d",
			count,
			refusalMaximumCodeCount,
		)
	}

	for _, code := range []RefusalCode{
		RefusalCodeUnknown, refusalCodeLimit, refusalCodeLimit + 1, RefusalCode(200),
	} {
		if err := code.Validate(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"RefusalCode(%d).Validate() error = %v, want %v",
				code,
				err,
				core.ErrTimeProofContract,
			)
		}
		if code.IsValid() {
			t.Fatalf("RefusalCode(%d).IsValid() = true, want false", code)
		}
		if _, err := code.rfcBit(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"RefusalCode(%d).rfcBit() error = %v, want %v",
				code,
				err,
				core.ErrTimeProofContract,
			)
		}
	}

	for _, bit := range []int{-1, 1, 3, 4, 6, 13, 18, 24, 26, 4096} {
		if _, err := refusalCodeFromRFCBit(bit); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"refusalCodeFromRFCBit(%d) error = %v, want %v",
				bit,
				err,
				core.ErrTimeProofContract,
			)
		}
	}
}

func TestRefusalConstructionBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("duplicate code in one set is rejected", func(t *testing.T) {
		t.Parallel()

		_, err := newRefusalCodeSet(
			RefusalCodeBadRequest, RefusalCodeBadRequest,
		)
		if !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"newRefusalCodeSet(duplicate) error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
	})

	t.Run("unknown code in a set is rejected", func(t *testing.T) {
		t.Parallel()

		_, err := newRefusalCodeSet(RefusalCodeBadRequest, RefusalCodeUnknown)
		if !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"newRefusalCodeSet(unknown) error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
	})

	t.Run("empty code set is zero and projects no codes", func(t *testing.T) {
		t.Parallel()

		set, err := newRefusalCodeSet()
		if err != nil || !set.isZero() || len(set.codes()) != 0 {
			t.Fatalf(
				"newRefusalCodeSet() set/zero/codes/error = (%v, %t, %v, %v), want empty and nil",
				set,
				set.isZero(),
				set.codes(),
				err,
			)
		}
	})

	t.Run("granted status cannot become a refusal", func(t *testing.T) {
		t.Parallel()

		for _, status := range []RefusalStatus{
			RefusalStatusGranted, RefusalStatusGrantedWithMods,
		} {
			_, err := newRefusal(authorityConclusion{status: status})
			if !errors.Is(err, core.ErrTimeProofContract) {
				t.Fatalf(
					"newRefusal(%v) error = %v, want %v",
					status,
					err,
					core.ErrTimeProofContract,
				)
			}
		}
	})

	t.Run("unknown status cannot become a refusal", func(t *testing.T) {
		t.Parallel()

		_, err := newRefusal(authorityConclusion{status: RefusalStatusUnknown})
		if !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"newRefusal(unknown) error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
	})

	t.Run("zero refusal keeps the Core identity reachable", func(t *testing.T) {
		t.Parallel()

		var refusal Refusal
		if !errors.Is(refusal, core.ErrTimeProofRefused) {
			t.Fatalf(
				"errors.Is(Refusal{}, %v) = false, want true",
				core.ErrTimeProofRefused,
			)
		}
		if err := refusal.Validate(); !errors.Is(err, core.ErrTimeProofContract) {
			t.Fatalf(
				"Refusal{}.Validate() error = %v, want %v",
				err,
				core.ErrTimeProofContract,
			)
		}
	})
}
