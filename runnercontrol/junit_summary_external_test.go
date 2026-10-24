package runnercontrol_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestJUnitSummaryCompilerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, document string
		want           runnercontrol.JUnitSummary
		executionErr   error
		wantErr        bool
	}{
		{name: "passing case is observed without a plan", document: `<testsuite><testcase/></testsuite>`, want: runnercontrol.JUnitSummary{Passed: 1}},
		{name: "failed case agrees with failed process", document: `<testsuite><testcase><failure/></testcase></testsuite>`, executionErr: context.Canceled, want: runnercontrol.JUnitSummary{Failed: 1}},
		{name: "skipped case is neutral observed evidence", document: `<testsuite><testcase><skipped/></testcase></testsuite>`, want: runnercontrol.JUnitSummary{Skipped: 1}},
		{name: "empty sibling suite invents no cases", document: `<testsuites><testsuite/><testsuite><testcase/></testsuite></testsuites>`, want: runnercontrol.JUnitSummary{Passed: 1}},
		{name: "nested suites resume parent case ownership", document: `<testsuite><testsuite><testcase/></testsuite><testcase/></testsuite>`, want: runnercontrol.JUnitSummary{Passed: 2}},
		{name: "mixed observed counters preserve failure precedence", document: `<testsuite><testcase/><testcase><disabled/></testcase><testcase><skipped/><error/></testcase></testsuite>`, executionErr: context.Canceled, want: runnercontrol.JUnitSummary{Passed: 1, Failed: 1, Skipped: 1}},
		{name: "aggregate attributes cannot invent planned units", document: `<testsuite tests="18446744073709551616" failures="999"><testcase/></testsuite>`, want: runnercontrol.JUnitSummary{Passed: 1}},
		{name: "namespaced document retains local JUnit structure", document: `<j:testsuites xmlns:j="urn:junit"><j:testsuite><j:testcase/></j:testsuite></j:testsuites>`, want: runnercontrol.JUnitSummary{Passed: 1}},
		{name: "diagnostic properties contain no outcomes", document: `<testsuite><properties><property name="failure" value="diagnostic"/></properties><testcase/></testsuite>`, want: runnercontrol.JUnitSummary{Passed: 1}},
		{name: "case diagnostics do not change outcome", document: `<testsuite><testcase><system-out><![CDATA[<failure/>]]></system-out></testcase></testsuite>`, want: runnercontrol.JUnitSummary{Passed: 1}},
		{name: "unknown root cannot impersonate JUnit", document: `<foo><testcase/></foo>`, wantErr: true},
		{name: "bare testcase is not a JUnit document", document: `<testcase/>`, wantErr: true},
		{name: "testcase cannot be direct testsuites child", document: `<testsuites><testcase/></testsuites>`, wantErr: true},
		{name: "testcase after closed sibling suite cannot borrow its identity", document: `<testsuites><testsuite><testcase/></testsuite><testcase/></testsuites>`, wantErr: true},
		{name: "diagnostic child cannot smuggle testcase", document: `<testsuite><properties><testcase/></properties></testsuite>`, wantErr: true},
		{name: "arbitrary suite child is not a suite container", document: `<testsuite><foo><testcase/></foo></testsuite>`, wantErr: true},
		{name: "suite cannot appear inside testcase", document: `<testsuite><testcase><testsuite><testcase/></testsuite></testcase></testsuite>`, wantErr: true},
		{name: "testsuites cannot nest inside suite", document: `<testsuite><testsuites><testsuite><testcase/></testsuite></testsuites></testsuite>`, wantErr: true},
		{name: "failure marker cannot belong to suite", document: `<testsuite><failure/><testcase/></testsuite>`, wantErr: true},
		{name: "skipped marker cannot belong to suite", document: `<testsuite><skipped/><testcase/></testsuite>`, wantErr: true},
		{name: "nested diagnostic failure is not testcase outcome", document: `<testsuite><testcase><system-out><failure/></system-out></testcase></testsuite>`, wantErr: true},
		{name: "missing report preserves failed process cause", executionErr: context.Canceled, wantErr: true},
		{name: "malformed document discards closed partial counters", document: `<testsuite><testcase/><testcase>`, executionErr: context.Canceled, wantErr: true},
		{name: "all passed report contradicts failed process but retains observed counts", document: `<testsuite><testcase/></testsuite>`, executionErr: context.Canceled, want: runnercontrol.JUnitSummary{Passed: 1}, wantErr: true},
		{name: "failed report contradicts successful process but retains observed counts", document: `<testsuite><testcase><failure/></testcase></testsuite>`, want: runnercontrol.JUnitSummary{Failed: 1}, wantErr: true},
		{name: "empty suite cannot be a successful zero-test report", document: `<testsuite/>`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler := runnercontrol.NewJUnitSummaryCompiler()
			defer compiler.Abort()
			_, writeErr := compiler.Write([]byte(tc.document))
			got, sealErr := compiler.Seal(tc.executionErr)
			gotErr := errors.Join(writeErr, sealErr)
			if got != tc.want || (gotErr != nil) != tc.wantErr {
				t.Fatalf("summary = (%+v, %v), want (%+v, refusal %t)", got, gotErr, tc.want, tc.wantErr)
			}
			if tc.wantErr && (!errors.Is(gotErr, core.ErrPrimitiveContract) || tc.executionErr != nil && !errors.Is(gotErr, tc.executionErr)) {
				t.Fatalf("refusal = %v, want contract and native cause %v", gotErr, tc.executionErr)
			}
		})
	}
}

func TestJUnitSummaryCheckedCountLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		summary runnercontrol.JUnitSummary
		want    uint64
		wantErr bool
	}{
		{name: "zero counts remain neutral", summary: runnercontrol.JUnitSummary{}},
		{name: "mixed counts derive exact observed cases", summary: runnercontrol.JUnitSummary{Passed: 2, Failed: 3, Skipped: 5}, want: 10},
		{name: "one below uint64 extent is representable", summary: runnercontrol.JUnitSummary{Passed: math.MaxUint64 - 2, Failed: 1}, want: math.MaxUint64 - 1},
		{name: "exact uint64 extent is representable", summary: runnercontrol.JUnitSummary{Passed: math.MaxUint64 - 2, Failed: 1, Skipped: 1}, want: math.MaxUint64},
		{name: "passed plus failed cannot wrap", summary: runnercontrol.JUnitSummary{Passed: math.MaxUint64, Failed: 1}, wantErr: true},
		{name: "skipped cannot wrap partial sum", summary: runnercontrol.JUnitSummary{Passed: math.MaxUint64 - 1, Failed: 1, Skipped: 1}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := tc.summary.Cases()
			if got != tc.want || (gotErr != nil) != tc.wantErr || tc.wantErr && !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("Cases(%+v) = (%d,%v), want (%d,refusal %t)", tc.summary, got, gotErr, tc.want, tc.wantErr)
			}
			if validationErr := tc.summary.Validate(); (validationErr != nil) != tc.wantErr || tc.wantErr && !errors.Is(validationErr, core.ErrPrimitiveContract) {
				t.Fatalf("Validate(%+v) = %v, want refusal %t", tc.summary, validationErr, tc.wantErr)
			}
		})
	}
}

func TestJUnitSummaryHasNoPlannedUnitCeiling(t *testing.T) {
	t.Parallel()
	compiler := runnercontrol.NewJUnitSummaryCompiler()
	defer compiler.Abort()
	if _, err := io.WriteString(compiler, "<testsuite>"); err != nil {
		t.Fatal(err)
	}
	// This crosses the old planned-accounting policy ceiling. It is an evidence
	// extent, not a new production quota or an inferred plan.
	const cases = 65537
	for range cases {
		if _, err := io.WriteString(compiler, "<testcase/>"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := io.WriteString(compiler, "</testsuite>"); err != nil {
		t.Fatal(err)
	}
	got, err := compiler.Seal(nil)
	if err != nil || got != (runnercontrol.JUnitSummary{Passed: cases}) {
		t.Fatalf("summary beyond planned policy ceiling = (%+v,%v), want %d observed passes", got, err, cases)
	}
}

func TestJUnitSummaryCompilerOwnsTerminalLifetime(t *testing.T) {
	t.Parallel()
	var absent *runnercontrol.JUnitSummaryCompiler
	absent.Abort()
	if _, err := absent.Write(nil); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("nil Write error = %v, want contract", err)
	}
	if got, err := absent.Seal(context.Canceled); got != (runnercontrol.JUnitSummary{}) || !errors.Is(err, core.ErrPrimitiveContract) || !errors.Is(err, context.Canceled) {
		t.Fatalf("nil Seal = (%+v,%v), want zero and native contract refusal", got, err)
	}
	var zero runnercontrol.JUnitSummaryCompiler
	zero.Abort()
	if _, err := zero.Write(nil); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("zero Write error = %v, want contract", err)
	}
	compiler := runnercontrol.NewJUnitSummaryCompiler()
	if _, err := io.WriteString(compiler, "<testsuite><testcase"); err != nil {
		t.Fatal(err)
	}
	compiler.Abort()
	compiler.Abort()
	if _, err := compiler.Write(nil); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("aborted Write error = %v, want contract", err)
	}
	if got, err := compiler.Seal(nil); got != (runnercontrol.JUnitSummary{}) || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("aborted Seal = (%+v,%v), want zero and contract refusal", got, err)
	}
	sealed := runnercontrol.NewJUnitSummaryCompiler()
	defer sealed.Abort()
	if _, err := io.WriteString(sealed, "<testsuite><testcase/></testsuite>"); err != nil {
		t.Fatal(err)
	}
	if got, err := sealed.Seal(nil); err != nil || got != (runnercontrol.JUnitSummary{Passed: 1}) {
		t.Fatalf("first Seal = (%+v,%v), want one pass", got, err)
	}
	if got, err := sealed.Seal(nil); got != (runnercontrol.JUnitSummary{}) || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("second Seal = (%+v,%v), want zero and contract refusal", got, err)
	}
	if _, err := sealed.Write(nil); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("sealed Write error = %v, want contract", err)
	}
}

