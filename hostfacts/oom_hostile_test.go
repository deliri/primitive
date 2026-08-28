package hostfacts

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoOOMBannerClassifierHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		data      string
		length    uint64
		chunk     int
		wantState GoOOMBannerState
	}{
		{name: "empty declared extent is absent", data: "", length: 0, chunk: 1, wantState: GoOOMBannerAbsent},
		{name: "unrelated one byte is absent", data: "x", length: 1, chunk: 1, wantState: GoOOMBannerAbsent},
		{name: "plain banner is present", data: GoOOMPlainBanner, length: uint64(len(GoOOMPlainBanner)), chunk: len(GoOOMPlainBanner), wantState: GoOOMBannerPresent},
		{name: "prefixed banner is present", data: GoOOMPrefixedBanner, length: uint64(len(GoOOMPrefixedBanner)), chunk: len(GoOOMPrefixedBanner), wantState: GoOOMBannerPresent},
		{name: "plain banner split every byte is present", data: GoOOMPlainBanner, length: uint64(len(GoOOMPlainBanner)), chunk: 1, wantState: GoOOMBannerPresent},
		{name: "prefixed banner split every byte is present", data: GoOOMPrefixedBanner, length: uint64(len(GoOOMPrefixedBanner)), chunk: 1, wantState: GoOOMBannerPresent},
		{name: "plain banner after prefix noise is present", data: "noise\n" + GoOOMPlainBanner, length: uint64(len("noise\n" + GoOOMPlainBanner)), chunk: 3, wantState: GoOOMBannerPresent},
		{name: "prefixed banner before suffix noise is present", data: GoOOMPrefixedBanner + "\nnoise", length: uint64(len(GoOOMPrefixedBanner + "\nnoise")), chunk: 5, wantState: GoOOMBannerPresent},
		{name: "both banners remain present", data: GoOOMPlainBanner + GoOOMPrefixedBanner, length: uint64(len(GoOOMPlainBanner + GoOOMPrefixedBanner)), chunk: 7, wantState: GoOOMBannerPresent},
		{name: "banner at maximum extent tail is present", data: strings.Repeat("x", GoOOMMaximumEvidenceBytes-len(GoOOMPlainBanner)) + GoOOMPlainBanner, length: GoOOMMaximumEvidenceBytes, chunk: 4093, wantState: GoOOMBannerPresent},
		{name: "one byte short plain banner is absent", data: GoOOMPlainBanner[:len(GoOOMPlainBanner)-1], length: uint64(len(GoOOMPlainBanner) - 1), chunk: 2, wantState: GoOOMBannerAbsent},
		{name: "one byte short prefixed banner is absent", data: GoOOMPrefixedBanner[:len(GoOOMPrefixedBanner)-1], length: uint64(len(GoOOMPrefixedBanner) - 1), chunk: 2, wantState: GoOOMBannerAbsent},
		{name: "wrong capitalization is absent", data: "Fatal error: out of memory", length: uint64(len("Fatal error: out of memory")), chunk: 4, wantState: GoOOMBannerAbsent},
		{name: "embedded NUL near miss is absent", data: "fatal error:\x00 out of memory", length: uint64(len("fatal error:\x00 out of memory")), chunk: 4, wantState: GoOOMBannerAbsent},
		{name: "declared extent hides trailing banner", data: "prefix" + GoOOMPlainBanner, length: uint64(len("prefix")), chunk: 2, wantState: GoOOMBannerAbsent},
		{name: "short source fails", data: "short", length: 6, chunk: 2, wantErr: io.ErrUnexpectedEOF},
		{name: "empty source with positive extent fails", data: "", length: 1, chunk: 1, wantErr: io.ErrUnexpectedEOF},
		{name: "declared maximum with short source fails", data: "x", length: GoOOMMaximumEvidenceBytes, chunk: 1, wantErr: io.ErrUnexpectedEOF},
		{name: "overlapping fatal prefix recovers", data: "fatal fatal error: out of memory", length: uint64(len("fatal fatal error: out of memory")), chunk: 2, wantState: GoOOMBannerPresent},
		{name: "repeated banner prefix near miss is absent", data: strings.Repeat("fatal error: out of memorx", 20), length: uint64(len(strings.Repeat("fatal error: out of memorx", 20))), chunk: 3, wantState: GoOOMBannerAbsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := &chunkReader{data: []byte(tc.data), maximum: tc.chunk}
			got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
				Source: source,
				Length: mustByteLength(t, tc.length),
			})
			if tc.wantErr != nil {
				if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ClassifyGoOOMBanner() = (%v, %v), want (zero, %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ClassifyGoOOMBanner() error = %v, want nil", gotErr)
			}
			if got.State() != tc.wantState || got.BytesExamined().Uint64() != tc.length {
				t.Fatalf(
					"ClassifyGoOOMBanner() = state %v bytes %d, want state %v bytes %d",
					got.State(),
					got.BytesExamined().Uint64(),
					tc.wantState,
					tc.length,
				)
			}
			if got.Validate() != nil {
				t.Fatalf("ClassifyGoOOMBanner().Validate() error = %v, want nil", got.Validate())
			}
		})
	}
}

