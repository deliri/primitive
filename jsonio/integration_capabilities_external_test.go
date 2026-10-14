package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type foreignObject struct {
	Name  string
	Count uint64
	Extra string
}

func (o foreignObject) Validate() error {
	return (scalarObject{Name: o.Name, Count: o.Count}).Validate()
}

func TestJSONForeignMembersAreExplicitAcrossEveryTypedReader(t *testing.T) {
	t.Parallel()
	canonical, err := jsonv2.Marshal(foreignObject{Name: "owned", Count: 7, Extra: "foreign"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, wire string
		want       scalarObject
		wantErr    bool
	}{
		{"extra scalar", string(canonical), scalarObject{Name: "owned", Count: 7}, false},
		{"extra null", `{"Name":"owned","Extra":null}`, scalarObject{Name: "owned"}, false},
		{"extra nested object", `{"Name":"owned","Extra":{"nested":[true,false,null]}}`, scalarObject{Name: "owned"}, false},
		{"extra array", `{"Extra":[1,2,3],"Name":"owned"}`, scalarObject{Name: "owned"}, false},
		{"known field null", `{"Name":"owned","Count":null,"Extra":1}`, scalarObject{}, true},
		{"known wrong type", `{"Name":"owned","Count":"7","Extra":1}`, scalarObject{}, true},
		{"known overflow", `{"Name":"owned","Count":18446744073709551616,"Extra":1}`, scalarObject{}, true},
		{"owner refusal", `{"Name":"","Extra":1}`, scalarObject{}, true},
		{"duplicate known member", `{"Name":"owned","Name":"other","Extra":1}`, scalarObject{}, true},
		{"duplicate unknown member", `{"Name":"owned","Extra":1,"Extra":2}`, scalarObject{}, true},
		{"escaped duplicate unknown member", `{"Name":"owned","Extra":1,"\u0045xtra":2}`, scalarObject{}, true},
		{"malformed ignored value", `{"Name":"owned","Extra":[1,]}`, scalarObject{}, true},
		{"unknown invalid UTF8", "{\"Name\":\"owned\",\"Extra\":\"\xff\"}", scalarObject{}, true},
		{"root null", `null`, scalarObject{}, true},
		{"root array", `[]`, scalarObject{}, true},
	}
	for _, tc := range cases {
		for _, policy := range []jsonio.MemberPolicy{jsonio.MembersDeclaredOnly, jsonio.MembersIgnoreExtra} {
			t.Run(tc.name+"/"+policy.String(), func(t *testing.T) {
				t.Parallel()
				wantErr := tc.wantErr || policy == jsonio.MembersDeclaredOnly
				for door := range 3 {
					var got scalarObject
					var gotErr error
					request := jsonio.ObjectSourceRequest{Source: iotest.OneByteReader(strings.NewReader(tc.wire)), Members: policy}
					switch door {
					case 0:
						decoder, err := jsonio.NewObjectDecoder[scalarObject](request)
						if err != nil {
							t.Fatal(err)
						}
						got, gotErr = decoder.Decode(t.Context())
					case 1:
						for value, err := range jsonio.Objects[scalarObject](t.Context(), request) {
							got, gotErr = value, err
							break
						}
					case 2:
						for value, err := range jsonio.Documents[scalarObject](t.Context(), jsonio.Request{Source: request.Source, Limits: core.DefaultStrictJSONLimits(), Members: policy}) {
							got, gotErr = value, err
							break
						}
					}
					if wantErr {
						if !errors.Is(gotErr, core.ErrJSONContract) || got != (scalarObject{}) {
							t.Fatalf("door %d refused = %+v/%v, want zero JSON refusal", door, got, gotErr)
						}
					} else if gotErr != nil || got != tc.want {
						t.Fatalf("door %d = %+v/%v, want %+v/nil", door, got, gotErr, tc.want)
					}
				}
			})
		}
	}
}

func TestJSONMemberPolicyExhaustiveAndResetKeepsRefusedSource(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		policy := jsonio.MemberPolicy(value)
		got := policy.Validate()
		if value < 2 && got != nil || value >= 2 && !errors.Is(got, core.ErrJSONContract) {
			t.Fatalf("policy %d = %v, want closed domain [0,2)", value, got)
		}
	}
	decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: strings.NewReader(`{"Name":"kept"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.Reset(t.Context(), jsonio.ObjectSourceRequest{Source: strings.NewReader("broken"), Members: 255}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("Reset(future policy) = %v, want JSON refusal", err)
	}
	got, err := decoder.Decode(t.Context())
	if got != (scalarObject{Name: "kept"}) || err != nil {
		t.Fatalf("refused Reset changed source = %+v/%v", got, err)
	}
}

func TestJSONCursorSkipCaptureAndTypedArrayUseNativeFile(t *testing.T) {
	t.Parallel()
	for _, items := range []int{0, 1, 32767, 32768, 32769, 262151} {
		t.Run(fmt.Sprintf("skipped-items-%d", items), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "array.json")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: file})
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
					t.Fatal(err)
				}
			}
			for range items {
				if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenTrue}); err != nil {
					t.Fatal(err)
				}
			}
			if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
				t.Fatal(err)
			}
			want := scalarObject{Name: "capture", Count: 9007199254740993}
			if err := jsonio.EncodeObject(t.Context(), encoder, want); err != nil {
				t.Fatal(err)
			}
			if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenNumber, Text: "1e400"}); err != nil {
				t.Fatal(err)
			}
			if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			decoder, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: file, NestingDepthMaximum: 2})
			if err != nil {
				t.Fatal(err)
			}
			first, err := decoder.Decode(t.Context())
			if err != nil || first.Kind != jsonio.TokenArrayStart {
				t.Fatalf("first = %+v/%v, want array start", first, err)
			}
			if err := decoder.SkipValue(t.Context()); err != nil {
				t.Fatal(err)
			}
			captured, err := decoder.ReadValue(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := jsonv2.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			for _, token := range []jsonio.Token{{Kind: jsonio.TokenNumber, Text: "1e400"}, {Kind: jsonio.TokenArrayEnd}} {
				got, err := decoder.Decode(t.Context())
				if err != nil || got != token {
					t.Fatalf("after skip/capture = %+v/%v, want %+v", got, err, token)
				}
			}
			if got, err := decoder.ReadValue(t.Context()); got != nil || err != io.EOF {
				t.Fatalf("closure = %q/%v, want nil/EOF", got, err)
			}
			if !bytes.Equal(captured, canonical) {
				t.Fatalf("capture changed after cursor advancement = %q, want %q", captured, canonical)
			}
			if _, err := file.Stat(); err != nil {
				t.Fatalf("cursor closed borrowed file = %v", err)
			}
		})
	}
}

type ownedCustomObject struct {
	Name    string
	Wire    string
	Failure error
}

func (o ownedCustomObject) Validate() error {
	if o.Name == "" {
		return core.ErrSourceObservationContract
	}
	return nil
}
func (o ownedCustomObject) MarshalJSON() ([]byte, error) {
	if o.Failure != nil {
		return nil, o.Failure
	}
	if o.Wire != "" {
		return []byte(o.Wire), nil
	}
	return jsonv2.Marshal(scalarObject{Name: o.Name})
}

func TestJSONCustomMarshalerAdmissionCannotBypassObjectGrammar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		object ownedCustomObject
		want   error
	}{
		{"owner emits object", ownedCustomObject{Name: "owned"}, nil},
		{"owner emits whitespace object", ownedCustomObject{Name: "owned", Wire: " {\"Name\":\"owned\"} "}, nil},
		{"owner refuses meaning", ownedCustomObject{}, core.ErrSourceObservationContract},
		{"marshaler native refusal", ownedCustomObject{Name: "owned", Failure: io.ErrClosedPipe}, io.ErrClosedPipe},
		{"scalar bypass", ownedCustomObject{Name: "owned", Wire: "1"}, core.ErrJSONContract},
		{"null bypass", ownedCustomObject{Name: "owned", Wire: "null"}, core.ErrJSONContract},
		{"array bypass", ownedCustomObject{Name: "owned", Wire: "[]"}, core.ErrJSONContract},
		{"two objects", ownedCustomObject{Name: "owned", Wire: "{}{}"}, core.ErrJSONContract},
		{"duplicate names", ownedCustomObject{Name: "owned", Wire: `{"a":1,"a":2}`}, core.ErrJSONContract},
		{"invalid utf8", ownedCustomObject{Name: "owned", Wire: "{\"a\":\"\xff\"}"}, core.ErrJSONContract},
		{"truncated object", ownedCustomObject{Name: "owned", Wire: "{"}, core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &output})
			if err != nil {
				t.Fatal(err)
			}
			got := jsonio.EncodeMarshaledObject(t.Context(), encoder, tc.object)
			if tc.want != nil {
				if !errors.Is(got, tc.want) || output.Len() != 0 {
					t.Fatalf("custom refusal = %v, bytes %d, want %v and zero effects", got, output.Len(), tc.want)
				}
			} else if got != nil || !jsontext.Value(output.Bytes()).IsValid() {
				t.Fatalf("custom object = %q/%v, want valid object", output.Bytes(), got)
			}
		})
	}
}

func TestJSONCursorDepthCancellationAndNilRefusals(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"skip", "capture"} {
		for _, depth := range []uint16{1, 2, 3} {
			t.Run(fmt.Sprintf("%s/depth-%d", operation, depth), func(t *testing.T) {
				t.Parallel()
				decoder, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: strings.NewReader(`[{"x":[]}]`), NestingDepthMaximum: depth})
				if err != nil {
					t.Fatal(err)
				}
				var captured []byte
				if operation == "skip" {
					err = decoder.SkipValue(t.Context())
				} else {
					captured, err = decoder.ReadValue(t.Context())
				}
				if depth < 3 {
					if !errors.Is(err, core.ErrJSONContract) || captured != nil {
						t.Fatalf("depth refusal = %q/%v", captured, err)
					}
				} else if err != nil {
					t.Fatalf("exact depth = %v, want nil", err)
				}
			})
		}
	}
	var absent *jsonio.TokenDecoder
	if err := absent.SkipValue(t.Context()); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil SkipValue = %v", err)
	}
	if data, err := absent.ReadValue(t.Context()); data != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil ReadValue = %q/%v", data, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	source := &jsonStreamReadCounter{Reader: strings.NewReader("{}")}
	decoder, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: source, NestingDepthMaximum: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := decoder.SkipValue(ctx); !errors.Is(err, context.Canceled) || source.calls != 0 {
		t.Fatalf("pre-cancel skip = %v, reads %d", err, source.calls)
	}
}

func FuzzJSONOpenObjectsAndCursorSemanticClosure(f *testing.F) {
	var seed bytes.Buffer
	writer, err := jsonio.NewObjectEncoder[foreignObject](jsonio.ObjectDestinationRequest{Destination: &seed})
	if err != nil {
		f.Fatal(err)
	}
	if err := writer.Encode(f.Context(), foreignObject{Name: "seed", Count: 7, Extra: "foreign"}); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes(), uint8(jsonio.MembersIgnoreExtra))
	f.Add([]byte(`{"Name":null}`), uint8(0))
	f.Add([]byte(`{"Name":"seed","Extra":1,"Extra":2}`), uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, member uint8) {
		policy := jsonio.MemberPolicy(member)
		decoder, err := jsonio.NewObjectDecoder[scalarObject](jsonio.ObjectSourceRequest{Source: bytes.NewReader(data), Members: policy})
		if !policy.IsValid() {
			if decoder != nil || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("future policy = %v/%v", decoder, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := decoder.Decode(t.Context())
		if err != nil {
			if got != (scalarObject{}) || err != io.EOF && !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("refusal = %+v/%v", got, err)
			}
			return
		}
		var independent scalarObject
		if err := jsonv2.Unmarshal(data, &independent, jsonv2.RejectUnknownMembers(policy == jsonio.MembersDeclaredOnly)); err != nil {
			// A sequence may follow the admitted prefix. Its closure is separately
			// tested; compare the exact first native captured value instead.
			cursor, createErr := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: bytes.NewReader(data), NestingDepthMaximum: core.JSONNestingDepthMaximum})
			if createErr != nil {
				t.Fatal(createErr)
			}
			first, readErr := cursor.ReadValue(t.Context())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if err := jsonv2.Unmarshal(first, &independent, jsonv2.RejectUnknownMembers(policy == jsonio.MembersDeclaredOnly)); err != nil {
				t.Fatal(err)
			}
		}
		if got.Validate() != nil || got != independent {
			t.Fatalf("accepted = %+v, want independent %+v", got, independent)
		}
		var encoded bytes.Buffer
		encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &encoded})
		if err != nil {
			t.Fatal(err)
		}
		if err := jsonio.EncodeObject(t.Context(), encoder, got); err != nil {
			t.Fatal(err)
		}
		var round scalarObject
		if err := jsonv2.Unmarshal(encoded.Bytes(), &round); err != nil || round != got {
			t.Fatalf("typed round trip = %+v/%v, want %+v", round, err, got)
		}
	})
}
