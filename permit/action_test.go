package permit

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

func TestActionRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr     error
		name, input string
	}{
		{name: "minimum single letter", input: "a"},
		{name: "minimum single digit", input: "0"},
		{name: "interior separators preserved", input: "a-b_c.d"},
		{name: "one below byte ceiling", input: strings.Repeat("a", ActionMaximumBytes-1)},
		{name: "exact byte ceiling", input: strings.Repeat("a", ActionMaximumBytes)},
		{name: "one above byte ceiling", input: strings.Repeat("a", ActionMaximumBytes+1), wantErr: core.ErrPermitContract},
		{name: "empty identifier refused", wantErr: core.ErrPermitContract},
		{name: "leading separator refused", input: "-a", wantErr: core.ErrPermitContract},
		{name: "trailing separator refused", input: "a-", wantErr: core.ErrPermitContract},
		{name: "case is not silently folded", input: "A", wantErr: core.ErrPermitContract},
		{name: "unicode is not silently normalized", input: "é", wantErr: core.ErrPermitContract},
		{name: "embedded zero cannot truncate identity", input: "a\x00b", wantErr: core.ErrPermitContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := ParseAction(tc.input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseAction(%q) error = %v, want %v", tc.input, gotErr, tc.wantErr)
			}
			if gotErr != nil {
				if got != (Action{}) {
					t.Fatalf("refused action = %v, want zero", got)
				}
				return
			}
			if got.String() != tc.input {
				t.Fatalf("action = %q, want %q", got.String(), tc.input)
			}
		})
	}
}

func TestActionsLayerTriad(t *testing.T) {
	t.Parallel()
	a, b := permitAction(t, "operation-a"), permitAction(t, "operation-b")
	for _, tc := range []struct {
		wantErr      error
		name         string
		input        []Action
		wantCount    int
		wantA, wantB bool
	}{
		{name: "two distinct identities canonically reordered", input: []Action{b, a}, wantCount: 2, wantA: true, wantB: true},
		{name: "duplicate identity rejected", input: []Action{a, a}, wantErr: core.ErrPermitContract},
		{name: "empty set grants neither identity"},
		{name: "one identity never grants its sibling", input: []Action{a}, wantCount: 1, wantA: true},
		{name: "zero identity refused", input: []Action{{}}, wantErr: core.ErrPermitContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := NewActions(tc.input...)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewActions error = %v, want %v", gotErr, tc.wantErr)
			}
			if gotErr != nil {
				if got != (Actions{}) {
					t.Fatalf("refused set = %v, want zero", got)
				}
				return
			}
			count, err := got.Count()
			if err != nil || count != tc.wantCount {
				t.Fatalf("Count = %d/%v, want %d/nil", count, err, tc.wantCount)
			}
			for _, member := range []struct {
				action Action
				want   bool
			}{{action: a, want: tc.wantA}, {action: b, want: tc.wantB}} {
				present, err := got.Contains(member.action)
				if err != nil || present != member.want {
					t.Fatalf("Contains(%v) = %t/%v, want %t/nil", member.action, present, err, member.want)
				}
			}
			encoded, err := got.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON = %v, want nil", err)
			}
			var decoded Actions
			if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
				t.Fatalf("round trip = %v/%v, want %v/nil", decoded, err, got)
			}
		})
	}
}

func FuzzActionSemanticClosure(f *testing.F) {
	seed := permitAction(f, "operation-a")
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON seed = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrPermitContract) || !errors.Is(err, core.ErrJSONContract) || got != seed {
				t.Fatalf("refusal = %v/%v, want typed error and unchanged %v", got, err, seed)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted Validate = %v, want nil", err)
		}
		var source string
		if err := json.Unmarshal(data, &source); err != nil || got.String() != source {
			t.Fatalf("accepted identity = %q/%v, want source %q/nil", got.String(), err, source)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > ActionMaximumBytes+2 {
			t.Fatalf("canonical = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip Action
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("round trip = %v/%v, want %v/nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("canonical fixed point = %q/%v, want %q/nil", second, err, encoded)
		}
	})
}

