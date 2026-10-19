package core

import (
	"errors"
	"testing"
)

// TestStrictJSONScanSharedNameArenaKeepsObjectsIndependent proves that names
// held in place and names folded into the shared arena stay scoped to their
// own object: sibling and nested objects may reuse names, an outer duplicate
// is still caught after an inner object closes, and a pooled scan state never
// carries one document's names into the next.
func TestStrictJSONScanSharedNameArenaKeepsObjectsIndependent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		wire    string
		wantErr bool
	}{
		{name: "positive/sibling objects reuse in-place names", wire: `{"x":{"k":1,"j":2},"y":{"k":1,"j":2}}`},
		{name: "positive/sibling objects reuse folded names", wire: `{"x":{"K":1,"\u00c9":2},"y":{"K":1,"\u00c9":2}}`},
		{name: "positive/nested object reuses outer name", wire: `{"a":{"a":{"A":1}}}`},
		{name: "positive/arrays of objects reuse names", wire: `[{"Name":1},{"Name":2},{"name":3}]`},
		{name: "negative/outer folded duplicate after inner arena names", wire: `{"a":{"\u00c9":1,"Z":2},"A":3}`, wantErr: true},
		{name: "negative/outer in-place duplicate after inner arena names", wire: `{"q":{"\u00c9":1},"r":2,"\u0071":3}`, wantErr: true},
		{name: "negative/escaped spelling of in-place name", wire: `{"key":1,"k\u0065y":2}`, wantErr: true},
		{name: "negative/escaped folded spelling of in-place name", wire: `{"key":1,"K\u0065Y":2}`, wantErr: true},
		{name: "negative/in-place name against folded name", wire: `{"name":1,"NAME":2}`, wantErr: true},
		{name: "negative/inner duplicate between in-place and folded", wire: `{"o":{"k":1,"\u212a":2}}`, wantErr: true},
		{name: "neutral/empty objects at every level", wire: `{"a":{},"b":[{}],"c":{"d":{}}}`},
	}
	limits := DefaultStrictJSONLimits()
	// Two passes run every case on a warm pooled scan state as well.
	for range 2 {
		for _, tc := range cases {
			err := limits.ValidateDocument([]byte(tc.wire))
			if tc.wantErr != (err != nil) || err != nil && !errors.Is(err, ErrJSONContract) {
				t.Fatalf("%s: ValidateDocument(%s) error = %v, want error %t with %v", tc.name, tc.wire, err, tc.wantErr, ErrJSONContract)
			}
		}
	}
}
