package chit

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestManifestAdmissionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive streamed tickets prove exact members across extent and cardinality boundaries", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name     string
			extents  []uint64
			selected int
		}{
			{name: "one authenticated empty object retains zero extent membership", extents: []uint64{0}},
			{name: "minimum nonempty object retains one-byte membership", extents: []uint64{1}},
			{name: "first member remains selectable before a nonempty tail", extents: []uint64{1, 2}},
			{name: "last member remains selectable after a nonempty prefix", extents: []uint64{2, 1}, selected: 1},
			{name: "empty interior member remains distinct between nonempty neighbors", extents: []uint64{1, 0, 2}, selected: 1},
			{name: "extent one below signed ceiling survives the ticket handoff", extents: []uint64{math.MaxInt64 - 1}},
			{name: "exact signed extent ceiling survives the ticket handoff", extents: []uint64{math.MaxInt64}},
			{name: "two entries summing to the exact extent ceiling remain separate", extents: []uint64{math.MaxInt64 - 1, 1}, selected: 1},
			{name: "zero prefix remains selectable beside a saturated tail", extents: []uint64{0, math.MaxInt64}},
			{
				name:    "manifest beyond one bounded catalog page releases the selected tail",
				extents: make([]uint64, core.CatalogPageMaximumEntries+1), selected: core.CatalogPageMaximumEntries,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				additions := manifestAdmissionExtentAdditions(t, tc.extents)
				accumulator := mustManifestAdmissionAccumulator(t)
				var selected ManifestAdmission
				for index, addition := range additions {
					admission, err := accumulator.Add(addition)
					if err != nil {
						t.Fatalf("ManifestAdmissionAccumulator.Add(%d) error = %v, want nil", index, err)
					}
					if index == tc.selected {
						selected = admission
					}
				}
				wantSummary := manifestSummaryFixture(t, additions...)
				verified, err := accumulator.Seal(wantSummary)
				if err != nil {
					t.Fatalf("ManifestAdmissionAccumulator.Seal() error = %v, want nil", err)
				}
				if err := accumulator.Destroy(); err != nil {
					t.Fatalf("ManifestAdmissionAccumulator.Destroy(after ownership transfer) error = %v, want nil", err)
				}
				t.Cleanup(func() {
					if err := verified.Destroy(); err != nil {
						t.Fatalf("VerifiedManifestAdmission.Destroy() error = %v, want nil", err)
					}
				})
				wire, err := selected.MarshalJSON()
				if err != nil {
					t.Fatalf("ManifestAdmission.MarshalJSON(selected) error = %v, want nil", err)
				}
				var replayed ManifestAdmission
				if err := json.Unmarshal(wire, &replayed); err != nil {
					t.Fatalf("ManifestAdmission.UnmarshalJSON(selected) error = %v, want nil", err)
				}
				proof, err := verified.Verify(replayed, additions[tc.selected].Evidence)
				if err != nil {
					t.Fatalf("VerifiedManifestAdmission.Verify(selected) error = %v, want nil", err)
				}
				got, additionErr := proof.Addition()
				gotSummary, summaryErr := proof.Summary()
				if additionErr != nil || summaryErr != nil || got.Entry != additions[tc.selected].Entry || gotSummary != wantSummary {
					t.Fatalf("VerifiedManifestEntry = (%v, %v, %v, %v), want exact selected entry and summary", got, additionErr, gotSummary, summaryErr)
				}
			})
		}
	})

	t.Run("negative independently valid mutations never acquire membership", func(t *testing.T) {
		t.Parallel()
		fixture := newChitFixture(t, 0xa1, 1)
		foreign := newChitFixture(t, 0xa2, 1)
		cases := []struct {
			name     string
			exercise func(*testing.T) (VerifiedManifestEntry, error)
			wantErr  error
		}{
			{name: "nil accumulator cannot issue a ticket", wantErr: core.ErrChitContract, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				_, err := (*ManifestAdmissionAccumulator)(nil).Add(fixture.addition)
				return VerifiedManifestEntry{}, err
			}},
			{name: "zero accumulator cannot issue a ticket", wantErr: core.ErrChitContract, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				_, err := (&ManifestAdmissionAccumulator{}).Add(fixture.addition)
				return VerifiedManifestEntry{}, err
			}},
			{name: "zero addition cannot enter the fold", wantErr: core.ErrChitContract, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				accumulator := mustManifestAdmissionAccumulator(t)
				_, err := accumulator.Add(ManifestAddition{})
				return VerifiedManifestEntry{}, err
			}},
			{name: "first sequence cannot begin at two", wantErr: core.ErrChitConflict, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				accumulator := mustManifestAdmissionAccumulator(t)
				addition := chitManifestEntryFixture(t, chitDerivedEntryFixtureRequest{Fixture: fixture, Marker: 0xb2, Sequence: 2, Name: chitFixtureNameB})
				_, err := accumulator.Add(addition)
				return VerifiedManifestEntry{}, err
			}},
			{name: "foreign summary destroys the unfinished authority", wantErr: core.ErrChitConflict, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				accumulator := mustManifestAdmissionAccumulator(t)
				if _, err := accumulator.Add(fixture.addition); err != nil {
					t.Fatalf("ManifestAdmissionAccumulator.Add() error = %v, want nil", err)
				}
				_, err := accumulator.Seal(foreign.summary)
				if destroyErr := accumulator.Destroy(); destroyErr != nil {
					t.Fatalf("destroy after refused seal error = %v, want nil", destroyErr)
				}
				return VerifiedManifestEntry{}, err
			}},
			{name: "one-bit ticket mutation is a typed conflict", wantErr: core.ErrChitConflict, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				admission, verified := sealedManifestAdmission(t, fixture.addition)
				admission.tag[0] ^= 1
				return verified.Verify(admission, fixture.addition.Evidence)
			}},
			{name: "valid foreign entry cannot borrow an authentic ticket", wantErr: core.ErrChitConflict, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				admission, verified := sealedManifestAdmission(t, fixture.addition)
				admission.entry = foreign.addition.Entry
				return verified.Verify(admission, foreign.addition.Evidence)
			}},
			{name: "foreign verified receipt cannot satisfy the admitted entry", wantErr: core.ErrChitConflict, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				admission, verified := sealedManifestAdmission(t, fixture.addition)
				return verified.Verify(admission, foreign.addition.Evidence)
			}},
			{name: "ticket from another sealed fold cannot cross capabilities", wantErr: core.ErrChitConflict, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				admission, first := sealedManifestAdmission(t, fixture.addition)
				_, second := sealedManifestAdmission(t, fixture.addition)
				if err := first.Destroy(); err != nil {
					t.Fatalf("first verifier Destroy() error = %v, want nil", err)
				}
				return second.Verify(admission, fixture.addition.Evidence)
			}},
			{name: "destroyed verifier cannot release a former member", wantErr: core.ErrChitContract, exercise: func(t *testing.T) (VerifiedManifestEntry, error) {
				admission, verified := sealedManifestAdmission(t, fixture.addition)
				if err := verified.Destroy(); err != nil {
					t.Fatalf("VerifiedManifestAdmission.Destroy() error = %v, want nil", err)
				}
				return verified.Verify(admission, fixture.addition.Evidence)
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got, gotErr := tc.exercise(t)
				if !errors.Is(gotErr, tc.wantErr) || got != (VerifiedManifestEntry{}) {
					t.Fatalf("membership mutation result = (%v, %v), want zero and errors.Is(_, %v)", got, gotErr, tc.wantErr)
				}
			})
		}
	})

	t.Run("neutral absence and terminal reuse emit no proof", func(t *testing.T) {
		t.Parallel()
		fixture := newChitFixture(t, 0xc1, 1)
		accumulator := mustManifestAdmissionAccumulator(t)
		if got, err := accumulator.Seal(ManifestSummary{}); !errors.Is(err, core.ErrChitContract) || got.Validate() == nil {
			t.Fatalf("Seal(zero summary) = (%v, %v), want zero and Chit contract", got, err)
		}
		if _, err := accumulator.Add(fixture.addition); err != nil {
			t.Fatalf("Add(after non-consuming refusal) error = %v, want nil", err)
		}
		verified, err := accumulator.Seal(fixture.summary)
		if err != nil {
			t.Fatalf("Seal(exact retry) error = %v, want nil", err)
		}
		t.Cleanup(func() {
			if err := verified.Destroy(); err != nil {
				t.Fatalf("VerifiedManifestAdmission.Destroy() error = %v, want nil", err)
			}
		})
		if got, gotErr := accumulator.Add(fixture.addition); !errors.Is(gotErr, core.ErrChitContract) || got.Validate() == nil {
			t.Fatalf("Add(after seal) = (%v, %v), want zero and Chit contract", got, gotErr)
		}
		if got, gotErr := verified.Verify(ManifestAdmission{}, fixture.addition.Evidence); !errors.Is(gotErr, core.ErrChitContract) || got != (VerifiedManifestEntry{}) {
			t.Fatalf("Verify(absent ticket) = (%v, %v), want zero and Chit contract", got, gotErr)
		}
	})
}