func TestGoOOMBannerEverySplitPositionPreservesPresence(t *testing.T) {
	t.Parallel()

	for _, banner := range []string{GoOOMPlainBanner, GoOOMPrefixedBanner} {
		for split := 1; split < len(banner); split++ {
			reader := io.MultiReader(
				bytes.NewReader([]byte(banner[:split])),
				bytes.NewReader([]byte(banner[split:])),
			)
			got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
				Source: reader,
				Length: mustByteLength(t, uint64(len(banner))),
			})
			if gotErr != nil || got.State() != GoOOMBannerPresent {
				t.Fatalf("ClassifyGoOOMBanner(split %d of %d) = (%v, %v), want present", split, len(banner), got, gotErr)
			}
		}
	}
}

func TestGoOOMBannerRequestAndReaderFailureBoundaries(t *testing.T) {
	t.Parallel()

	overMaximum := GoOOMBannerRequest{
		Source: bytes.NewReader(nil),
		Length: mustByteLength(t, GoOOMMaximumEvidenceBytes+1),
	}
	if gotErr := overMaximum.Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("GoOOMBannerRequest(over maximum).Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}
	if gotErr := (GoOOMBannerRequest{}).Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("GoOOMBannerRequest{}.Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}

	native := errors.New("source failed")
	got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: errorReader{err: native},
		Length: mustByteLength(t, 1),
	})
	if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, core.ErrHostFactsObservation) || !errors.Is(gotErr, native) {
		t.Fatalf("ClassifyGoOOMBanner(native failure) = (%v, %v), want zero and both observation/native identities", got, gotErr)
	}

	got, gotErr = ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: zeroReader{},
		Length: mustByteLength(t, 1),
	})
	if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, io.ErrNoProgress) {
		t.Fatalf("ClassifyGoOOMBanner(no progress) = (%v, %v), want (zero, %v)", got, gotErr, io.ErrNoProgress)
	}

	got, gotErr = ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: invalidCountReader{},
		Length: mustByteLength(t, 1),
	})
	var failure Failure
	if got != (GoOOMBannerEvidence{}) ||
		!errors.As(gotErr, &failure) ||
		failure.Operation != OperationGoOOMBanner {
		t.Fatalf("ClassifyGoOOMBanner(invalid count) = (%v, %v), want one typed banner-operation failure", got, gotErr)
	}
	if _, ok := errors.AsType[Failure](failure.Cause); ok {
		t.Fatalf("ClassifyGoOOMBanner(invalid count) cause = %v, want leaf cause without nested Failure", failure.Cause)
	}
}

