package cloudbilling

import (
	"testing"

	googlebilling "google.golang.org/api/cloudbilling/v1"
)

func TestProviderEmptySKUsWireRepresentations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		producer googlebilling.ListSkusResponse
	}{
		{name: "omitted repeated field", producer: googlebilling.ListSkusResponse{}},
		{name: "explicit empty repeated field", producer: googlebilling.ListSkusResponse{ForceSendFields: []string{"Skus"}}},
		{name: "explicit null repeated field", producer: googlebilling.ListSkusResponse{NullFields: []string{"Skus"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Google's actual generated producer owns these distinct valid wire
			// representations; SDK decoding does not assert pricing eligibility.
			data, err := tc.producer.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodePage(data, fixtureLimits(t))
			if err != nil || len(got.SKUs) != 0 || got.Next != "" {
				t.Fatalf("DecodePage(provider empty %q)=(%+v,%v),want neutral provider facts", data, got, err)
			}
		})
	}
}