// DecodeElement provides a separate standard-library projection of each accepted
// testcase. Its typed marker fields ignore diagnostic text without retaining a
// report or testcase inventory.
type junitSummaryOracleCase struct {
	Failure  *struct{} `xml:"failure"`
	Error    *struct{} `xml:"error"`
	Skipped  *struct{} `xml:"skipped"`
	Disabled *struct{} `xml:"disabled"`
}

func FuzzJUnitSummaryAndPlannedAccounting(f *testing.F) {
	for _, document := range []junitFuzzDocument{
		{Cases: []junitFuzzCase{{Name: "pass"}}},
		{Cases: []junitFuzzCase{{Name: "skip", Skipped: &struct{}{}}, {Name: "pass"}}},
		{Cases: []junitFuzzCase{{Name: "one"}, {Name: "two"}, {Name: "three"}}},
	} {
		data, err := xml.Marshal(document)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte(`<foo><testcase/></foo>`))
	f.Add([]byte(`<testsuite><testcase><failure/></testcase></testsuite>`))
	f.Add([]byte(`<testsuite><testcase/></testsuite>trailing`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		summaryCompiler := runnercontrol.NewJUnitSummaryCompiler()
		defer summaryCompiler.Abort()
		_, summaryWriteErr := summaryCompiler.Write(data)
		got, summarySealErr := summaryCompiler.Seal(nil)
		summaryErr := errors.Join(summaryWriteErr, summarySealErr)
		planned, err := runnercontrol.NewJUnitObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationJUnitXML, ExpectedUnits: 2})
		if err != nil {
			t.Fatal(err)
		}
		defer planned.Abort()
		_, plannedWriteErr := planned.Write(data)
		accounting, plannedSealErr := planned.Seal(nil)
		plannedErr := errors.Join(plannedWriteErr, plannedSealErr)
		attempt, present := accounting.Accounting.Latest()
		cases, countErr := got.Cases()
		if countErr != nil || summaryErr == nil && cases == 0 {
			t.Fatalf("observed counts = %+v/%v, want representable nonempty admitted evidence", got, countErr)
		}
		if summaryErr != nil {
			if !errors.Is(summaryErr, core.ErrPrimitiveContract) || got != (runnercontrol.JUnitSummary{}) && got.Failed == 0 {
				t.Fatalf("summary refusal = (%+v,%v), want zero or observed failed-process contradiction", got, summaryErr)
			}
		}
		if plannedErr != nil {
			if !errors.Is(plannedErr, core.ErrPrimitiveContract) || !present || attempt.Unavailable != 2 || attempt.Passed != 0 || attempt.Failed != 0 || attempt.Skipped != 0 {
				t.Fatalf("planned refusal = (%+v,%v), want exact unavailable plan", accounting, plannedErr)
			}
		} else if summaryErr != nil || !present || cases > 2 || uint64(attempt.Passed) != got.Passed || uint64(attempt.Skipped) != got.Skipped || attempt.NotRun != 2-uint32(cases) || attempt.Failed != 0 {
			t.Fatalf("planned projection = (%+v,%v), observed (%+v,%v), want exact conserved projection", accounting, plannedErr, got, summaryErr)
		}
		if summaryErr != nil && got == (runnercontrol.JUnitSummary{}) {
			return
		}
		if summaryErr == nil && cases <= 2 && plannedErr != nil {
			t.Fatalf("known plan refused admitted observed summary %+v: %v", got, plannedErr)
		}
		var want runnercontrol.JUnitSummary
		decoder := xml.NewDecoder(bytes.NewReader(data))
		rootSeen := false
		for {
			token, decodeErr := decoder.Token()
			if errors.Is(decodeErr, io.EOF) {
				break
			}
			if decodeErr != nil {
				t.Fatalf("accepted XML native decode error = %v, want nil", decodeErr)
			}
			start, ok := token.(xml.StartElement)
			if !ok {
				continue
			}
			if !rootSeen {
				if start.Name.Local != "testsuite" && start.Name.Local != "testsuites" {
					t.Fatalf("accepted XML root = %q, want native JUnit suite container", start.Name.Local)
				}
				rootSeen = true
			}
			if start.Name.Local != "testcase" {
				continue
			}
			var testcase junitSummaryOracleCase
			if err := decoder.DecodeElement(&testcase, &start); err != nil {
				t.Fatal(err)
			}
			switch {
			case testcase.Failure != nil || testcase.Error != nil:
				want.Failed++
			case testcase.Skipped != nil || testcase.Disabled != nil:
				want.Skipped++
			default:
				want.Passed++
			}
		}
		if got != want {
			t.Fatalf("summary = %+v, want independently decoded %+v", got, want)
		}
	})
}