func TestGoOOMBannerEvidenceJSONHostileTable(t *testing.T) {
	t.Parallel()

	validCases := []struct {
		name   string
		extent uint64
		state  GoOOMBannerState
	}{
		{name: "zero-byte absence is canonical", extent: 0, state: GoOOMBannerAbsent},
		{name: "one-byte absence is canonical", extent: 1, state: GoOOMBannerAbsent},
		{name: "one below shortest banner remains canonical absence", extent: uint64(len(GoOOMPlainBanner) - 1), state: GoOOMBannerAbsent},
		{name: "shortest banner extent remains canonical absence", extent: uint64(len(GoOOMPlainBanner)), state: GoOOMBannerAbsent},
		{name: "shortest banner extent admits presence", extent: uint64(len(GoOOMPlainBanner)), state: GoOOMBannerPresent},
		{name: "prefixed banner extent admits presence", extent: uint64(len(GoOOMPrefixedBanner)), state: GoOOMBannerPresent},
		{name: "kilobyte absence is canonical", extent: 1 << 10, state: GoOOMBannerAbsent},
		{name: "kilobyte presence is canonical", extent: 1 << 10, state: GoOOMBannerPresent},
		{name: "one below evidence ceiling is canonical", extent: GoOOMMaximumEvidenceBytes - 1, state: GoOOMBannerAbsent},
		{name: "exact evidence ceiling is canonical", extent: GoOOMMaximumEvidenceBytes, state: GoOOMBannerPresent},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want := classifiedEvidenceFixture(t, tc.extent, tc.state)
			wire, gotErr := json.Marshal(want)
			if gotErr != nil {
				t.Fatalf("json.Marshal(%v) error = %v, want nil", want, gotErr)
			}
			var got GoOOMBannerEvidence
			gotErr = json.Unmarshal(wire, &got)
			if gotErr != nil || got != want {
				t.Fatalf("json.Unmarshal(%q) = (%v, %v), want (%v, nil)", wire, got, gotErr, want)
			}
			second, gotErr := got.MarshalJSON()
			if gotErr != nil || !bytes.Equal(second, wire) {
				t.Fatalf("GoOOMBannerEvidence.MarshalJSON(round trip) = (%q, %v), want (%q, nil)", second, gotErr, wire)
			}
		})
	}

	rejectedCases := []struct {
		name string
		wire string
	}{
		{name: "empty document is rejected", wire: ""},
		{name: "whitespace-only document is rejected", wire: " \t\n"},
		{name: "null document is rejected", wire: "null"},
		{name: "empty object is rejected", wire: "{}"},
		{name: "missing state is rejected", wire: `{"bytes_examined":0}`},
		{name: "missing extent is rejected", wire: `{"state":"absent"}`},
		{name: "null state is rejected", wire: `{"bytes_examined":0,"state":null}`},
		{name: "null extent is rejected", wire: `{"bytes_examined":null,"state":"absent"}`},
		{name: "unknown state is rejected", wire: `{"bytes_examined":0,"state":"unknown"}`},
		{name: "extra field is rejected", wire: `{"bytes_examined":0,"state":"absent","extra":0}`},
		{name: "reordered fields are rejected as noncanonical", wire: `{"state":"absent","bytes_examined":0}`},
		{name: "leading-zero extent is rejected", wire: `{"bytes_examined":00,"state":"absent"}`},
		{name: "negative extent is rejected", wire: `{"bytes_examined":-1,"state":"absent"}`},
		{name: "fractional extent is rejected", wire: `{"bytes_examined":1.0,"state":"absent"}`},
		{name: "string extent is rejected", wire: `{"bytes_examined":"1","state":"absent"}`},
		{name: "one over evidence ceiling is rejected", wire: fmt.Sprintf(`{"bytes_examined":%d,"state":"absent"}`, GoOOMMaximumEvidenceBytes+1)},
		{name: "trailing newline is rejected", wire: "{\"bytes_examined\":0,\"state\":\"absent\"}\n"},
		{name: "leading space is rejected", wire: ` {"bytes_examined":0,"state":"absent"}`},
		{name: "space before separator is rejected", wire: `{"bytes_examined":0, "state":"absent"}`},
		{name: "duplicate state is rejected", wire: `{"bytes_examined":0,"state":"absent","state":"present"}`},
		{name: "duplicate extent is rejected", wire: `{"bytes_examined":0,"bytes_examined":1,"state":"absent"}`},
		{name: "maximum uint64 extent is rejected", wire: `{"bytes_examined":18446744073709551615,"state":"absent"}`},
		{name: "array document is rejected", wire: `[]`},
		{name: "boolean document is rejected", wire: `true`},
		{name: "uppercase state is rejected", wire: `{"bytes_examined":0,"state":"ABSENT"}`},
		{name: "empty state is rejected", wire: `{"bytes_examined":0,"state":""}`},
		{name: "escaped state spelling is rejected as noncanonical", wire: `{"bytes_examined":0,"state":"ab\u0073ent"}`},
		{name: "presence below shortest banner is rejected", wire: fmt.Sprintf(`{"bytes_examined":%d,"state":"present"}`, len(GoOOMPlainBanner)-1)},
		{name: "nested extent is rejected", wire: `{"bytes_examined":{"value":0},"state":"absent"}`},
		{name: "second document is rejected", wire: `{"bytes_examined":0,"state":"absent"}{}`},
	}
	wantPreserved := classifiedEvidenceFixture(t, uint64(len(GoOOMPlainBanner)), GoOOMBannerPresent)
	for _, tc := range rejectedCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := wantPreserved
			gotErr := got.UnmarshalJSON([]byte(tc.wire))
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
				t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(%q) error = %v, want JSON and evidence identities", tc.wire, gotErr)
			}
			if got != wantPreserved {
				t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(%q) receiver = %v, want preserved %v", tc.wire, got, wantPreserved)
			}
		})
	}

	var nilEvidence *GoOOMBannerEvidence
	if gotErr := nilEvidence.UnmarshalJSON([]byte(`{"bytes_examined":0,"state":"absent"}`)); !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
		t.Fatalf("nil GoOOMBannerEvidence.UnmarshalJSON() error = %v, want JSON and evidence identities", gotErr)
	}
}

func TestGoOOMBannerStateJSONExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		state := GoOOMBannerState(raw)
		wire, gotErr := state.MarshalJSON()
		wantValid := state == GoOOMBannerAbsent || state == GoOOMBannerPresent
		if !wantValid {
			if wire != nil || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
				t.Fatalf("GoOOMBannerState(%d).MarshalJSON() = (%s, %v), want (nil, %v)", raw, wire, gotErr, core.ErrHostFactsEvidence)
			}
			continue
		}
		if gotErr != nil {
			t.Fatalf("GoOOMBannerState(%d).MarshalJSON() error = %v, want nil", raw, gotErr)
		}
		var roundTrip GoOOMBannerState
		if gotErr := roundTrip.UnmarshalJSON(wire); gotErr != nil || roundTrip != state {
			t.Fatalf("GoOOMBannerState.UnmarshalJSON(%s) = (%v, %v), want (%v, nil)", wire, roundTrip, gotErr, state)
		}
	}

	rejectedCases := []struct {
		name string
		wire string
	}{
		{name: "empty document", wire: ""},
		{name: "null document", wire: "null"},
		{name: "empty token", wire: `""`},
		{name: "unknown token", wire: `"unknown"`},
		{name: "uppercase token", wire: `"Present"`},
		{name: "trailing space", wire: `"present" `},
		{name: "leading space", wire: ` "present"`},
		{name: "escaped canonical token", wire: `"pre\u0073ent"`},
		{name: "embedded NUL", wire: `"present\u0000"`},
		{name: "unquoted token", wire: `present`},
		{name: "array token", wire: `[]`},
		{name: "object token", wire: `{}`},
		{name: "numeric token", wire: `1`},
		{name: "boolean token", wire: `true`},
		{name: "two documents", wire: `"present""absent"`},
		{name: "truncated quote", wire: `"present`},
		{name: "invalid escape", wire: `"pre\qsent"`},
		{name: "surrogate escape", wire: `"\ud800"`},
		{name: "newline after token", wire: "\"present\"\n"},
		{name: "maximum plus one bytes", wire: strings.Repeat("x", len(strconv.Quote(goOOMPresentToken))+1)},
	}
	for _, tc := range rejectedCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := GoOOMBannerPresent
			gotErr := got.UnmarshalJSON([]byte(tc.wire))
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
				t.Fatalf("GoOOMBannerState.UnmarshalJSON(%q) error = %v, want JSON and evidence identities", tc.wire, gotErr)
			}
			if got != GoOOMBannerPresent {
				t.Fatalf("GoOOMBannerState.UnmarshalJSON(%q) receiver = %v, want preserved %v", tc.wire, got, GoOOMBannerPresent)
			}
		})
	}

	var nilState *GoOOMBannerState
	if gotErr := nilState.UnmarshalJSON([]byte(`"absent"`)); !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
		t.Fatalf("nil GoOOMBannerState.UnmarshalJSON() error = %v, want JSON and evidence identities", gotErr)
	}
}