func TestManifestAdmissionJSONTicketBoundaryTable(t *testing.T) {
	t.Parallel()
	fixture := newChitFixture(t, 0xd1, 1)
	admission, verified := sealedManifestAdmission(t, fixture.addition)
	t.Cleanup(func() {
		if err := verified.Destroy(); err != nil {
			t.Fatalf("VerifiedManifestAdmission.Destroy() error = %v, want nil", err)
		}
	})
	canonical, err := admission.MarshalJSON()
	if err != nil {
		t.Fatalf("ManifestAdmission.MarshalJSON() error = %v, want nil", err)
	}
	ticket := strings.Repeat("0", 64)
	cases := []struct {
		name    string
		wire    []byte
		wantErr error
	}{
		{name: "canonical compiler-owned ticket is accepted", wire: canonical},
		{name: "surrounding JSON whitespace preserves the typed ticket", wire: append(append([]byte(" \n"), canonical...), '\t')},
		{name: "empty input is refused", wire: nil, wantErr: core.ErrJSONContract},
		{name: "whitespace-only input is refused", wire: []byte(" \n\t"), wantErr: core.ErrJSONContract},
		{name: "null cannot erase an existing admission", wire: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "array cannot masquerade as an admission", wire: []byte("[]"), wantErr: core.ErrJSONContract},
		{name: "truncated object is refused", wire: []byte("{\"entry\":"), wantErr: core.ErrJSONContract},
		{name: "trailing document is refused", wire: append(append([]byte{}, canonical...), canonical...), wantErr: core.ErrJSONContract},
		{name: "unknown member is refused", wire: bytes.Replace(canonical, []byte("}"), []byte(",\"unknown\":1}"), 1), wantErr: core.ErrJSONContract},
		{name: "duplicate ticket is refused", wire: bytes.Replace(canonical, []byte("}"), []byte(",\"ticket\":\""+ticket+"\"}"), 1), wantErr: core.ErrJSONContract},
		{name: "missing ticket is refused", wire: bytes.Replace(canonical, []byte(",\"ticket\":"), []byte(",\"absent\":"), 1), wantErr: core.ErrJSONContract},
		{name: "ticket one nibble short is refused", wire: replaceManifestAdmissionTicket(canonical, strings.Repeat("0", 63)), wantErr: core.ErrJSONContract},
		{name: "ticket one nibble long is refused", wire: replaceManifestAdmissionTicket(canonical, strings.Repeat("0", 65)), wantErr: core.ErrJSONContract},
		{name: "ticket exact extent with non-hex symbol is refused", wire: replaceManifestAdmissionTicket(canonical, strings.Repeat("g", 64)), wantErr: core.ErrJSONContract},
		{name: "uppercase ticket is noncanonical", wire: replaceManifestAdmissionTicket(canonical, strings.Repeat("A", 64)), wantErr: core.ErrJSONContract},
		{name: "numeric-looking string remains a foreign ticket", wire: replaceManifestAdmissionTicket(canonical, ticket), wantErr: nil},
		{name: "entry array is refused", wire: bytes.Replace(canonical, []byte("{\"entry\":{"), []byte("{\"entry\":["), 1), wantErr: core.ErrJSONContract},
		{name: "one byte beyond document ceiling is refused", wire: bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1), wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := admission
			gotErr := got.UnmarshalJSON(tc.wire)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != admission {
					t.Fatalf("ManifestAdmission.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is(_, %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ManifestAdmission.UnmarshalJSON() error = %v, want nil", gotErr)
			}
			if bytes.Equal(tc.wire, canonical) || bytes.Equal(bytes.TrimSpace(tc.wire), canonical) {
				if got != admission {
					t.Fatalf("ManifestAdmission.UnmarshalJSON(canonical) = %v, want exact admission", got)
				}
				return
			}
			if proof, verifyErr := verified.Verify(got, fixture.addition.Evidence); !errors.Is(verifyErr, core.ErrChitConflict) || proof != (VerifiedManifestEntry{}) {
				t.Fatalf("foreign structural ticket verification = (%v, %v), want zero and Chit conflict", proof, verifyErr)
			}
		})
	}
}

