package jsonio_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// nativeStamp owns no JSON methods, like a Temporal instant whose document
// spelling is lent by the document owner: exact nanoseconds as one integer.
type nativeStamp struct{ nanoseconds int64 }

var errNativeStampRange = errors.New("native stamp outside the owner's range")

func stampCodec(t testing.TB, restoreCalls *int) jsonio.NativeCodec {
	t.Helper()
	codec, err := jsonio.ProjectNative(func(value nativeStamp) (int64, error) {
		if value.nanoseconds == math.MinInt64 {
			return 0, errNativeStampRange
		}
		return value.nanoseconds, nil
	}, func(projected int64) (nativeStamp, error) {
		if restoreCalls != nil {
			*restoreCalls++
		}
		if projected == math.MinInt64 {
			return nativeStamp{}, errNativeStampRange
		}
		return nativeStamp{nanoseconds: projected}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

type nativeStampDocument struct {
	Text  string         `json:"text"`
	When  nativeStamp    `json:"when"`
	Maybe *nativeStamp   `json:"maybe"`
	List  []nativeStamp  `json:"list"`
	Pair  [2]nativeStamp `json:"pair"`
}

func (d nativeStampDocument) Validate() error {
	if d.Text == "" {
		return core.ErrJSONContract
	}
	return nil
}

// nativeStampControl is the hook-free twin of the document: the lent codec
// must spell exactly what Go spells for declared int64 members.
type nativeStampControl struct {
	Text  string   `json:"text"`
	When  int64    `json:"when"`
	Maybe *int64   `json:"maybe"`
	List  []int64  `json:"list"`
	Pair  [2]int64 `json:"pair"`
}

func (d nativeStampControl) Validate() error {
	if d.Text == "" {
		return core.ErrJSONContract
	}
	return nil
}

func permitted() jsonio.OwnedCodecPolicy {
	return jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly, Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullPermitted}
}

func TestNativeCodecProjectsOwnerWithoutMethodsThroughBothDoors(t *testing.T) {
	t.Parallel()
	codec := stampCodec(t, nil)
	seven := int64(7)
	document := nativeStampDocument{Text: "x", When: nativeStamp{math.MaxInt64}, Maybe: &nativeStamp{7}, List: []nativeStamp{{-1}, {0}}, Pair: [2]nativeStamp{{1}, {math.MinInt64 + 1}}}
	control := nativeStampControl{Text: "x", When: math.MaxInt64, Maybe: &seven, List: []int64{-1, 0}, Pair: [2]int64{1, math.MinInt64 + 1}}
	want, err := jsonio.MarshalValidated(control, permitted())
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonio.MarshalValidatedWith(document, permitted(), codec)
	if err != nil || string(got) != string(want) {
		t.Fatalf("MarshalValidatedWith = %s/%v, want control %s", got, err, want)
	}
	decoded, err := jsonio.DecodeOwnedWith[nativeStampDocument](got, permitted(), codec)
	if err != nil || decoded.Text != "x" || decoded.When != document.When || decoded.Maybe == nil || *decoded.Maybe != *document.Maybe || len(decoded.List) != 2 || decoded.List[0] != document.List[0] || decoded.List[1] != document.List[1] || decoded.Pair != document.Pair {
		t.Fatalf("DecodeOwnedWith = %+v/%v, want the original owner facts", decoded, err)
	}
	nilMaybe := nativeStampDocument{Text: "x"}
	encoded, err := jsonio.MarshalValidatedWith(nilMaybe, permitted(), codec)
	if err != nil || string(encoded) != `{"text":"x","when":0,"maybe":null,"list":[],"pair":[0,0]}` {
		t.Fatalf("nil pointer owner = %s/%v, want null under the permitted arm", encoded, err)
	}
	if _, err := jsonio.MarshalValidatedWith(nilMaybe, jsonio.OwnedCodecPolicy{}, codec); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil pointer owner under the refused arm = %v, want ErrJSONContract", err)
	}
}

func TestNativeCodecNullNeverRestoresAndPointerNullFollowsPolicy(t *testing.T) {
	t.Parallel()
	restoreCalls := 0
	codec := stampCodec(t, &restoreCalls)
	for _, tc := range []struct {
		name     string
		data     string
		policy   jsonio.OwnedCodecPolicy
		wantErr  bool
		wantNil  bool
		restores int
	}{
		{"projected non-pointer null refuses under the permitted arm", `{"text":"x","when":null,"maybe":null,"list":null,"pair":[0,0]}`, permitted(), true, false, 0},
		{"projected non-pointer null refuses under the refused arm", `{"text":"x","when":null,"maybe":{"n":1},"list":[],"pair":[0,0]}`, jsonio.OwnedCodecPolicy{}, true, false, 0},
		{"array element null refuses before restore", `{"text":"x","when":1,"maybe":2,"list":[null],"pair":[0,0]}`, permitted(), true, false, 2},
		{"pointer null is nil under the permitted arm", `{"text":"x","when":1,"maybe":null,"list":[],"pair":[0,0]}`, permitted(), false, true, 3},
		{"pointer null refuses under the refused arm", `{"text":"x","when":1,"maybe":null,"list":[],"pair":[0,0]}`, jsonio.OwnedCodecPolicy{}, true, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreCalls = 0
			decoded, err := jsonio.DecodeOwnedWith[nativeStampDocument]([]byte(tc.data), tc.policy, codec)
			if (err != nil) != tc.wantErr || err != nil && (!errors.Is(err, core.ErrJSONContract) || decoded.Text != "" || decoded.List != nil) {
				t.Fatalf("DecodeOwnedWith = %+v/%v, want refusal=%t with zero", decoded, err, tc.wantErr)
			}
			if err == nil && (decoded.Maybe == nil) != tc.wantNil {
				t.Fatalf("pointer member = %v, want nil=%t", decoded.Maybe, tc.wantNil)
			}
			if restoreCalls > tc.restores {
				t.Fatalf("restore ran %d times, want at most %d", restoreCalls, tc.restores)
			}
		})
	}
}