func FuzzGoOOMBannerStateJSONSemanticClosure(f *testing.F) {
	for _, state := range []GoOOMBannerState{GoOOMBannerAbsent, GoOOMBannerPresent} {
		seed, err := state.MarshalJSON()
		if err != nil {
			f.Fatalf("GoOOMBannerState(%d).MarshalJSON(seed) error = %v, want nil", state, err)
		}
		f.Add(seed)
	}
	for _, seed := range [][]byte{
		{},
		[]byte("null"),
		[]byte(`"unknown"`),
		[]byte(`"pre\u0073ent"`),
		[]byte(strings.Repeat("x", len(strconv.Quote(goOOMPresentToken))+1)),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		want, wantErr := referenceGoOOMBannerStateJSON(data)
		got := GoOOMBannerPresent
		gotErr := got.UnmarshalJSON(data)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
				t.Fatalf("GoOOMBannerState.UnmarshalJSON(rejected %q) error = %v, want JSON and evidence identities", data, gotErr)
			}
			if got != GoOOMBannerPresent {
				t.Fatalf("GoOOMBannerState.UnmarshalJSON(rejected %q) receiver = %v, want preserved %v", data, got, GoOOMBannerPresent)
			}
			return
		}
		if gotErr != nil || got != want {
			t.Fatalf("GoOOMBannerState.UnmarshalJSON(%q) = (%v, %v), want (%v, nil)", data, got, gotErr, want)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("GoOOMBannerState.UnmarshalJSON(%q).Validate() error = %v, want nil", data, err)
		}
		canonical, err := got.MarshalJSON()
		if err != nil || !bytes.Equal(canonical, data) {
			t.Fatalf("GoOOMBannerState.MarshalJSON(accepted) = (%q, %v), want (%q, nil)", canonical, err, data)
		}
		var roundTrip GoOOMBannerState
		if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != got {
			t.Fatalf("GoOOMBannerState canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzGoOOMBannerEvidenceJSONSemanticClosure(f *testing.F) {
	for _, fixture := range []GoOOMBannerEvidence{
		classifiedEvidenceFixture(f, 0, GoOOMBannerAbsent),
		classifiedEvidenceFixture(f, uint64(len(GoOOMPlainBanner)), GoOOMBannerPresent),
		classifiedEvidenceFixture(f, 1<<10, GoOOMBannerAbsent),
	} {
		seed, err := fixture.MarshalJSON()
		if err != nil {
			f.Fatalf("GoOOMBannerEvidence.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(seed)
	}
	for _, seed := range [][]byte{
		{},
		[]byte("null"),
		[]byte(`{}`),
		[]byte(`{"bytes_examined":0,"state":"unknown"}`),
		[]byte(fmt.Sprintf(`{"bytes_examined":%d,"state":"absent"}`, GoOOMMaximumEvidenceBytes+1)),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		want, wantErr := referenceGoOOMBannerEvidenceJSON(data)
		preserved := classifiedEvidenceFixture(t, uint64(len(GoOOMPlainBanner)), GoOOMBannerPresent)
		got := preserved
		gotErr := got.UnmarshalJSON(data)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
				t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(rejected %d bytes) error = %v, want JSON and evidence identities", len(data), gotErr)
			}
			if got != preserved {
				t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(rejected %d bytes) receiver = %v, want preserved %v", len(data), got, preserved)
			}
			return
		}
		if gotErr != nil || got != want {
			t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(%q) = (%v, %v), want (%v, nil)", data, got, gotErr, want)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(%q).Validate() error = %v, want nil", data, err)
		}
		canonical, err := got.MarshalJSON()
		if err != nil || !bytes.Equal(canonical, data) {
			t.Fatalf("GoOOMBannerEvidence.MarshalJSON(accepted) = (%q, %v), want (%q, nil)", canonical, err, data)
		}
		var roundTrip GoOOMBannerEvidence
		if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != got {
			t.Fatalf("GoOOMBannerEvidence canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("GoOOMBannerEvidence second canonical projection = (%q, %v), want (%q, nil)", second, err, canonical)
		}
	})
}

