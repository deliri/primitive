package plunk

import (
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

// Absence (null) and any parsed instant validate; both survive the JSON door.
func TestSendTimestampValidateThroughJSON(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, json string
		zero       bool
	}{
		{"null is absence", `null`, true},
		{"a whitespace-wrapped null", ` null `, true},
		{"an RFC 3339 instant", `"2026-11-02T15:04:05Z"`, false},
		{"a nanosecond instant", `"2026-11-02T15:04:05.123456789Z"`, false},
	} {
		var stamp SendTimestamp
		if err := stamp.UnmarshalJSON([]byte(tc.json)); err != nil {
			t.Fatalf("%s: decode: %v", tc.name, err)
		}
		if stamp.IsZero() != tc.zero || stamp.Validate() != nil {
			t.Fatalf("%s: zero=%v validate=%v", tc.name, stamp.IsZero(), stamp.Validate())
		}
	}
	made, err := NewSendTimestamp(temporal.InstantFromNanoseconds(1))
	if err != nil || made.Validate() != nil {
		t.Fatalf("a constructed timestamp did not validate: %v", err)
	}
	if (SendTimestamp{}).Validate() != nil {
		t.Fatal("the zero value is documented absence and must validate")
	}
}
