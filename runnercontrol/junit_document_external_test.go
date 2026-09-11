package runnercontrol_test

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestJUnitDocumentFramingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, document string
		wantErr        error
	}{
		{name: "one document contributes one testcase", document: "<testcase/>"},
		{name: "surrounding XML whitespace contributes no evidence", document: " \t\r\n<testcase/> \t\r\n"},
		{name: "two roots cannot merge evidence", document: "<testcase/><empty/>", wantErr: core.ErrPrimitiveContract},
		{name: "leading text cannot be discarded", document: "noise<testcase/>", wantErr: core.ErrPrimitiveContract},
		{name: "trailing text cannot be discarded", document: "<testcase/>noise", wantErr: core.ErrPrimitiveContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compiler, err := runnercontrol.NewJUnitObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationJUnitXML, ExpectedUnits: 1})
			if err != nil {
				t.Fatal(err)
			}
			defer compiler.Abort()
			_, writeErr := compiler.Write([]byte(tc.document))
			got, sealErr := compiler.Seal(nil)
			err = errors.Join(writeErr, sealErr)
			attempt, ok := got.Accounting.Latest()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || !ok || attempt.Unavailable != 1 || attempt.Passed != 0 {
					t.Fatalf("framing = %+v/%v, want unavailable/%v", got, err, tc.wantErr)
				}
			} else if err != nil || !ok || attempt.Passed != 1 {
				t.Fatalf("framing = %+v/%v, want one passed/nil", got, err)
			}
		})
	}
}

// The standard XML decoder owns its current materialized token and nesting
// stack. Total stream extent is not an admission limit or a retained document.
func TestJUnitStreamingHasNoTotalExtentQuota(t *testing.T) {
	t.Parallel()
	compiler, err := runnercontrol.NewJUnitObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationJUnitXML, ExpectedUnits: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Abort()
	if _, err = compiler.Write([]byte("<testsuite>")); err != nil {
		t.Fatal(err)
	}
	chunk := []byte("<system-out>" + strings.Repeat("x", 4096) + "</system-out>")
	for range 4096 {
		if _, err = compiler.Write(chunk); err != nil {
			t.Fatalf("diagnostic write = %v, want nil", err)
		}
	}
	if _, err = compiler.Write([]byte("<testcase/></testsuite>")); err != nil {
		t.Fatal(err)
	}
	got, err := compiler.Seal(nil)
	attempt, ok := got.Accounting.Latest()
	if err != nil || !ok || attempt.Passed != 1 {
		t.Fatalf("stream = %+v/%v, want one passed/nil", got, err)
	}
}

type junitFuzzDocument struct {
	XMLName xml.Name        `xml:"testsuite"`
	Cases   []junitFuzzCase `xml:"testcase"`
}
type junitFuzzCase struct {
	Name    string    `xml:"name,attr"`
	Skipped *struct{} `xml:"skipped,omitempty"`
}

func FuzzJUnitObservationSemanticAccounting(f *testing.F) {
	for _, document := range []junitFuzzDocument{
		{Cases: []junitFuzzCase{{Name: "one"}}},
		{Cases: []junitFuzzCase{{Name: "one", Skipped: &struct{}{}}, {Name: "two"}}},
	} {
		data, err := xml.Marshal(document)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte("<testcase/><testcase/>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		compiler, err := runnercontrol.NewJUnitObservationCompiler(runnercontrol.ObservationPolicy{Format: runnercontrol.ObservationJUnitXML, ExpectedUnits: 2})
		if err != nil {
			t.Fatal(err)
		}
		defer compiler.Abort()
		_, writeErr := compiler.Write(data)
		got, sealErr := compiler.Seal(nil)
		err = errors.Join(writeErr, sealErr)
		attempt, ok := got.Accounting.Latest()
		if err != nil {
			if !errors.Is(err, core.ErrPrimitiveContract) || !ok || attempt.Unavailable != 2 || attempt.Passed != 0 || attempt.Failed != 0 || attempt.Skipped != 0 {
				t.Fatalf("rejection = %+v/%v, want typed unavailable", got, err)
			}
			return
		}
		if validationErr := got.Validate(); validationErr != nil || !ok || attempt.Planned != 2 || attempt.Passed+attempt.Skipped+attempt.NotRun != 2 || attempt.Failed != 0 {
			t.Fatalf("accepted accounting = %+v/%v, want conserved successful process facts", got, validationErr)
		}
		var document junitFuzzDocument
		// This independent oracle applies to the ordinary direct-child suite shape;
		// nested suites and namespace-local JUnit extensions have separate tables.
		if decodeErr := xml.Unmarshal(data, &document); decodeErr == nil && len(document.Cases) > 0 {
			var skipped uint32
			for _, testcase := range document.Cases {
				if testcase.Skipped != nil {
					skipped++
				}
			}
			if attempt.Skipped < skipped || uint64(attempt.Passed+attempt.Skipped) < uint64(len(document.Cases)) {
				t.Fatalf("projection = %+v, want all independently decoded cases retained", attempt)
			}
		}
	})
}