type methodOwner struct{ value int64 }

func (methodOwner) MarshalJSON() ([]byte, error)  { return []byte(`"owner-method"`), nil }
func (o *methodOwner) UnmarshalJSON([]byte) error { o.value = -1; return nil }

type methodOwnerDocument struct {
	Owner methodOwner `json:"owner"`
}

func (methodOwnerDocument) Validate() error { return nil }

type hookedProjection struct{ Value int64 }

func (hookedProjection) MarshalJSON() ([]byte, error) { return []byte(`1`), nil }

func TestNativeCodecPrecedenceAdmissionAndOwnerCauses(t *testing.T) {
	t.Parallel()
	lent, err := jsonio.ProjectNative(func(value methodOwner) (int64, error) { return value.value, nil }, func(projected int64) (methodOwner, error) { return methodOwner{value: projected}, nil })
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := jsonio.MarshalValidatedWith(methodOwnerDocument{Owner: methodOwner{5}}, permitted(), lent)
	if err != nil || string(encoded) != `{"owner":5}` {
		t.Fatalf("lent codec over owner methods = %s/%v, want the projection", encoded, err)
	}
	decoded, err := jsonio.DecodeOwnedWith[methodOwnerDocument](encoded, permitted(), lent)
	if err != nil || decoded.Owner.value != 5 {
		t.Fatalf("lent decode over owner methods = %+v/%v, want restored 5", decoded, err)
	}
	plain, err := jsonio.MarshalValidatedWith(methodOwnerDocument{Owner: methodOwner{5}}, permitted())
	if err != nil || string(plain) != `{"owner":"owner-method"}` {
		t.Fatalf("no codec keeps owner methods = %s/%v", plain, err)
	}
	if _, err := jsonio.ProjectNative[nativeStamp, nativeStamp](func(v nativeStamp) (nativeStamp, error) { return v, nil }, func(v nativeStamp) (nativeStamp, error) { return v, nil }); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("owner as its own projection = %v, want ErrJSONContract", err)
	}
	if _, err := jsonio.ProjectNative[nativeStamp, hookedProjection](func(nativeStamp) (hookedProjection, error) { return hookedProjection{}, nil }, func(hookedProjection) (nativeStamp, error) { return nativeStamp{}, nil }); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("projection with JSON hooks = %v, want ErrJSONContract", err)
	}
	if _, err := jsonio.ProjectNative[nativeStamp, map[string]int64](nil, nil); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil projection functions = %v, want ErrJSONContract", err)
	}
	if _, err := jsonio.ProjectNative[nativeStamp, map[string]int64](func(nativeStamp) (map[string]int64, error) { return nil, nil }, func(map[string]int64) (nativeStamp, error) { return nativeStamp{}, nil }); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("map projection = %v, want ErrJSONContract", err)
	}
	if _, err := jsonio.MarshalValidatedWith(methodOwnerDocument{}, permitted(), jsonio.NativeCodec{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("zero codec = %v, want ErrJSONContract", err)
	}
	codec := stampCodec(t, nil)
	if _, err := jsonio.MarshalValidatedWith(nativeStampDocument{Text: "x", When: nativeStamp{math.MinInt64}}, permitted(), codec); !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, errNativeStampRange) {
		t.Fatalf("project refusal = %v, want ErrJSONContract with the owner cause", err)
	}
	decodedStamp, err := jsonio.DecodeOwnedWith[nativeStampDocument]([]byte(`{"text":"x","when":-9223372036854775808,"maybe":null,"list":[],"pair":[0,0]}`), permitted(), codec)
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, errNativeStampRange) || decodedStamp.Text != "" || decodedStamp.When != (nativeStamp{}) {
		t.Fatalf("restore refusal = %+v/%v, want zero, ErrJSONContract and the owner cause", decodedStamp, err)
	}
	if _, err := jsonio.DecodeOwnedWith[nativeStampDocument]([]byte(`{"text":"x","when":1.5,"maybe":null,"list":[],"pair":[0,0]}`), permitted(), codec); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("fraction into an integer projection = %v, want ErrJSONContract", err)
	}
}

