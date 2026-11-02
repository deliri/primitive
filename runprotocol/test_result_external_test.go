package runprotocol_test

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

func testLabel(t testing.TB, s string) runprotocol.TestLabel {
	t.Helper()
	v, e := runprotocol.NewTestLabel(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func testResultCodec() jsonio.OwnedCodecPolicy {
	return jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly, Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullRefused}
}
func runResultFixture(t testing.TB) runprotocol.RunResult {
	t.Helper()
	name := testLabel(t, "TestNative/dynamic/subtest")
	outcome := runprotocol.OutcomePassed
	return runprotocol.RunResult{Kind: runprotocol.RunResultTest, Suite: testLabel(t, "example.org/project/pkg"), Name: &name, Outcome: &outcome}
}
func TestRunResultLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		edit    func(*runprotocol.RunResult)
		wantErr error
	}{
		{name: "native test retains suite and dynamic name", edit: func(*runprotocol.RunResult) {}},
		{name: "package terminal invents no test name", edit: func(r *runprotocol.RunResult) { r.Kind = runprotocol.RunResultPackage; r.Name = nil }},
		{name: "explicit fuzz terminal retains actual kind", edit: func(r *runprotocol.RunResult) {
			r.Kind = runprotocol.RunResultFuzz
			v := testLabel(t, "FuzzNative")
			r.Name = &v
		}},
		{name: "Fuzz prefix does not change a test kind", edit: func(r *runprotocol.RunResult) { v := testLabel(t, "FuzzNative/seed"); r.Name = &v }},
		{name: "zero observed elapsed remains present", edit: func(r *runprotocol.RunResult) { v := temporal.Duration{}; r.Elapsed = &v }},
		{name: "unobserved elapsed stays absent", edit: func(r *runprotocol.RunResult) { r.Elapsed = nil }},
		{name: "skipped target remains skipped", edit: func(r *runprotocol.RunResult) { v := runprotocol.OutcomeSkipped; r.Outcome = &v }},
		{name: "failed target remains failed", edit: func(r *runprotocol.RunResult) { v := runprotocol.OutcomeFailed; r.Outcome = &v }},
		{name: "unknown result kind refuses", edit: func(r *runprotocol.RunResult) { r.Kind = runprotocol.RunResultUnknown }, wantErr: core.ErrRunProtocolContract},
		{name: "absent suite cannot borrow another target", edit: func(r *runprotocol.RunResult) { r.Suite = runprotocol.TestLabel{} }, wantErr: core.ErrRunProtocolContract},
		{name: "test name cannot be absent", edit: func(r *runprotocol.RunResult) { r.Name = nil }, wantErr: core.ErrRunProtocolContract},
		{name: "unset nominal name cannot replace absent name", edit: func(r *runprotocol.RunResult) { r.Name = &runprotocol.TestLabel{} }, wantErr: core.ErrRunProtocolContract},
		{name: "package cannot invent a test occurrence", edit: func(r *runprotocol.RunResult) { r.Kind = runprotocol.RunResultPackage }, wantErr: core.ErrRunProtocolConflict},
		{name: "terminal target needs observed outcome", edit: func(r *runprotocol.RunResult) { r.Outcome = nil }, wantErr: core.ErrRunProtocolContract},
		{name: "benchmark cannot borrow a test outcome", edit: func(r *runprotocol.RunResult) {
			r.Kind = runprotocol.RunResultBenchmark
			r.Benchmark = &runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMetricsMissing}
		}, wantErr: core.ErrRunProtocolConflict},
		{name: "benchmark requires its actual native row", edit: func(r *runprotocol.RunResult) { r.Kind = runprotocol.RunResultBenchmark; r.Outcome = nil }, wantErr: core.ErrRunProtocolConflict},
		{name: "missing metrics remains an explicit row", edit: func(r *runprotocol.RunResult) {
			r.Kind = runprotocol.RunResultBenchmark
			r.Outcome = nil
			r.Benchmark = &runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowMetricsMissing}
		}},
		{name: "unrepresentable benchmark name remains unobserved", edit: func(r *runprotocol.RunResult) {
			r.Kind = runprotocol.RunResultBenchmark
			r.Name = nil
			r.Outcome = nil
			r.Benchmark = &runprotocol.GoBenchmarkRow{Kind: runprotocol.GoBenchmarkRowNameUnrepresentable}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := runResultFixture(t)
			tc.edit(&r)
			err := r.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("RunResult.Validate = %v, want %v", err, tc.wantErr)
			}
			encoded, err := jsonio.MarshalValidated(r, testResultCodec())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(encoded) != 0 {
					t.Fatalf("refused result encoding = (%d,%v), want zero/%v", len(encoded), err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			round, err := jsonio.DecodeOwned[runprotocol.RunResult](encoded, testResultCodec())
			if err != nil {
				t.Fatal(err)
			}
			second, err := jsonio.MarshalValidated(round, testResultCodec())
			if err != nil || !bytes.Equal(second, encoded) {
				t.Fatalf("result roundtrip = (%d,%v), want %d identical bytes", len(second), err, len(encoded))
			}
		})
	}
}
func TestRunResultExhaustsOutcomeDomain(t *testing.T) {
	t.Parallel()
	for n := 0; n <= math.MaxUint8; n++ {
		r := runResultFixture(t)
		v := runprotocol.Outcome(n)
		r.Outcome = &v
		want := v == runprotocol.OutcomePassed || v == runprotocol.OutcomeFailed || v == runprotocol.OutcomeSkipped
		err := r.Validate()
		if (err == nil) != want {
			t.Fatalf("RunResult.Validate outcome%d = %v, want admitted%t", n, err, want)
		}
	}
}
func TestRunResultJSONHostileAdmission(t *testing.T) {
	t.Parallel()
	good, err := jsonio.MarshalValidated(runResultFixture(t), testResultCodec())
	if err != nil {
		t.Fatal(err)
	}
	replace := func(old, next string) []byte {
		b := bytes.Replace(good, []byte(old), []byte(next), 1)
		if bytes.Equal(b, good) {
			t.Fatalf("hostile mutation %q changed no bytes, want real boundary mutation", old)
		}
		return b
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty source", nil}, {"null root", []byte("null")}, {"array root", []byte("[]")},
		{"missing native kind", replace("\"kind\":\"test\",", "")},
		{"future native kind", replace("\"kind\":\"test\"", "\"kind\":\"future\"")},
		{"wrong kind representation", replace("\"kind\":\"test\"", "\"kind\":9")},
		{"missing native suite", replace("\"suite\":\"example.org/project/pkg\",", "")},
		{"empty native suite", replace("example.org/project/pkg", "")},
		{"missing native name", replace("\"name\":\"TestNative/dynamic/subtest\",", "")},
		{"null name is not unobserved", replace("\"name\":\"TestNative/dynamic/subtest\"", "\"name\":null")},
		{"duplicate suite member", replace("\"suite\":", "\"suite\":\"foreign\",\"suite\":")},
		{"unknown extra member", replace("\"suite\":", "\"future\":true,\"suite\":")},
		{"null duration", replace("\"outcome\":", "\"elapsed\":null,\"outcome\":")},
		{"negative duration", replace("\"outcome\":", "\"elapsed\":\"-1\",\"outcome\":")},
		{"duration overflow", replace("\"outcome\":", "\"elapsed\":\"9223372036854775808\",\"outcome\":")},
		{"wrong duration representation", replace("\"outcome\":", "\"elapsed\":1,\"outcome\":")},
		{"missing terminal outcome", replace(",\"outcome\":\"outcome_passed\"", "")},
		{"unknown terminal outcome", replace("outcome_passed", "future")},
		{"truncated record", good[:len(good)-1]},
		{"two roots at document boundary", append(append([]byte{}, good...), good...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.DecodeOwned[runprotocol.RunResult](tc.data, testResultCodec())
			if !errors.Is(err, core.ErrJSONContract) || got != (runprotocol.RunResult{}) {
				t.Fatalf("hostile result = (%+v,%v), want zero/JSON contract", got, err)
			}
		})
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
			result := runResultFixture(t)
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
			got, err := jsonio.DecodeOwned[runprotocol.RunResult]([]byte(document), testResultCodec())
			if (err == nil) != row.admit || row.admit && got.Name.String() != row.value {
				t.Fatalf("decoded label = (%d bytes,%v), want admitted %t with exact bytes", len(got.Name.String()), err, row.admit)
			}
		})
	}
}
