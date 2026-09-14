package permit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

// Every mutation preserves structural validity and changes exactly one signed
// field. Authentication, rather than a parser failure, must reject it.
func mutatePermitTerm(t testing.TB, terms Terms, selector uint8) Terms {
	t.Helper()
	got := terms
	var err error
	switch selector % 15 {
	case 0:
		got.Actions = Actions{}
	case 1:
		got.NotBefore = temporal.InstantFromNanoseconds(99)
	case 2:
		got.ExpiresAt = temporal.InstantFromNanoseconds(201)
	case 3:
		got.ContactAt = temporal.InstantFromNanoseconds(151)
	case 4:
		got.RetryAfter, err = temporal.DurationFromMilliseconds(1001)
	case 5:
		got.Generation, err = lease.NewGeneration(2)
	case 6:
		got.Subject.DeviceID, err = lease.NewDeviceID([16]byte{99})
	case 7:
		got.Subject.EntitlementID, err = lease.NewEntitlementID([16]byte{98})
	case 8:
		got.RequestNonce, err = controlwire.NewRequestNonce([32]byte{32})
	case 9:
		got.Reporting.NextReportAt = temporal.InstantFromNanoseconds(101)
	case 10:
		got.Reporting.WindowDuration = reportDuration(t, 11)
	case 11:
		got.Reporting.RepeatInterval = reportDuration(t, 101)
	case 12:
		got.Reporting.JitterMaximum = reportDuration(t, 2)
	case 13:
		got.Reporting.Policy.Activation++
	case 14:
		got.Reporting = ReportSchedule{}
	default:
		t.Fatalf("mutation selector = %d, want within exhaustive domain", selector)
	}
	if err != nil || got == terms || got.Validate() != nil {
		t.Fatalf("mutation changed/valid/error = %v/%v/%v, want true/nil/nil", got != terms, got.Validate(), err)
	}
	return got
}

func TestPermitEverySignedTermMutationRefuses(t *testing.T) {
	t.Parallel()
	names := []string{"skill set", "activation", "expiry", "contact", "retry", "generation", "installation", "entitlement", "response identity", "report opening", "report width", "report recurrence", "report jitter", "report policy", "report grant removed"}
	for selector, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request, key := permitFixture(t)
			terms := request.Document.Terms
			terms.Reporting = reportSchedule(t)
			var signErr error
			request.Document, signErr = Sign(terms, key)
			if signErr != nil {
				t.Fatalf("Sign(reporting grant) error = %v, want nil", signErr)
			}
			baseline, err := Verify(request)
			if err != nil || baseline.Allows(permitAction(t, "operation-a"), request.EffectiveAt) != nil {
				t.Fatalf("baseline = %v, want authenticated start", err)
			}
			request.Document.Terms = mutatePermitTerm(t, request.Document.Terms, uint8(selector))
			var encoded bytes.Buffer
			if err := request.Document.Write(&encoded); err != nil {
				t.Fatalf("Write mutation = %v, want nil", err)
			}
			decoded, err := Decode(&encoded)
			if err != nil || decoded != request.Document {
				t.Fatalf("Decode mutation = %v, want exact structurally valid document", err)
			}
			request.Document = decoded
			got, err := Verify(request)
			if !errors.Is(err, core.ErrPermitAuthentication) || got != (Verified{}) {
				t.Fatalf("Verify mutation = %v/%v, want zero/core.ErrPermitAuthentication", got, err)
			}
		})
	}
}

func TestPermitGenerationRollbackBoundary(t *testing.T) {
	t.Parallel()
	request, _ := permitFixture(t)
	baseline, err := Verify(request)
	if err != nil || baseline.Allows(permitAction(t, "operation-a"), request.EffectiveAt) != nil {
		t.Fatalf("generation at minimum = %v, want nil", err)
	}
	request.MinimumGeneration, err = lease.NewGeneration(2)
	if err != nil {
		t.Fatalf("NewGeneration() = %v, want nil", err)
	}
	got, err := Verify(request)
	if !errors.Is(err, core.ErrPermitReplay) || got != (Verified{}) {
		t.Fatalf("older authentic permission = %v/%v, want zero/core.ErrPermitReplay", got, err)
	}
}

func TestPermitDecoderByteBoundary(t *testing.T) {
	t.Parallel()
	for _, size := range []int{DocumentMaximumBytes - 1, DocumentMaximumBytes, DocumentMaximumBytes + 1} {
		t.Run(fmt.Sprintf("document extent %d", size), func(t *testing.T) {
			t.Parallel()
			request, _ := permitFixture(t)
			var encoded bytes.Buffer
			if err := request.Document.Write(&encoded); err != nil {
				t.Fatalf("Write() = %v, want nil", err)
			}
			if encoded.Len() >= size {
				t.Fatalf("seed size = %d, want below %d", encoded.Len(), size)
			}
			data := append(encoded.Bytes(), bytes.Repeat([]byte{' '}, size-encoded.Len())...)
			got, err := Decode(bytes.NewReader(data))
			if size <= DocumentMaximumBytes {
				if err != nil || got != request.Document {
					t.Fatalf("Decode(%d) = %v, want exact document/nil", size, err)
				}
			} else if !errors.Is(err, core.ErrPermitContract) || got != (Document{}) {
				t.Fatalf("Decode oversized = %v/%v, want zero/core.ErrPermitContract", got, err)
			}
		})
	}
}

func TestPermitWriterFailurePreservesCause(t *testing.T) {
	t.Parallel()
	request, _ := permitFixture(t)
	err := request.Document.Write(failedPermitWriter{})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write closed sink = %v, want ErrClosedPipe", err)
	}
}

type failedPermitWriter struct{}

func (failedPermitWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
