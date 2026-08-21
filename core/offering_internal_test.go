package core

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
)

type offeringPressureClass uint8

const (
	offeringPressureAccepted offeringPressureClass = iota
	offeringPressureRejected
	offeringPressureBoundary
	offeringPressureClassCount
)

func TestOfferingCoreSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		token     string
		other     string
		wantEqual bool
	}{
		{name: "positive canonical token constructs exact offering", token: "kernel-manual", other: "kernel-manual", wantEqual: true},
		{name: "negative noncanonical token returns typed refusal and zero offering", token: "KernelManual", wantErr: ErrPrimitiveContract},
		{name: "neutral distinct opaque tokens remain distinct without interpretation", token: "kernel-manual", other: "future-product"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := Offering{Token: tc.token}
			gotErr := got.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Offering{%q}.Validate() error = %v, want %v", tc.token, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				var decoded Offering
				decodedErr := decoded.UnmarshalText([]byte(tc.token))
				if !errors.Is(decodedErr, tc.wantErr) || decoded != (Offering{}) {
					t.Fatalf("Offering.UnmarshalText(%q) = (%v, %v), want zero offering and %v", tc.token, decoded, decodedErr, tc.wantErr)
				}
				return
			}
			other := Offering{Token: tc.other}
			if got.Validate() != nil || other.Validate() != nil || (got == other) != tc.wantEqual {
				t.Fatalf("offering relation = (%v, %v, equal %t), want two valid offerings with equal %t", got, other, got == other, tc.wantEqual)
			}
		})
	}
}

func TestOfferingValidationHostileTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		token   string
		class   offeringPressureClass
	}{
		{name: "accepted repeated lowercase letters", token: "aa", class: offeringPressureAccepted},
		{name: "accepted descending lowercase letters", token: "za", class: offeringPressureAccepted},
		{name: "accepted ascending lowercase letters", token: "az", class: offeringPressureAccepted},
		{name: "accepted interior digit", token: "a1b", class: offeringPressureAccepted},
		{name: "accepted trailing digit", token: "abc0", class: offeringPressureAccepted},
		{name: "accepted one separated suffix", token: "alpha-beta", class: offeringPressureAccepted},
		{name: "accepted digit-bearing suffix", token: "a-b9", class: offeringPressureAccepted},
		{name: "accepted several separated segments", token: "a-b-c-d", class: offeringPressureAccepted},
		{name: "accepted mixed canonical segments", token: "alpha-2-beta-9", class: offeringPressureAccepted},
		{name: "accepted ordinary opaque identity", token: "consumer-fixture", class: offeringPressureAccepted},

		{name: "rejected uppercase first byte", token: "Alpha", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected uppercase interior byte", token: "alPha", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected underscore separator", token: "alpha_beta", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected slash separator", token: "alpha/beta", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected dotted identity", token: "alpha.beta", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected colon separator", token: "alpha:beta", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected leading space", token: " alpha", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected trailing space", token: "alpha ", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected non ASCII letter", token: "alphå", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},
		{name: "rejected embedded NUL", token: "alpha\x00beta", class: offeringPressureRejected, wantErr: ErrPrimitiveContract},

		{name: "boundary one below maximum extent", token: strings.Repeat("a", offeringMaximumBytes-1), class: offeringPressureBoundary},
		{name: "boundary exact maximum extent", token: strings.Repeat("a", offeringMaximumBytes), class: offeringPressureBoundary},
		{name: "boundary one above maximum extent", token: strings.Repeat("a", offeringMaximumBytes+1), class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary exact minimum extent", token: "a", class: offeringPressureBoundary},
		{name: "boundary one below minimum extent", token: "", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary first alphabet byte", token: "a-tail", class: offeringPressureBoundary},
		{name: "boundary last alphabet byte", token: "z-tail", class: offeringPressureBoundary},
		{name: "boundary byte immediately below alphabet", token: "`tail", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary byte immediately above alphabet", token: "{tail", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary first digit after leading letter", token: "b0", class: offeringPressureBoundary},
		{name: "boundary last digit after leading letter", token: "b9", class: offeringPressureBoundary},
		{name: "boundary byte immediately below digits", token: "b/", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary byte immediately above digits", token: "b:", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary separator at first legal interior position", token: "c-d", class: offeringPressureBoundary},
		{name: "boundary separator at leading position", token: "-ab", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary separator at trailing position", token: "ab-", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary separator immediately after separator", token: "a--b", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
		{name: "boundary maximum extent ending in digit", token: strings.Repeat("a", offeringMaximumBytes-1) + "9", class: offeringPressureBoundary},
		{name: "boundary maximum extent with final separated segment", token: strings.Repeat("a", offeringMaximumBytes-2) + "-a", class: offeringPressureBoundary},
		{name: "boundary maximum extent ending in separator", token: strings.Repeat("a", offeringMaximumBytes-1) + "-", class: offeringPressureBoundary, wantErr: ErrPrimitiveContract},
	}

	var gotClassCounts [offeringPressureClassCount]int
	for _, tc := range cases {
		gotClassCounts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			offering := Offering{Token: tc.token}
			gotErr := offering.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Offering{%q}.Validate() error = %v, want %v", tc.token, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				preserved := Offering{Token: "preserved-fixture"}
				gotText := preserved
				gotTextErr := gotText.UnmarshalText([]byte(tc.token))
				if !errors.Is(gotTextErr, tc.wantErr) || gotText != preserved {
					t.Fatalf("Offering.UnmarshalText(%q) = (%v, %v), want (%v, %v)", tc.token, gotText, gotTextErr, preserved, tc.wantErr)
				}

				encodedToken, err := json.Marshal(tc.token)
				if err != nil {
					t.Fatalf("json.Marshal(%q) error = %v, want nil", tc.token, err)
				}
				gotJSON := preserved
				gotJSONErr := gotJSON.UnmarshalJSON(encodedToken)
				if !errors.Is(gotJSONErr, ErrJSONContract) || !errors.Is(gotJSONErr, tc.wantErr) || gotJSON != preserved {
					t.Fatalf("Offering.UnmarshalJSON(%q) = (%v, %v), want (%v, errors including %v and %v)", encodedToken, gotJSON, gotJSONErr, preserved, ErrJSONContract, tc.wantErr)
				}
				return
			}

			if got := offering.String(); got != tc.token {
				t.Fatalf("Offering{%q}.String() = %q, want %q", tc.token, got, tc.token)
			}
			firstJSON, err := offering.MarshalJSON()
			if err != nil {
				t.Fatalf("Offering{%q}.MarshalJSON() error = %v, want nil", tc.token, err)
			}
			var fromJSON Offering
			if err := fromJSON.UnmarshalJSON(firstJSON); err != nil || fromJSON != offering {
				t.Fatalf("Offering.UnmarshalJSON(%q) = (%v, %v), want (%v, nil)", firstJSON, fromJSON, err, offering)
			}
			secondJSON, err := fromJSON.MarshalJSON()
			if err != nil || !bytes.Equal(secondJSON, firstJSON) {
				t.Fatalf("Offering second MarshalJSON() = (%q, %v), want (%q, nil)", secondJSON, err, firstJSON)
			}
			var fromText Offering
			if err := fromText.UnmarshalText([]byte(tc.token)); err != nil || fromText != offering {
				t.Fatalf("Offering.UnmarshalText(%q) = (%v, %v), want (%v, nil)", tc.token, fromText, err, offering)
			}
		})
	}

	wantClassCounts := [offeringPressureClassCount]int{10, 10, 20}
	if gotClassCounts != wantClassCounts {
		t.Fatalf("Offering pressure class counts = %v, want %v", gotClassCounts, wantClassCounts)
	}
}
