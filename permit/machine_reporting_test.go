package permit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestMachineReportingPermissionLayerTriad(t *testing.T) {
	t.Parallel()
	request, key := permitFixture(t)
	terms := request.Document.Terms
	terms.Reporting = reportSchedule(t)
	document, err := Sign(terms, key)
	if err != nil {
		t.Fatalf("Sign(machine window) error = %v, want nil", err)
	}
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(machine window) error = %v, want nil", err)
	}
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode(machine window) error = %v, want nil", err)
	}
	request.Document = decoded
	verified, err := Verify(request)
	if err != nil {
		t.Fatalf("Verify(machine window) error = %v, want nil", err)
	}
	got, err := verified.Terms()
	if err != nil || got != terms {
		t.Fatalf("verified terms = %+v/%v, want %+v/nil", got, err, terms)
	}
	request.Document.Terms.Reporting.NextReportAt = temporal.InstantFromNanoseconds(101)
	if request.Document.Terms.Reporting == terms.Reporting {
		t.Fatal("mutated window = signed window, want changed opening")
	}
	if _, err := Verify(request); !errors.Is(err, core.ErrPermitAuthentication) {
		t.Fatalf("Verify(substituted opening) error = %v, want %v", err, core.ErrPermitAuthentication)
	}
	terms.Reporting = ReportSchedule{}
	document, err = Sign(terms, key)
	if err != nil {
		t.Fatalf("Sign(no reporting grant) error = %v, want nil", err)
	}
	request.Document = document
	verified, err = Verify(request)
	if err != nil {
		t.Fatalf("Verify(no reporting grant) error = %v, want nil", err)
	}
	got, err = verified.Terms()
	if err != nil || got.Reporting != (ReportSchedule{}) {
		t.Fatalf("absent reporting grant = %+v/%v, want zero/nil", got.Reporting, err)
	}
	terms.Reporting.NextReportAt = temporal.InstantFromNanoseconds(0)
	if err := terms.Validate(); !errors.Is(err, core.ErrReportSchedule) {
		t.Fatalf("Validate(partial reporting grant) error = %v, want %v", err, core.ErrReportSchedule)
	}
}