func FuzzGoOOMBannerClassifier(f *testing.F) {
	f.Add([]byte(GoOOMPlainBanner), uint32(1))
	f.Add([]byte(GoOOMPrefixedBanner), uint32(7))
	f.Add([]byte("fatal error: out of memorx"), uint32(3))
	f.Add([]byte{}, uint32(1))

	f.Fuzz(func(t *testing.T, data []byte, chunk uint32) {
		if len(data) > GoOOMMaximumEvidenceBytes {
			return
		}
		maximum := int(chunk%uint32(goOOMBufferBytes)) + 1
		got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
			Source: &chunkReader{data: append([]byte(nil), data...), maximum: maximum},
			Length: mustByteLength(t, uint64(len(data))),
		})
		if gotErr != nil {
			t.Fatalf("ClassifyGoOOMBanner(%d bytes) error = %v, want nil", len(data), gotErr)
		}
		wantPresent := bytes.Contains(data, []byte(GoOOMPlainBanner)) ||
			bytes.Contains(data, []byte(GoOOMPrefixedBanner))
		if gotPresent := got.State() == GoOOMBannerPresent; gotPresent != wantPresent {
			t.Fatalf("ClassifyGoOOMBanner(%d bytes).State() = %v, want present %t", len(data), got.State(), wantPresent)
		}
		if got.BytesExamined().Uint64() != uint64(len(data)) || got.Validate() != nil {
			t.Fatalf("ClassifyGoOOMBanner(%d bytes) emitted invalid extent/state: %v", len(data), got)
		}
	})
}

func classifiedEvidenceFixture(t testing.TB, extent uint64, state GoOOMBannerState) GoOOMBannerEvidence {
	t.Helper()

	data := bytes.Repeat([]byte{'x'}, int(extent))
	if state == GoOOMBannerPresent {
		copy(data[len(data)-len(GoOOMPlainBanner):], GoOOMPlainBanner)
	}
	got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: bytes.NewReader(data),
		Length: mustByteLength(t, extent),
	})
	if gotErr != nil {
		t.Fatalf("ClassifyGoOOMBanner(%d-byte fixture) error = %v, want nil", extent, gotErr)
	}
	if got.State() != state || got.BytesExamined().Uint64() != extent {
		t.Fatalf("ClassifyGoOOMBanner(%d-byte fixture) = %v, want state %v and exact extent", extent, got, state)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("ClassifyGoOOMBanner(%d-byte fixture).Validate() error = %v, want nil", extent, err)
	}
	return got
}

