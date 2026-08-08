package controlplane_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// TestEveryDocumentCeilingRefusesAnOversizedInput drives each declared byte
// ceiling from the hostile side. The twelve constants existed as enforced
// bounds with no test on either side; this pins the refusing side for every
// document family with a syntactically valid JSON object one byte past its
// ceiling, so a ceiling silently dropped from a decoder goes red. The
// accepting side cannot be pinned at the exact boundary, because a canonical
// document's size is determined by its facts and no valid document lands on
// the ceiling byte for byte; the committed goldens prove acceptance well
// inside every bound.
func TestEveryDocumentCeilingRefusesAnOversizedInput(t *testing.T) {
	t.Parallel()

	oversized := func(ceiling int) []byte {
		return []byte(`{"` + strings.Repeat("a", ceiling) + `":1}`)
	}
	cases := []struct {
		decode  func([]byte) error
		name    string
		ceiling int
	}{
		{name: "check-in payload", ceiling: controlplane.CheckInPayloadJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.CheckInPayload; return v.UnmarshalJSON(b) }},
		{name: "check-in request", ceiling: controlplane.CheckInRequestJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.CheckInRequest; return v.UnmarshalJSON(b) }},
		{name: "check-in response payload", ceiling: controlplane.CheckInResponsePayloadJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.CheckInResponsePayload; return v.UnmarshalJSON(b) }},
		{name: "check-in response document", ceiling: controlplane.CheckInResponseDocumentJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.CheckInResponseDocument; return v.UnmarshalJSON(b) }},
		{name: "response header", ceiling: controlplane.ResponseHeaderJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.ResponseHeader; return v.UnmarshalJSON(b) }},
		{name: "registration request", ceiling: controlplane.RegistrationRequestJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.RegistrationRequest; return v.UnmarshalJSON(b) }},
		{name: "installation certificate body", ceiling: controlplane.InstallationCertificateBodyJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.InstallationCertificateBody; return v.UnmarshalJSON(b) }},
		{name: "installation certificate document", ceiling: controlplane.InstallationCertificateDocumentJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.InstallationCertificateDocument; return v.UnmarshalJSON(b) }},
		{name: "registration payload", ceiling: controlplane.RegistrationPayloadJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.RegistrationPayload; return v.UnmarshalJSON(b) }},
		{name: "registration document", ceiling: controlplane.RegistrationDocumentJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.RegistrationDocument; return v.UnmarshalJSON(b) }},
		{name: "usage watermark", ceiling: controlplane.UsageWatermarkJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.UsageWatermark; return v.UnmarshalJSON(b) }},
		{name: "usage window", ceiling: controlplane.UsageWindowJSONMaximumBytes,
			decode: func(b []byte) error { var v controlplane.UsageWindow; return v.UnmarshalJSON(b) }},
	}

	for _, tc := range cases {
		t.Run(tc.name+" refuses one byte past its ceiling", func(t *testing.T) {
			t.Parallel()

			err := tc.decode(oversized(tc.ceiling))
			if !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("UnmarshalJSON(%d bytes over the %d ceiling) error = %v, want errors.Is %v",
					len(oversized(tc.ceiling))-tc.ceiling, tc.ceiling, err, core.ErrControlPlaneContract)
			}
		})
	}
}
