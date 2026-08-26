package chit

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestPartitionConstructionPressuresTheCompleteZeroBoundary(t *testing.T) {
	t.Parallel()

	type partitionCase struct {
		name    string
		raw     [core.SHA256DigestBytes]byte
		wantErr error
	}
	cases := []partitionCase{{
		name: "all zero commitment is the sole refused typed value", wantErr: core.ErrChitContract,
	}}
	for position := range core.SHA256DigestBytes {
		raw := [core.SHA256DigestBytes]byte{}
		raw[position] = 1
		cases = append(cases, partitionCase{
			name: "minimum nonzero commitment at byte " + strconv.Itoa(position), raw: raw,
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			digest := core.NewSHA256Digest(tc.raw)
			got, gotErr := NewPartition(digest)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (Partition{}) {
					t.Fatalf("NewPartition() = (%v, %v), want zero and errors.Is %v", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("NewPartition() = (%v, %v), validation %v, want admitted and nil", got, gotErr, got.Validate())
			}
			gotJSON, gotMarshalErr := got.MarshalJSON()
			wantJSON, wantMarshalErr := digest.MarshalJSON()
			if gotMarshalErr != nil || wantMarshalErr != nil || !bytes.Equal(gotJSON, wantJSON) {
				t.Fatalf(
					"Partition.MarshalJSON() = (%q, %v), want digest projection (%q, %v)",
					gotJSON, gotMarshalErr, wantJSON, wantMarshalErr,
				)
			}
		})
	}
}

func TestPartitionJSONHostileTablePreservesTheReceiverAndCanonicalClosure(t *testing.T) {
	t.Parallel()

	seedRaw := [core.SHA256DigestBytes]byte{0x81}
	seed, err := NewPartition(core.NewSHA256Digest(seedRaw))
	if err != nil {
		t.Fatalf("NewPartition(seed) error = %v, want nil", err)
	}

	type partitionCase struct {
		name         string
		encoded      []byte
		want         Partition
		wantErr      error
		wantCauseErr error
	}
	cases := make([]partitionCase, 0, 40)
	for marker := byte(1); marker <= 10; marker++ {
		raw := [core.SHA256DigestBytes]byte{}
		for index := range raw {
			raw[index] = marker
		}
		want, constructErr := NewPartition(core.NewSHA256Digest(raw))
		if constructErr != nil {
			t.Fatalf("NewPartition(valid marker %d) error = %v, want nil", marker, constructErr)
		}
		encoded, marshalErr := want.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("Partition.MarshalJSON(valid marker %d) error = %v, want nil", marker, marshalErr)
		}
		cases = append(cases, partitionCase{
			name: "canonical independent commitment marker " + strconv.Itoa(int(marker)), encoded: encoded, want: want,
		})
	}
	cases = append(cases,
		partitionCase{name: "empty document is refused", wantErr: core.ErrJSONContract},
		partitionCase{name: "null is refused", encoded: []byte(`null`), wantErr: core.ErrJSONContract},
		partitionCase{name: "empty string is refused", encoded: []byte(`""`), wantErr: core.ErrJSONContract},
		partitionCase{name: "one hexadecimal digit is refused", encoded: []byte(`"0"`), wantErr: core.ErrJSONContract},
		partitionCase{name: "one digit below exact extent is refused", encoded: []byte(`"111111111111111111111111111111111111111111111111111111111111111"`), wantErr: core.ErrJSONContract},
		partitionCase{name: "one digit above exact extent is refused", encoded: []byte(`"11111111111111111111111111111111111111111111111111111111111111111"`), wantErr: core.ErrJSONContract},
		partitionCase{name: "uppercase hexadecimal is refused", encoded: []byte(`"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"`), wantErr: core.ErrJSONContract},
		partitionCase{name: "non hexadecimal text is refused", encoded: []byte(`"gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"`), wantErr: core.ErrJSONContract},
		partitionCase{name: "numeric JSON is refused", encoded: []byte(`1`), wantErr: core.ErrJSONContract},
		partitionCase{name: "all zero canonical digest is refused", encoded: []byte(`"0000000000000000000000000000000000000000000000000000000000000000"`), wantErr: core.ErrJSONContract, wantCauseErr: core.ErrChitContract},
	)
	for position := range 20 {
		raw := [core.SHA256DigestBytes]byte{}
		raw[position] = 1
		want, constructErr := NewPartition(core.NewSHA256Digest(raw))
		if constructErr != nil {
			t.Fatalf("NewPartition(boundary byte %d) error = %v, want nil", position, constructErr)
		}
		encoded, marshalErr := want.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("Partition.MarshalJSON(boundary byte %d) error = %v, want nil", position, marshalErr)
		}
		cases = append(cases, partitionCase{
			name: "minimum nonzero JSON commitment at byte " + strconv.Itoa(position), encoded: encoded, want: want,
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := seed
			gotErr := got.UnmarshalJSON(tc.encoded)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) ||
					(tc.wantCauseErr != nil && !errors.Is(gotErr, tc.wantCauseErr)) || got != seed {
					t.Fatalf("Partition.UnmarshalJSON() = (%v, %v), want preserved %v and errors.Is (%v, %v)",
						got, gotErr, seed, tc.wantErr, tc.wantCauseErr)
				}
				return
			}
			if gotErr != nil || got != tc.want || got.Validate() != nil {
				t.Fatalf("Partition.UnmarshalJSON() = (%v, %v), validation %v, want (%v, nil)", got, gotErr, got.Validate(), tc.want)
			}
			canonical, marshalErr := got.MarshalJSON()
			var roundTrip Partition
			roundTripErr := roundTrip.UnmarshalJSON(canonical)
			second, secondErr := roundTrip.MarshalJSON()
			if marshalErr != nil || roundTripErr != nil || secondErr != nil || roundTrip != got || !bytes.Equal(second, canonical) {
				t.Fatalf(
					"Partition canonical closure = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
					roundTrip, second, marshalErr, roundTripErr, secondErr, got, canonical,
				)
			}
		})
	}

	var nilReceiver *Partition
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("Partition.MarshalJSON(seed) error = %v, want nil", err)
	}
	if gotErr := nilReceiver.UnmarshalJSON(canonical); !errors.Is(gotErr, core.ErrJSONContract) ||
		!errors.Is(gotErr, core.ErrChitContract) {
		t.Fatalf("nil Partition.UnmarshalJSON() error = %v, want errors.Is (%v, %v)",
			gotErr, core.ErrJSONContract, core.ErrChitContract)
	}
	got, gotErr := (Partition{}).MarshalJSON()
	if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrChitContract) || got != nil {
		t.Fatalf("zero Partition.MarshalJSON() = (%q, %v), want nil and errors.Is (%v, %v)",
			got, gotErr, core.ErrJSONContract, core.ErrChitContract)
	}
}

func mustPartition(t testing.TB, marker byte) Partition {
	t.Helper()
	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	partition, err := NewPartition(core.NewSHA256Digest(raw))
	if err != nil {
		t.Fatalf("NewPartition(marker %d) error = %v, want nil", marker, err)
	}
	return partition
}