func referenceGoOOMBannerStateJSON(data []byte) (GoOOMBannerState, error) {
	if len(data) == 0 || len(data) > len(strconv.Quote(goOOMPresentToken)) {
		return GoOOMBannerUnknown, core.ErrJSONContract
	}
	token, err := strconv.Unquote(string(data))
	if err != nil || !bytes.Equal(strconv.AppendQuote(nil, token), data) {
		return GoOOMBannerUnknown, core.ErrJSONContract
	}
	switch token {
	case goOOMAbsentToken:
		return GoOOMBannerAbsent, nil
	case goOOMPresentToken:
		return GoOOMBannerPresent, nil
	default:
		return GoOOMBannerUnknown, core.ErrJSONContract
	}
}

func referenceGoOOMBannerEvidenceJSON(data []byte) (GoOOMBannerEvidence, error) {
	maximum, err := goOOMEvidenceJSONLimits().DocumentMaximumBytes.Uint64()
	if err != nil || len(data) == 0 || uint64(len(data)) > maximum || data[len(data)-1] != '}' {
		return GoOOMBannerEvidence{}, core.ErrJSONContract
	}
	if !bytes.HasPrefix(data, []byte(goOOMEvidenceBytesPrefix)) {
		return GoOOMBannerEvidence{}, core.ErrJSONContract
	}
	body := data[len(goOOMEvidenceBytesPrefix) : len(data)-1]
	extentToken, stateToken, found := bytes.Cut(body, []byte(goOOMEvidenceStatePrefix))
	if !found || len(extentToken) == 0 || bytes.Contains(stateToken, []byte(goOOMEvidenceStatePrefix)) {
		return GoOOMBannerEvidence{}, core.ErrJSONContract
	}
	extent, err := strconv.ParseUint(string(extentToken), 10, 64)
	if err != nil || strconv.FormatUint(extent, 10) != string(extentToken) || extent > GoOOMMaximumEvidenceBytes {
		return GoOOMBannerEvidence{}, core.ErrJSONContract
	}
	state, err := referenceGoOOMBannerStateJSON(stateToken)
	if err != nil || state == GoOOMBannerPresent && extent < uint64(len(GoOOMPlainBanner)) {
		return GoOOMBannerEvidence{}, core.ErrJSONContract
	}
	length, err := core.NewByteLength(extent)
	if err != nil {
		return GoOOMBannerEvidence{}, core.ErrJSONContract
	}
	return GoOOMBannerEvidence{examined: length, state: state}, nil
}

func BenchmarkClassifyGoOOMBanner1KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkGoOOMBanner(b, 1<<10)
}

func BenchmarkClassifyGoOOMBanner1MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkGoOOMBanner(b, 1<<20)
}

func benchmarkGoOOMBanner(b *testing.B, size int) {
	b.Helper()
	data := bytes.Repeat([]byte{'x'}, size)
	length := mustByteLength(b, uint64(size))
	b.ResetTimer()

	for b.Loop() {
		_, err := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
			Source: bytes.NewReader(data),
			Length: length,
		})
		if err != nil {
			b.Fatalf("ClassifyGoOOMBanner(%d bytes) error = %v", size, err)
		}
	}
}

type chunkReader struct {
	data    []byte
	maximum int
}

func (r *chunkReader) Read(destination []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	count := min(len(destination), len(r.data), r.maximum)
	copy(destination, r.data[:count])
	r.data = r.data[count:]
	return count, nil
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

type zeroReader struct{}

func (zeroReader) Read([]byte) (int, error) {
	return 0, nil
}

type invalidCountReader struct{}

func (invalidCountReader) Read(destination []byte) (int, error) {
	return len(destination) + 1, nil
}
