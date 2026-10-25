package runprotocol_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

func testLabel(t testing.TB, value string) runprotocol.TestLabel {
	t.Helper()
	label, err := runprotocol.NewTestLabel(value)
	if err != nil {
		t.Fatal(err)
	}
	return label
}

func testResultFixture(t testing.TB) runprotocol.TestResult {
	t.Helper()
	return runprotocol.TestResult{Test: runprotocol.TestIdentity{Suite: testLabel(t, "example.org/project/pkg"), Name: testLabel(t, "TestNative/dynamic/subtest")}, Ordinal: 1, Outcome: runprotocol.OutcomePassed}
}

func testResultCodec() jsonio.OwnedCodecPolicy {
	return jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly, Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullRefused}
}

// The shared record is an observation, not an inferred lifecycle or acceptance.
func TestTestResultLayerTriad(t *testing.T) {
	t.Parallel()
	zeroElapsed := temporal.Duration{}
	elapsed, err := temporal.DurationFromNanoseconds(math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		edit    func(*runprotocol.TestResult)
		wantErr error
	}{
		{name: "positive ordinary native result", edit: func(*runprotocol.TestResult) {}},
		{name: "positive observed zero duration remains present", edit: func(r *runprotocol.TestResult) { r.Elapsed = &zeroElapsed }},
		{name: "positive maximum native duration", edit: func(r *runprotocol.TestResult) { r.Elapsed = &elapsed }},
		{name: "positive maximum occurrence ordinal", edit: func(r *runprotocol.TestResult) { r.Ordinal = math.MaxUint64 }},
		{name: "negative ordinal zero cannot name an occurrence", edit: func(r *runprotocol.TestResult) { r.Ordinal = 0 }, wantErr: core.ErrRunProtocolContract},
		{name: "negative absent suite cannot borrow another result identity", edit: func(r *runprotocol.TestResult) { r.Test.Suite = runprotocol.TestLabel{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative absent name cannot become package success", edit: func(r *runprotocol.TestResult) { r.Test.Name = runprotocol.TestLabel{} }, wantErr: core.ErrRunProtocolContract},
		{name: "neutral absent duration stays absent", edit: func(r *runprotocol.TestResult) { r.Outcome = runprotocol.OutcomeUnavailable }},
		{name: "neutral skipped result stays skipped", edit: func(r *runprotocol.TestResult) { r.Outcome = runprotocol.OutcomeSkipped; r.Elapsed = &zeroElapsed }},
		{name: "neutral no result is not an empty pass", edit: func(r *runprotocol.TestResult) { *r = runprotocol.TestResult{} }, wantErr: core.ErrRunProtocolContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := testResultFixture(t)
			tc.edit(&got)
			err := got.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("TestResult.Validate(%+v) error=%v, want %v", got, err, tc.wantErr)
			}
			encoded, encodeErr := jsonio.MarshalValidated(got, testResultCodec())
			if tc.wantErr != nil {
				if !errors.Is(encodeErr, tc.wantErr) || len(encoded) != 0 {
					t.Fatalf("rejected result encoding=(%q,%v), want empty and %v", encoded, encodeErr, tc.wantErr)
				}
				return
			}
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			roundTrip, decodeErr := jsonio.DecodeOwned[runprotocol.TestResult](encoded, testResultCodec())
			if decodeErr != nil || roundTrip.Test != got.Test || roundTrip.Ordinal != got.Ordinal || roundTrip.Outcome != got.Outcome || (roundTrip.Elapsed == nil) != (got.Elapsed == nil) {
				t.Fatalf("round trip=(%+v,%v), want %+v", roundTrip, decodeErr, got)
			}
			if got.Elapsed != nil && *got.Elapsed != *roundTrip.Elapsed {
				t.Fatalf("round trip duration=%v, want %v", roundTrip.Elapsed, got.Elapsed)
			}
		})
	}
}

func TestTestResultExhaustsExistingOutcomeDomain(t *testing.T) {
	t.Parallel()
	// Pin the existing closed domain independently of Outcome.Validate.
	for number := 0; number <= math.MaxUint8; number++ {
		result := testResultFixture(t)
		result.Outcome = runprotocol.Outcome(number)
		admitted := number >= int(runprotocol.OutcomePassed) && number <= int(runprotocol.OutcomeNonAccepting)
		err := result.Validate()
		if admitted && err != nil || !admitted && !errors.Is(err, core.ErrRunProtocolContract) {
			t.Fatalf("TestResult.Validate(outcome %d) error=%v, want admitted=%t", number, err, admitted)
		}
	}
}

// Labels preserve exact nonempty UTF-8 native data across multiple windows;
// a former working/acceptance boundary cannot truncate or reject valid data.
func TestTestLabelBoundary(t *testing.T) {
	t.Parallel()
	native := "TestNative/é界😀\t\"\\"
	for _, row := range []struct {
		name  string
		value string
		admit bool
	}{
		{name: "native punctuation and control characters", value: native, admit: true},
		{name: "one below former quota", value: native + strings.Repeat("x", 4095-len(native)), admit: true},
		{name: "former quota is ordinary valid data", value: native + strings.Repeat("x", 4096-len(native)), admit: true},
		{name: "one beyond former quota remains exact", value: native + strings.Repeat("x", 4097-len(native)), admit: true},
		{name: "a multi-byte rune crossing former quota", value: strings.Repeat("x", 4095) + "é", admit: true},
		{name: "label beyond a mebibyte remains exact", value: native + strings.Repeat("x", (1<<20)+1-len(native)), admit: true},
		{name: "invalid UTF-8", value: string([]byte{0xff})},
		{name: "empty", value: ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			label, err := runprotocol.NewTestLabel(row.value)
			if (err == nil) != row.admit || !row.admit && (!errors.Is(err, core.ErrRunProtocolContract) || label != (runprotocol.TestLabel{})) {
				t.Fatalf("NewTestLabel(%d bytes) = (%q,%v), want admitted %t", len(row.value), label.String(), err, row.admit)
			}
			result := testResultFixture(t)
			quoted, err := core.MarshalCanonicalJSONString(row.value)
			if err != nil {
				if row.admit {
					t.Fatal(err)
				}
				return
			}
			encoded, err := jsonio.MarshalValidated(result, testResultCodec())
			if err != nil {
				t.Fatal(err)
			}
			document := strings.Replace(string(encoded), `"TestNative/dynamic/subtest"`, string(quoted), 1)
			got, err := jsonio.DecodeOwned[runprotocol.TestResult]([]byte(document), testResultCodec())
			if (err == nil) != row.admit || row.admit && got.Test.Name.String() != row.value {
				t.Fatalf("decoded label = (%d bytes,%v), want admitted %t with exact bytes", len(got.Test.Name.String()), err, row.admit)
			}
		})
	}
}