func FuzzActionsSemanticClosure(f *testing.F) {
	seed, err := NewActions(permitAction(f, "operation-a"))
	if err != nil {
		f.Fatalf("NewActions = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON seed = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte(`[]`))
	f.Add([]byte(`null`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrPermitContract) || !errors.Is(err, core.ErrJSONContract) || got != seed {
				t.Fatalf("refusal = %v/%v, want typed error and unchanged %v", got, err, seed)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("accepted Validate = %v, want nil", err)
		}
		var source []string
		if err := json.Unmarshal(data, &source); err != nil {
			t.Fatalf("accepted source decode = %v, want nil", err)
		}
		slices.Sort(source)
		count, err := got.Count()
		if err != nil || count != len(source) {
			t.Fatalf("accepted count = %d/%v, want %d/nil", count, err, len(source))
		}
		for i, name := range source {
			if got.values[i].String() != name {
				t.Fatalf("accepted identity[%d] = %q, want %q", i, got.values[i].String(), name)
			}
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > ActionsMaximumCount*(ActionMaximumBytes+3)+2 {
			t.Fatalf("canonical = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip Actions
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("round trip = %v/%v, want %v/nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("canonical fixed point = %q/%v, want %q/nil", second, err, encoded)
		}
	})
}

func TestActionSetCardinalityBoundary(t *testing.T) {
	t.Parallel()
	for _, count := range []int{ActionsMaximumCount - 1, ActionsMaximumCount, ActionsMaximumCount + 1} {
		t.Run(fmt.Sprintf("distinct action count %d", count), func(t *testing.T) {
			t.Parallel()
			values := make([]Action, count)
			for i := range values {
				values[i] = permitAction(t, fmt.Sprintf("operation-%02d", i))
			}
			got, err := NewActions(values...)
			if count > ActionsMaximumCount {
				if !errors.Is(err, core.ErrPermitContract) || got != (Actions{}) {
					t.Fatalf("oversized set = %v/%v, want zero/typed refusal", got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewActions = %v, want nil", err)
			}
			for _, action := range values {
				present, err := got.Contains(action)
				if err != nil || !present {
					t.Fatalf("Contains(%v) = %t/%v, want true/nil", action, present, err)
				}
			}
			values[0] = Action{}
			if err := got.Validate(); err != nil {
				t.Fatalf("caller mutation changed owned set = %v, want nil", err)
			}
		})
	}
}

func TestRouteActionDomainExhaustive(t *testing.T) {
	t.Parallel()
	for value := 0; value <= 255; value++ {
		family := controlwire.RouteFamily(value)
		got, err := ActionForRoute(family)
		if family.Validate() != nil {
			if !errors.Is(err, core.ErrPermitContract) || got != (Action{}) {
				t.Fatalf("route %d = %v/%v, want zero/refusal", value, got, err)
			}
			continue
		}
		if err != nil || got.String() != "route."+family.Token() {
			t.Fatalf("route %v = %v/%v, want exact shared name/nil", family, got, err)
		}
	}
}

func FuzzParseActionSemanticClosure(f *testing.F) {
	seed := permitAction(f, "operation-a")
	f.Add(seed.String())
	f.Add("")
	f.Add("\x00")
	grammar := regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$`)
	f.Fuzz(func(t *testing.T, text string) {
		got, err := ParseAction(text)
		wantAccepted := grammar.MatchString(text)
		if !wantAccepted {
			if !errors.Is(err, core.ErrPermitContract) || got != (Action{}) {
				t.Fatalf("ParseAction = %v/%v, want zero/typed refusal", got, err)
			}
			return
		}
		if err != nil || got.String() != text {
			t.Fatalf("ParseAction = %q/%v, want %q/nil", got.String(), err, text)
		}
	})
}