func FuzzManifestAdmissionExternalDecoderAndMembership(f *testing.F) {
	fixture := newChitFixture(f, 0xe1, 1)
	admission, verified := sealedManifestAdmission(f, fixture.addition)
	f.Cleanup(func() {
		if err := verified.Destroy(); err != nil {
			f.Fatalf("VerifiedManifestAdmission.Destroy() error = %v, want nil", err)
		}
	})
	canonical, err := admission.MarshalJSON()
	if err != nil {
		f.Fatalf("ManifestAdmission.MarshalJSON(seed) error = %v, want nil", err)
	}
	for _, seed := range [][]byte{canonical, nil, []byte("{}"), []byte("null"), replaceManifestAdmissionTicket(canonical, strings.Repeat("0", 64))} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := admission
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || candidate != admission {
				t.Fatalf("ManifestAdmission.UnmarshalJSON(rejected) = (%v, %v), want preserved receiver and JSON contract", candidate, decodeErr)
			}
			return
		}
		if err := candidate.Validate(); err != nil {
			t.Fatalf("ManifestAdmission.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := candidate.MarshalJSON()
		if err != nil || len(encoded) > core.JSONDocumentMaximumBytes {
			t.Fatalf("ManifestAdmission.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip ManifestAdmission
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != candidate {
			t.Fatalf("ManifestAdmission canonical round trip = (%v, %v), want exact candidate and nil", roundTrip, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("ManifestAdmission second canonical projection = (%q, %v), want %q and nil", second, err, encoded)
		}
		proof, verifyErr := verified.Verify(candidate, fixture.addition.Evidence)
		if candidate == admission {
			if verifyErr != nil || proof.Validate() != nil {
				t.Fatalf("genuine admission verification = (%v, %v), want valid proof and nil", proof, verifyErr)
			}
		} else if !errors.Is(verifyErr, core.ErrChitConflict) || proof != (VerifiedManifestEntry{}) {
			t.Fatalf("mutated admission verification = (%v, %v), want zero and Chit conflict", proof, verifyErr)
		}
	})
}

func manifestAdmissionExtentAdditions(t testing.TB, extents []uint64) []ManifestAddition {
	t.Helper()
	fixture := newChitFixture(t, 0x21, 1)
	additions := make([]ManifestAddition, 0, len(extents))
	for index, extent := range extents {
		length := mustChitByteLength(t, extent)
		additions = append(additions, chitEvidenceEntryFixture(t, chitEntryFixtureRequest{
			Private: fixture.private, Trusted: fixture.trusted, Scope: fixture.scope,
			Sequence: uint64(index + 1), Marker: byte(index + 0x31), Name: chitFixtureNameA,
			Extent: &length,
		}))
	}
	return additions
}

func mustManifestAdmissionAccumulator(t testing.TB) *ManifestAdmissionAccumulator {
	t.Helper()
	accumulator, err := NewManifestAdmissionAccumulator()
	if err != nil {
		t.Fatalf("NewManifestAdmissionAccumulator() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := accumulator.Destroy(); err != nil {
			t.Fatalf("ManifestAdmissionAccumulator.Destroy() error = %v, want nil", err)
		}
	})
	return accumulator
}

func sealedManifestAdmission(t testing.TB, addition ManifestAddition) (ManifestAdmission, VerifiedManifestAdmission) {
	t.Helper()
	accumulator := mustManifestAdmissionAccumulator(t)
	admission, err := accumulator.Add(addition)
	if err != nil {
		t.Fatalf("ManifestAdmissionAccumulator.Add() error = %v, want nil", err)
	}
	verified, err := accumulator.Seal(manifestSummaryFixture(t, addition))
	if err != nil {
		t.Fatalf("ManifestAdmissionAccumulator.Seal() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := verified.Destroy(); err != nil {
			t.Fatalf("VerifiedManifestAdmission.Destroy() error = %v, want nil", err)
		}
	})
	return admission, verified
}

func replaceManifestAdmissionTicket(document []byte, ticket string) []byte {
	start := bytes.Index(document, []byte(`"ticket":"`))
	if start < 0 {
		return append([]byte(nil), document...)
	}
	start += len(`"ticket":"`)
	end := start + 64
	return append(append(append([]byte{}, document[:start]...), ticket...), document[end:]...)
}