func TestTestResultJSONHostileAdmission(t *testing.T) {
	t.Parallel()
	good, err := jsonio.MarshalValidated(testResultFixture(t), testResultCodec())
	if err != nil {
		t.Fatal(err)
	}
	replace := func(old, new string) string { return strings.Replace(string(good), old, new, 1) }
	for _, tc := range []struct{ name, input string }{
		{"empty source", ""}, {"whitespace source", " \n\t"}, {"null root", "null"}, {"array root", "[]"},
		{"missing identity", `{"ordinal":1,"outcome":"outcome_passed"}`},
		{"missing suite", `{"test":{"name":"TestX"},"ordinal":1,"outcome":"outcome_passed"}`},
		{"missing name", `{"test":{"suite":"pkg"},"ordinal":1,"outcome":"outcome_passed"}`},
		{"missing ordinal", replace(`"ordinal":1,`, ``)},
		{"zero ordinal", replace(`"ordinal":1`, `"ordinal":0`)},
		{"negative ordinal", replace(`"ordinal":1`, `"ordinal":-1`)},
		{"fractional ordinal", replace(`"ordinal":1`, `"ordinal":1.5`)},
		{"ordinal overflow", replace(`"ordinal":1`, `"ordinal":18446744073709551616`)},
		{"unknown outcome", replace(`"outcome_passed"`, `"future"`)},
		{"wrong outcome type", replace(`"outcome_passed"`, `1`)},
		{"duplicate root member", replace(`"ordinal":1`, `"ordinal":1,"ordinal":2`)},
		{"duplicate identity member", replace(`"name":`, `"name":"another","name":`)},
		{"unknown root member", replace(`"ordinal":1`, `"future":true,"ordinal":1`)},
		{"unknown identity member", replace(`"name":`, `"future":true,"name":`)},
		{"null identity", replace(`"test":{`+strings.Split(strings.Split(string(good), `"test":{`)[1], `}`)[0]+`}`, `"test":null`)},
		{"null duration", replace(`"ordinal":1`, `"elapsed":null,"ordinal":1`)},
		{"negative duration", replace(`"ordinal":1`, `"elapsed":"-1","ordinal":1`)},
		{"numeric duration cannot replace exact duration string", replace(`"ordinal":1`, `"elapsed":1,"ordinal":1`)},
		{"duration overflow", replace(`"ordinal":1`, `"elapsed":"9223372036854775808","ordinal":1`)},
		{"fractional duration", replace(`"ordinal":1`, `"elapsed":"0.5","ordinal":1`)},
		{"missing terminal outcome", replace(`,"outcome":"outcome_passed"`, ``)},
		{"truncated object", string(good[:len(good)-1])}, {"trailing prose", string(good) + " false"},
		{"multiple roots at single document boundary", string(good) + string(good)},
		{"native invalid UTF8", replace(`TestNative`, string([]byte{0xff}))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.DecodeOwned[runprotocol.TestResult]([]byte(tc.input), testResultCodec())
			if !errors.Is(err, core.ErrJSONContract) || got != (runprotocol.TestResult{}) {
				t.Fatalf("DecodeOwned(%q)=(%+v,%v), want zero and JSON contract refusal", tc.input, got, err)
			}
		})
	}
}

