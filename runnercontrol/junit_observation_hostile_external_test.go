package runnercontrol_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

type junitBoundaryCase struct {
	name      string
	class     string
	document  string
	expected  uint32
	filtered  bool
	exitError bool
	chunk     int
	want      projectstandards.ExecutionAttempt
	wantErr   error
}

func TestJUnitObservationCompilerHostileEvidenceMatrix(t *testing.T) {
	t.Parallel()

	cases := junitHostileCases()
	classes := map[string]int{}
	for index := range cases {
		classes[cases[index].class]++
	}
	wantClasses := map[string]int{"valid": 10, "rejection": 10, "boundary": 20}
	if len(cases) != 40 || classes["valid"] != 10 || classes["rejection"] != 10 || classes["boundary"] != 20 {
		t.Fatalf("JUnit hostile matrix = %v across %d cases, want %v across 40 earned cases", classes, len(cases), wantClasses)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationJUnitXML, ExpectedUnits: tc.expected, Filtered: tc.filtered}
			compiler, err := runnercontrol.NewJUnitObservationCompiler(policy)
			if err != nil {
				t.Fatalf("NewJUnitObservationCompiler(%s) error = %v, want nil", tc.name, err)
			}
			writeJUnitChunks(t, compiler, tc.document, tc.chunk)
			var executionErr error
			if tc.exitError {
				executionErr = errors.New("observed Bun process exited unsuccessfully")
			}
			got, gotErr := compiler.Seal(executionErr)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("JUnitObservationCompiler.Seal(%s) error = %v, want errors.Is(..., %v)", tc.name, gotErr, tc.wantErr)
				}
				return
			}
			latest, present := got.Accounting.Latest()
			if gotErr != nil || !present || latest != tc.want {
				t.Fatalf("JUnitObservationCompiler.Seal(%s) = (attempt %+v, present %t, error %v), want (%+v, true, nil)", tc.name, latest, present, gotErr, tc.want)
			}
		})
	}
}

func junitHostileCases() []junitBoundaryCase {
	pass := `<testsuite><testcase name="one"/></testsuite>`
	fail := `<testsuite><testcase name="one"><failure>assertion</failure></testcase></testsuite>`
	skipped := `<testsuite><testcase name="one"><skipped/></testcase></testsuite>`
	errorCase := `<testsuite><testcase name="one"><error>setup</error></testcase></testsuite>`
	disabled := `<testsuite><testcase name="one"><disabled/></testcase></testsuite>`
	return []junitBoundaryCase{
		junitPass("valid one passing testcase closes one planned unit", "valid", pass, 1),
		junitFail("valid assertion failure remains failed evidence", "valid", fail, 1),
		junitFail("valid setup error remains failed evidence", "valid", errorCase, 1),
		junitSkip("valid skipped testcase remains in the denominator", "valid", skipped, 1),
		junitSkip("valid disabled testcase remains in the denominator", "valid", disabled, 1),
		junitPassCount("valid two passing cases close two planned units", "valid", `<testsuite><testcase name="one"/><testcase name="two"/></testsuite>`, 2, 2),
		{name: "valid mixed outcomes preserve every distinct counter", class: "valid", document: `<testsuite><testcase name="pass"/><testcase name="fail"><failure/></testcase><testcase name="skip"><skipped/></testcase></testsuite>`, expected: 3, exitError: true, want: junitAttempt(3, 1, 1, 1, 0, false)},
		junitPass("valid nested suite container does not duplicate testcase accounting", "valid", `<testsuites><testsuite name="outer"><testsuite name="inner"><testcase name="one"/></testsuite></testsuite></testsuites>`, 1),
		junitPass("valid attributes remain diagnostic rather than accounting authority", "valid", `<testsuite tests="999" failures="999"><testcase classname="pkg" name="one" time="0.1"/></testsuite>`, 1),
		junitPass("valid system output does not manufacture a testcase", "valid", `<testsuite><system-out>diagnostic</system-out><testcase name="one"/></testsuite>`, 1),

		junitReject("rejection empty report has no evidence", "rejection", "", 1),
		junitReject("rejection suite without testcase has no evidence", "rejection", `<testsuite/>`, 1),
		junitReject("rejection truncated testcase cannot be counted", "rejection", `<testsuite><testcase>`, 1),
		junitReject("rejection nested testcase makes ownership ambiguous", "rejection", `<testsuite><testcase><testcase/></testcase></testsuite>`, 1),
		junitReject("rejection report cannot exceed the planned denominator", "rejection", `<testsuite><testcase/><testcase/></testsuite>`, 1),
		junitRejectExit("rejection successful process cannot report a failed testcase", "rejection", fail, 1, false),
		junitRejectExit("rejection failed process cannot report every testcase passed", "rejection", pass, 1, true),
		junitReject("rejection duplicate XML attributes are malformed", "rejection", `<testsuite><testcase name="one" name="two"/></testsuite>`, 1),
		junitReject("rejection unresolved entity is malformed external input", "rejection", `<testsuite><testcase>&unknown;</testcase></testsuite>`, 1),
		junitReject("rejection mismatched closing element is malformed", "rejection", `<testsuite><testcase></testsuite>`, 1),

		junitPass("boundary self-closing testcase is an exact passing unit", "boundary", `<testcase name="one"/>`, 1),
		{name: "boundary one observed below two planned remains explicit not-run evidence", class: "boundary", document: pass, expected: 2, want: junitAttempt(2, 1, 0, 0, 1, false)},
		junitPassCount("boundary exact planned count is admitted", "boundary", `<testsuite><testcase/><testcase/></testsuite>`, 2, 2),
		junitReject("boundary one case above the denominator is refused", "boundary", `<testsuite><testcase/><testcase/><testcase/></testsuite>`, 2),
		junitPass("boundary XML declaration is accepted without changing evidence", "boundary", `<?xml version="1.0"?><testsuite><testcase/></testsuite>`, 1),
		junitPass("boundary namespace prefixes retain testcase local identity", "boundary", `<j:testsuite xmlns:j="urn:junit"><j:testcase/></j:testsuite>`, 1),
		junitPass("boundary CDATA diagnostic text does not alter a passing case", "boundary", `<testsuite><testcase><![CDATA[<not-markup>]]></testcase></testsuite>`, 1),
		junitPass("boundary processing instruction does not alter accounting", "boundary", `<testsuite><?runner exact?><testcase/></testsuite>`, 1),
		junitPass("boundary comment does not alter accounting", "boundary", `<testsuite><!-- retained diagnostic --><testcase/></testsuite>`, 1),
		{name: "boundary filtered policy remains visible in accounting", class: "boundary", document: pass, expected: 1, filtered: true, want: junitAttempt(1, 1, 0, 0, 0, true)},
		junitPassChunked("boundary one-byte chunks preserve streaming parse", "boundary", pass, 1, 1),
		junitPassChunked("boundary token-boundary chunks preserve streaming parse", "boundary", pass, 1, 7),
		{name: "boundary failure takes precedence over a sibling skipped marker", class: "boundary", document: `<testsuite><testcase><skipped/><failure/></testcase></testsuite>`, expected: 1, exitError: true, want: junitAttempt(1, 0, 1, 0, 0, false)},
		{name: "boundary error takes precedence over a sibling disabled marker", class: "boundary", document: `<testsuite><testcase><disabled/><error/></testcase></testsuite>`, expected: 1, exitError: true, want: junitAttempt(1, 0, 1, 0, 0, false)},
		{name: "boundary two skipped cases retain exact planned count", class: "boundary", document: `<testsuite><testcase><skipped/></testcase><testcase><skipped/></testcase></testsuite>`, expected: 2, want: junitAttempt(2, 0, 0, 2, 0, false)},
		junitPass("boundary Unicode diagnostic attributes remain transportable", "boundary", `<testsuite><testcase name="測試-✓"/></testsuite>`, 1),
		junitPass("boundary empty diagnostic elements remain neutral", "boundary", `<testsuite><properties/><system-out/><system-err/><testcase/></testsuite>`, 1),
		junitPass("boundary failure-like prose outside testcase cannot create failure", "boundary", `<testsuite><system-out>&lt;failure/&gt;</system-out><testcase/></testsuite>`, 1),
		junitPass("boundary maximum admitted nesting depth retains the testcase", "boundary", nestedJUnit(runnercontrol.JUnitXMLDepthMaximum, pass), 1),
		junitReject("boundary one nesting level above ceiling is refused", "boundary", nestedJUnit(runnercontrol.JUnitXMLDepthMaximum+1, pass), 1),
	}
}