type pairProjection struct {
	Low  int64 `json:"low"`
	High int64 `json:"high"`
}

type pairOwner struct{ low, high int64 }

type pairDocument struct {
	Pair pairOwner `json:"pair"`
}

func (pairDocument) Validate() error { return nil }

func TestNativeCodecStructProjectionRefusesUnknownMembers(t *testing.T) {
	t.Parallel()
	codec, err := jsonio.ProjectNative(func(value pairOwner) (pairProjection, error) {
		return pairProjection{Low: value.low, High: value.high}, nil
	}, func(projected pairProjection) (pairOwner, error) {
		return pairOwner{low: projected.Low, high: projected.High}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := jsonio.MarshalValidatedWith(pairDocument{Pair: pairOwner{1, 2}}, permitted(), codec)
	if err != nil || string(encoded) != `{"pair":{"low":1,"high":2}}` {
		t.Fatalf("struct projection = %s/%v", encoded, err)
	}
	decoded, err := jsonio.DecodeOwnedWith[pairDocument](encoded, permitted(), codec)
	if err != nil || decoded.Pair != (pairOwner{1, 2}) {
		t.Fatalf("struct restore = %+v/%v", decoded, err)
	}
	for _, data := range []string{`{"pair":{"low":1,"high":2,"extra":3}}`, `{"pair":{"low":1,"low":2}}`, `{"pair":"\xff"}`, `{"pair":[1,2]}`} {
		if got, err := jsonio.DecodeOwnedWith[pairDocument]([]byte(data), jsonio.OwnedCodecPolicy{Members: jsonio.MembersIgnoreExtra, PointerNulls: jsonio.PointerNullPermitted}, codec); !errors.Is(err, core.ErrJSONContract) || got != (pairDocument{}) {
			t.Fatalf("projection admission of %q = %+v/%v, want zero and ErrJSONContract even when the outer document ignores extra members", data, got, err)
		}
	}
}

func TestNativeCodecZeroCodecsAreTheExistingDoors(t *testing.T) {
	t.Parallel()
	seven := int64(7)
	control := nativeStampControl{Text: "x", When: 1, Maybe: &seven, List: []int64{2}, Pair: [2]int64{3, 4}}
	for _, policy := range []jsonio.OwnedCodecPolicy{permitted(), {}, {Members: jsonio.MembersIgnoreExtra}} {
		want, wantErr := jsonio.MarshalValidated(control, policy)
		got, err := jsonio.MarshalValidatedWith(control, policy)
		if string(want) != string(got) || !errors.Is(err, wantErr) && (err == nil) != (wantErr == nil) {
			t.Fatalf("MarshalValidatedWith() = %s/%v, want MarshalValidated %s/%v", got, err, want, wantErr)
		}
		data := []byte(`{"text":"x","when":1,"maybe":7,"list":[2],"pair":[3,4],"extra":true}`)
		wantDoc, wantErr := jsonio.DecodeOwned[nativeStampControl](data, policy)
		gotDoc, err := jsonio.DecodeOwnedWith[nativeStampControl](data, policy)
		if (err == nil) != (wantErr == nil) || err == nil && (gotDoc.Text != wantDoc.Text || gotDoc.When != wantDoc.When || *gotDoc.Maybe != *wantDoc.Maybe) {
			t.Fatalf("DecodeOwnedWith() = %+v/%v, want DecodeOwned %+v/%v", gotDoc, err, wantDoc, wantErr)
		}
	}
}

// One fact under control: for every nanosecond value the lent codec spells
// and restores exactly what Go spells for a declared int64 member.
func FuzzNativeCodecSemanticClosure(f *testing.F) {
	for _, seed := range []int64{0, 1, -1, math.MaxInt64, math.MinInt64 + 1, 1 << 53, -(1 << 53) - 1} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, nanoseconds int64) {
		if nanoseconds == math.MinInt64 {
			t.Skip("reserved by the owner as its refusal")
		}
		codec := stampCodec(t, nil)
		document := nativeStampDocument{Text: "x", When: nativeStamp{nanoseconds}, List: []nativeStamp{{nanoseconds}}, Pair: [2]nativeStamp{{nanoseconds}, {0}}}
		control := nativeStampControl{Text: "x", When: nanoseconds, List: []int64{nanoseconds}, Pair: [2]int64{nanoseconds, 0}}
		want, err := jsonio.MarshalValidated(control, permitted())
		if err != nil {
			t.Fatal(err)
		}
		got, err := jsonio.MarshalValidatedWith(document, permitted(), codec)
		if err != nil || string(got) != string(want) {
			t.Fatalf("spelling %s/%v differs from control %s", got, err, want)
		}
		decoded, err := jsonio.DecodeOwnedWith[nativeStampDocument](got, permitted(), codec)
		if err != nil || decoded.When != document.When || decoded.List[0] != document.List[0] || decoded.Pair != document.Pair || decoded.Maybe != nil {
			t.Fatalf("restore %+v/%v differs from %+v", decoded, err, document)
		}
	})
}