func TestTestResultStreamLayerTriad(t *testing.T) {
	t.Parallel()
	t.Run("positive repeated names keep every native occurrence", func(t *testing.T) {
		t.Parallel()
		var source bytes.Buffer
		encoder, err := jsonio.NewObjectEncoder[runprotocol.TestResult](jsonio.ObjectDestinationRequest{Destination: &source})
		if err != nil {
			t.Fatal(err)
		}
		outcomes := []runprotocol.Outcome{runprotocol.OutcomeFailed, runprotocol.OutcomeSkipped, runprotocol.OutcomePassed}
		for i, outcome := range outcomes {
			result := testResultFixture(t)
			result.Ordinal = uint64(i + 1)
			result.Outcome = outcome
			if err := encoder.Encode(t.Context(), result); err != nil {
				t.Fatal(err)
			}
		}
		count := 0
		for result, err := range jsonio.Objects[runprotocol.TestResult](t.Context(), jsonio.ObjectSourceRequest{Source: &source}) {
			if err != nil || count >= len(outcomes) || result.Test != testResultFixture(t).Test || result.Ordinal != uint64(count+1) || result.Outcome != outcomes[count] {
				t.Fatalf("result%d=(%+v,%v), want exact ordered native occurrence", count, result, err)
			}
			count++
		}
		if count != len(outcomes) {
			t.Fatalf("result count=%d, want %d", count, len(outcomes))
		}
	})
	t.Run("negative truncated source cannot invent a terminal result", func(t *testing.T) {
		t.Parallel()
		good, err := jsonio.MarshalValidated(testResultFixture(t), testResultCodec())
		if err != nil {
			t.Fatal(err)
		}
		source := io.MultiReader(bytes.NewReader(good), strings.NewReader("\n{"))
		count, refusals := 0, 0
		for result, err := range jsonio.Objects[runprotocol.TestResult](t.Context(), jsonio.ObjectSourceRequest{Source: source}) {
			if err != nil {
				if !errors.Is(err, core.ErrJSONContract) || result != (runprotocol.TestResult{}) {
					t.Fatalf("truncation=(%+v,%v), want zero and typed refusal", result, err)
				}
				refusals++
				continue
			}
			count++
		}
		if count != 1 || refusals != 1 {
			t.Fatalf("truncated stream=(%d results,%d refusals), want 1 prefix result and 1 refusal", count, refusals)
		}
	})
	t.Run("neutral empty source emits no result", func(t *testing.T) {
		t.Parallel()
		for result, err := range jsonio.Objects[runprotocol.TestResult](t.Context(), jsonio.ObjectSourceRequest{Source: strings.NewReader("")}) {
			t.Fatalf("empty stream=(%+v,%v), want no observations", result, err)
		}
	})
}

func FuzzTestResultJSONSemanticClosure(f *testing.F) {
	seed, err := jsonio.MarshalValidated(testResultFixture(f), testResultCodec())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	observedZero := testResultFixture(f)
	observedZero.Elapsed = new(temporal.Duration)
	zeroSeed, err := jsonio.MarshalValidated(observedZero, testResultCodec())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(zeroSeed)
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := jsonio.DecodeOwned[runprotocol.TestResult](data, testResultCodec())
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || got != (runprotocol.TestResult{}) {
				t.Fatalf("rejected result=(%+v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("admitted result.Validate()=%v, want nil", err)
		}
		canonical, err := jsonio.MarshalValidated(got, testResultCodec())
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := jsonio.DecodeOwned[runprotocol.TestResult](canonical, testResultCodec())
		if err != nil || roundTrip.Test != got.Test || roundTrip.Ordinal != got.Ordinal || roundTrip.Outcome != got.Outcome || (roundTrip.Elapsed == nil) != (got.Elapsed == nil) {
			t.Fatalf("round trip=(%+v,%v), want %+v", roundTrip, err, got)
		}
		if got.Elapsed != nil && *got.Elapsed != *roundTrip.Elapsed {
			t.Fatalf("round trip duration=%v, want %v", roundTrip.Elapsed, got.Elapsed)
		}
		second, err := jsonio.MarshalValidated(roundTrip, testResultCodec())
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("canonical second write=(%q,%v), want %q", second, err, canonical)
		}
	})
}