func junitPass(name, class, document string, planned uint32) junitBoundaryCase {
	return junitBoundaryCase{name: name, class: class, document: document, expected: planned, want: junitAttempt(planned, 1, 0, 0, planned-1, false)}
}

func junitPassCount(name, class, document string, planned, passed uint32) junitBoundaryCase {
	return junitBoundaryCase{name: name, class: class, document: document, expected: planned, want: junitAttempt(planned, passed, 0, 0, planned-passed, false)}
}

func junitPassChunked(name, class, document string, planned uint32, chunk int) junitBoundaryCase {
	result := junitPass(name, class, document, planned)
	result.chunk = chunk
	return result
}

func junitFail(name, class, document string, planned uint32) junitBoundaryCase {
	return junitBoundaryCase{name: name, class: class, document: document, expected: planned, exitError: true, want: junitAttempt(planned, 0, 1, 0, planned-1, false)}
}

func junitSkip(name, class, document string, planned uint32) junitBoundaryCase {
	return junitBoundaryCase{name: name, class: class, document: document, expected: planned, want: junitAttempt(planned, 0, 0, 1, planned-1, false)}
}

func junitReject(name, class, document string, planned uint32) junitBoundaryCase {
	return junitBoundaryCase{name: name, class: class, document: document, expected: planned, wantErr: core.ErrPrimitiveContract}
}

func junitRejectExit(name, class, document string, planned uint32, exitError bool) junitBoundaryCase {
	result := junitReject(name, class, document, planned)
	result.exitError = exitError
	return result
}

func junitAttempt(planned, passed, failed, skipped, notRun uint32, filtered bool) projectstandards.ExecutionAttempt {
	return projectstandards.ExecutionAttempt{Sequence: 1, Planned: planned, Passed: passed, Failed: failed, Skipped: skipped, NotRun: notRun, Cache: projectstandards.CacheDisabled, Filtered: filtered}
}

func nestedJUnit(depth int, inner string) string {
	return strings.Repeat("<suite>", depth-2) + inner + strings.Repeat("</suite>", depth-2)
}

func writeJUnitChunks(t testing.TB, compiler *runnercontrol.JUnitObservationCompiler, document string, size int) {
	t.Helper()
	if size <= 0 {
		size = len(document)
		if size == 0 {
			size = 1
		}
	}
	for start := 0; start < len(document); start += size {
		end := min(start+size, len(document))
		if _, err := compiler.Write([]byte(document[start:end])); err != nil {
			t.Fatalf("JUnitObservationCompiler.Write(%d:%d) error = %v, want nil until Seal classifies the stream", start, end, err)
		}
	}
}
