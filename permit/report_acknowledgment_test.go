package permit

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestReportAcknowledgmentRevisionLayerTriad(t *testing.T) {
	t.Parallel()
	request, key := permitFixture(t)
	scope := reportScopeFixture(t)
	for _, tc := range []struct {
		wantErr  error
		name     string
		revision uint64
	}{
		{name: "empty first report preserves zero master revision", revision: 0},
		{name: "first contribution records revision one", revision: 1},
		{name: "largest stored revision is representable", revision: math.MaxInt64},
		{name: "one above stored ceiling refuses acknowledgment", revision: math.MaxInt64 + 1, wantErr: core.ErrReportContract},
		{name: "unsigned extreme refuses acknowledgment", revision: math.MaxUint64, wantErr: core.ErrReportContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := ReportAcknowledgment{Scope: scope, Sequence: 1, ReportDigest: core.SHA256Of([]byte("empty or populated report identity")), ProjectRevision: tc.revision, AcceptedAt: temporal.InstantFromNanoseconds(0), Schedule: reportSchedule(t)}
			got, err := SignReportAcknowledgment(payload, key)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("sign revision %d = %v, want %v", tc.revision, err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (SignedReportAcknowledgment{}) {
					t.Fatalf("refused acknowledgment = %+v, want zero", got)
				}
				return
			}
			if got.Payload != payload {
				t.Fatalf("signed payload = %+v, want %+v", got.Payload, payload)
			}
			if err := got.Verify(request.TrustedKeys, scope); err != nil {
				t.Fatalf("verify = %v, want nil", err)
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("marshal = %v, want nil", err)
			}
			var decoded SignedReportAcknowledgment
			if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
				t.Fatalf("round trip = %+v/%v, want %+v/nil", decoded, err, got)
			}
		})
	}
}
