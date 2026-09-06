package attest_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json/jsontext"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

// canonicalMember is a well-behaved nested member owner.
type canonicalMember struct {
	encoded string
	valid   bool
}

func (m canonicalMember) Validate() error {
	if !m.valid {
		return core.ErrAttestContract
	}
	return nil
}

func (m canonicalMember) MarshalJSON() ([]byte, error) {
	return []byte(m.encoded), nil
}

// panickingMember proves a hostile member owner cannot escape attest's error
// identities or leave a partially written document behind.
type panickingMember struct {
	inValidate bool
}

func (m panickingMember) Validate() error {
	if m.inValidate {
		panic("hostile validate")
	}
	return nil
}

func (m panickingMember) MarshalJSON() ([]byte, error) {
	panic("hostile marshal")
}

// erroringMember returns a failure from MarshalJSON rather than panicking.
type erroringMember struct{}

func (erroringMember) Validate() error { return nil }

func (erroringMember) MarshalJSON() ([]byte, error) {
	return nil, fixtureErrorMarshal
}

// observedMember records whether CanonicalObject invoked a nested owner. It
// proves object-owned admission happens before the external callback boundary.
type observedMember struct {
	calls *int
}

func (m observedMember) Validate() error {
	*m.calls++
	return nil
}

func (m observedMember) MarshalJSON() ([]byte, error) {
	*m.calls++
	return []byte(`1`), nil
}

// builtBody is a CanonicalBody whose canonical bytes come from the production
// builder, so signing proof runs over the real emission path rather than a
// second projection written for the test.
type builtBody struct {
	commit string
	count  uint64
}

func (builtBody) Validate() error { return nil }

func (builtBody) AttestationDomain() testDomain { return testDomainPrimary }

func (b builtBody) canonical() ([]byte, error) {
	object := attest.BeginCanonicalObject(nil)
	object.String("commit", b.commit)
	object.Uint64("count", b.count)
	return object.End()
}

func (b builtBody) WriteCanonical(destination io.Writer) error {
	canonical, err := b.canonical()
	if err != nil {
		return err
	}
	written, err := destination.Write(canonical)
	if err != nil {
		return err
	}
	if written != len(canonical) {
		return io.ErrShortWrite
	}
	return nil
}

func TestCanonicalObjectMemberNameGrammarAdmitsExactlyTheCanonicalWordShape(t *testing.T) {
	t.Parallel()

	longestName := strings.Repeat("a", attest.CanonicalFieldNameMaximumBytes)
	overLongName := strings.Repeat("a", attest.CanonicalFieldNameMaximumBytes+1)

	cases := []struct {
		name     string
		field    string
		wantName bool
	}{
		{name: "single lowercase letter is the shortest legal name", field: "a", wantName: true},
		{name: "single decimal digit is legal because names are words not identifiers", field: "0", wantName: true},
		{name: "highest lowercase byte is admitted", field: "z", wantName: true},
		{name: "highest decimal byte is admitted", field: "9", wantName: true},
		{name: "one interior separator is legal", field: "run_id", wantName: true},
		{name: "longest legal name is admitted at the exact boundary", field: longestName, wantName: true},

		{name: "empty name is rejected at the lower boundary", field: "", wantName: false},
		{name: "one byte over the extent boundary is rejected", field: overLongName, wantName: false},
		{name: "leading separator is rejected", field: "_commit", wantName: false},
		{name: "trailing separator is rejected", field: "commit_", wantName: false},
		{name: "doubled interior separator is rejected", field: "run__id", wantName: false},
		{name: "uppercase letter is rejected because case folding would collide", field: "runID", wantName: false},
		{name: "hyphen is rejected because the separator is exclusive", field: "run-id", wantName: false},

		{name: "quote is rejected because a name must never need escaping", field: "run\"id", wantName: false},
		{name: "backslash is rejected because a name must never need escaping", field: "run\\id", wantName: false},
		{name: "null byte is rejected", field: "run\x00id", wantName: false},
		{name: "non-ascii multibyte rune is rejected", field: "runé", wantName: false},
		{name: "byte one below lowercase a is rejected", field: "run`id", wantName: false},
		{name: "byte one above lowercase z is rejected", field: "run{id", wantName: false},
		{name: "byte one above decimal nine is rejected", field: "run:id", wantName: false},
		{name: "byte one below decimal zero is rejected", field: "run/id", wantName: false},
		{name: "one byte under the longest legal name stays admitted", field: overLongName[:attest.CanonicalFieldNameMaximumBytes-1], wantName: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			object := attest.BeginCanonicalObject(nil)
			object.Bool(tc.field, true)
			got, gotErr := object.End()
			if !tc.wantName {
				if !errors.Is(gotErr, core.ErrAttestContract) {
					t.Fatalf("End() after member %q = (%q, %v), want (nil, %v)", tc.field, got, gotErr, core.ErrAttestContract)
				}
				if got != nil {
					t.Fatalf("End() after rejected member %q = %q, want nil", tc.field, got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("End() after member %q error = %v, want nil", tc.field, gotErr)
			}
			want := `{"` + tc.field + `":true}`
			if string(got) != want {
				t.Fatalf("End() after member %q = %q, want %q", tc.field, got, want)
			}
		})
	}
}

func TestCanonicalObjectTypedMembersEmitExactlyOneAcceptedEncoding(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		write func(*attest.CanonicalObject)
		want  string
	}{
		{name: "empty string member keeps its quotes", write: func(o *attest.CanonicalObject) { o.String("v", "") }, want: `{"v":""}`},
		{name: "plain string member is unescaped", write: func(o *attest.CanonicalObject) { o.String("v", "abc") }, want: `{"v":"abc"}`},
		{name: "html metacharacters remain direct JSON v2 UTF8", write: func(o *attest.CanonicalObject) { o.String("v", "a<b>c&d") }, want: `{"v":"a<b>c&d"}`},
		{name: "quote inside a string value is escaped exactly once", write: func(o *attest.CanonicalObject) { o.String("v", `a"b`) }, want: `{"v":"a\"b"}`},
		{name: "backslash inside a string value is escaped", write: func(o *attest.CanonicalObject) { o.String("v", `a\b`) }, want: `{"v":"a\\b"}`},
		{name: "control byte inside a string value uses the short escape", write: func(o *attest.CanonicalObject) { o.String("v", "a\nb") }, want: `{"v":"a\nb"}`},
		{name: "multibyte rune survives as literal utf-8", write: func(o *attest.CanonicalObject) { o.String("v", "é") }, want: `{"v":"é"}`},

		{name: "signed zero has no sign", write: func(o *attest.CanonicalObject) { o.Int64("v", 0) }, want: `{"v":0}`},
		{name: "signed one is bare", write: func(o *attest.CanonicalObject) { o.Int64("v", 1) }, want: `{"v":1}`},
		{name: "signed negative one carries exactly one minus", write: func(o *attest.CanonicalObject) { o.Int64("v", -1) }, want: `{"v":-1}`},
		{name: "signed maximum is exact at the int64 ceiling", write: func(o *attest.CanonicalObject) { o.Int64("v", math.MaxInt64) }, want: `{"v":9223372036854775807}`},
		{name: "signed minimum is exact at the int64 floor", write: func(o *attest.CanonicalObject) { o.Int64("v", math.MinInt64) }, want: `{"v":-9223372036854775808}`},
		{name: "one below the signed maximum is exact", write: func(o *attest.CanonicalObject) { o.Int64("v", math.MaxInt64-1) }, want: `{"v":9223372036854775806}`},
		{name: "one above the signed minimum is exact", write: func(o *attest.CanonicalObject) { o.Int64("v", math.MinInt64+1) }, want: `{"v":-9223372036854775807}`},

		{name: "unsigned zero is bare", write: func(o *attest.CanonicalObject) { o.Uint64("v", 0) }, want: `{"v":0}`},
		{name: "unsigned maximum is exact at the uint64 ceiling", write: func(o *attest.CanonicalObject) { o.Uint64("v", math.MaxUint64) }, want: `{"v":18446744073709551615}`},
		{name: "one below the unsigned maximum is exact", write: func(o *attest.CanonicalObject) { o.Uint64("v", math.MaxUint64-1) }, want: `{"v":18446744073709551614}`},
		{name: "unsigned value above the signed ceiling is exact", write: func(o *attest.CanonicalObject) { o.Uint64("v", uint64(math.MaxInt64)+1) }, want: `{"v":9223372036854775808}`},

		{name: "boolean true is a bare literal", write: func(o *attest.CanonicalObject) { o.Bool("v", true) }, want: `{"v":true}`},
		{name: "boolean false is a bare literal", write: func(o *attest.CanonicalObject) { o.Bool("v", false) }, want: `{"v":false}`},

		{name: "nested member is inlined exactly as its owner emitted it", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `{"a":1}`, valid: true})
		}, want: `{"v":{"a":1}}`},
		{name: "nested array member is inlined exactly", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `[1,2]`, valid: true})
		}, want: `{"v":[1,2]}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			object := attest.BeginCanonicalObject(nil)
			tc.write(&object)
			got, gotErr := object.End()
			if gotErr != nil {
				t.Fatalf("End() error = %v, want nil", gotErr)
			}
			if string(got) != tc.want {
				t.Fatalf("canonical object = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCanonicalObjectRejectsHostileMemberOwnersWithoutPartialOutput(t *testing.T) {
	t.Parallel()

	oversized := strings.Repeat("x", attest.CanonicalBodyMaximumBytes+1)

	cases := []struct {
		write func(*attest.CanonicalObject)
		name  string
	}{
		{name: "nil member owner is refused", write: func(o *attest.CanonicalObject) { o.Value("v", nil) }},
		{name: "member owner whose invariant fails is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `1`, valid: false})
		}},
		{name: "member owner that panics in validate is contained", write: func(o *attest.CanonicalObject) {
			o.Value("v", panickingMember{inValidate: true})
		}},
		{name: "member owner that panics in marshal is contained", write: func(o *attest.CanonicalObject) {
			o.Value("v", panickingMember{})
		}},
		{name: "member owner that returns an error is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", erroringMember{})
		}},
		{name: "member owner emitting empty output is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: "", valid: true})
		}},
		{name: "member owner emitting json null is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: "null", valid: true})
		}},
		{name: "member owner emitting whitespace wrapped json null is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: " \nnull\t", valid: true})
		}},
		{name: "member owner emitting invalid json is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `{"a":}`, valid: true})
		}},
		{name: "member owner emitting duplicate nested names is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `{"a":1,"a":2}`, valid: true})
		}},
		{name: "member owner emitting case folded duplicate nested names is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `{"a":1,"A":2}`, valid: true})
		}},
		{name: "member owner emitting a bare unquoted word is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `abc`, valid: true})
		}},
		{name: "member owner emitting invalid utf-8 is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: "\"\xff\"", valid: true})
		}},
		{name: "member owner emitting more than the body ceiling is refused", write: func(o *attest.CanonicalObject) {
			o.Value("v", canonicalMember{encoded: `"` + oversized + `"`, valid: true})
		}},
		{name: "string member carrying invalid utf-8 is refused", write: func(o *attest.CanonicalObject) {
			o.String("v", "\xff")
		}},
		{name: "string member larger than the complete object ceiling is refused", write: func(o *attest.CanonicalObject) {
			o.String("v", oversized)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			object := attest.BeginCanonicalObject(nil)
			tc.write(&object)
			got, gotErr := object.End()
			if !errors.Is(gotErr, core.ErrAttestContract) {
				t.Fatalf("End() = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
			}
			if got != nil {
				t.Fatalf("End() after refusal = %q, want nil", got)
			}
		})
	}
}

func TestCanonicalObjectStateTransitionsRefuseReuseDuplicatesAndOverflow(t *testing.T) {
	t.Parallel()

	t.Run("a repeated member name is refused rather than emitted twice", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(nil)
		object.Uint64("run_id", 1)
		object.Uint64("run_id", 2)
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
			t.Fatalf("End() with a duplicate name = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
		}
	})

	t.Run("distinct names that differ only after the duplicate check stay admitted", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(nil)
		object.Uint64("run_id", 1)
		object.Uint64("run_ids", 2)
		got, gotErr := object.End()
		want := `{"run_id":1,"run_ids":2}`
		if gotErr != nil || string(got) != want {
			t.Fatalf("End() = (%q, %v), want (%q, nil)", got, gotErr, want)
		}
	})

	t.Run("member count pressures both sides of the ceiling without losing fields", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name    string
			count   int
			wantErr error
		}{
			{name: "one below maximum", count: attest.CanonicalObjectMaximumFields - 1},
			{name: "exact maximum", count: attest.CanonicalObjectMaximumFields},
			{name: "one above maximum", count: attest.CanonicalObjectMaximumFields + 1, wantErr: core.ErrAttestContract},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				object := attest.BeginCanonicalObject(nil)
				var oracle bytes.Buffer
				encoder := jsontext.NewEncoder(&oracle)
				if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
					t.Fatalf("oracle opening token error = %v, want nil", err)
				}
				for index := range tc.count {
					name := "f" + strconv.Itoa(index)
					object.Uint64(name, uint64(index))
					for _, token := range []jsontext.Token{jsontext.String(name), jsontext.Uint(uint64(index))} {
						if err := encoder.WriteToken(token); err != nil {
							t.Fatalf("oracle member token error = %v, want nil", err)
						}
					}
				}
				if err := encoder.WriteToken(jsontext.EndObject); err != nil {
					t.Fatalf("oracle closing token error = %v, want nil", err)
				}
				got, err := object.End()
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("End() error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if got != nil {
						t.Fatalf("rejected object = %q, want nil", got)
					}
					return
				}
				want := bytes.TrimSuffix(oracle.Bytes(), []byte{'\n'})
				if !bytes.Equal(got, want) {
					t.Fatalf("bounded fields = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("a second End on the same object is refused", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(nil)
		object.Bool("v", true)
		if _, gotErr := object.End(); gotErr != nil {
			t.Fatalf("first End() error = %v, want nil", gotErr)
		}
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
			t.Fatalf("second End() = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
		}
	})

	t.Run("a member after End cannot invoke its owner or mutate caller storage", func(t *testing.T) {
		t.Parallel()
		storage := make([]byte, 0, 128)
		object := attest.BeginCanonicalObject(storage)
		object.Bool("v", true)
		got, err := object.End()
		if err != nil || string(got) != `{"v":true}` {
			t.Fatalf("first End() = (%q, %v), want complete true member", got, err)
		}
		before := bytes.Clone(storage[:cap(storage)])
		calls := 0
		object.Value("late", observedMember{calls: &calls})
		if calls != 0 || !bytes.Equal(storage[:cap(storage)], before) {
			t.Fatalf("late member effects = (%d callbacks, storage unchanged %t), want 0 and true", calls, bytes.Equal(storage[:cap(storage)], before))
		}
		got, err = object.End()
		if !errors.Is(err, core.ErrAttestContract) || got != nil {
			t.Fatalf("End() after late member = (%q, %v), want nil and %v", got, err, core.ErrAttestContract)
		}
	})

	t.Run("the zero object is not usable", func(t *testing.T) {
		t.Parallel()

		var object attest.CanonicalObject
		object.Bool("v", true)
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
			t.Fatalf("zero object End() = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
		}
	})

	t.Run("a destination already above the body ceiling is refused before any member", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(make([]byte, attest.CanonicalBodyMaximumBytes+1))
		object.Bool("v", true)
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
			t.Fatalf("End() over an oversized destination = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
		}
	})

	t.Run("a destination exactly at the body ceiling is refused before any member", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(make([]byte, attest.CanonicalBodyMaximumBytes))
		object.Bool("v", true)
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
			t.Fatalf("End() at an exhausted destination = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
		}
	})

	t.Run("an invalid field name is rejected before its nested owner is invoked", func(t *testing.T) {
		t.Parallel()

		calls := 0
		object := attest.BeginCanonicalObject(nil)
		object.Value("INVALID", observedMember{calls: &calls})
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
			t.Fatalf("End() after invalid nested name = (%q, %v), want (nil, %v)", got, gotErr, core.ErrAttestContract)
		}
		if calls != 0 {
			t.Fatalf("nested owner calls after invalid field name = %d, want 0", calls)
		}
	})
}

func TestCanonicalObjectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive: a multi-member object appends after a caller prefix", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject([]byte(`[1,`))
		object.String("commit", "abc")
		object.Uint64("count", 7)
		object.Bool("known", false)
		object.Int64("delta", -3)
		object.Value("nested", canonicalMember{encoded: `{"a":1}`, valid: true})
		got, gotErr := object.End()
		want := `[1,{"commit":"abc","count":7,"known":false,"delta":-3,"nested":{"a":1}}`
		if gotErr != nil {
			t.Fatalf("End() error = %v, want nil", gotErr)
		}
		if string(got) != want {
			t.Fatalf("canonical object = %q, want %q", got, want)
		}
	})

	t.Run("negative: a rejected member yields no bytes at all", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(nil)
		object.String("commit", "abc")
		object.Bool("BAD", true)
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) {
			t.Fatalf("End() error = %v, want %v", gotErr, core.ErrAttestContract)
		}
		if got != nil {
			t.Fatalf("End() after rejection = %q, want nil", got)
		}
	})

	t.Run("neutral: an object with no members is refused instead of emitting an empty fact", func(t *testing.T) {
		t.Parallel()

		object := attest.BeginCanonicalObject(nil)
		got, gotErr := object.End()
		if !errors.Is(gotErr, core.ErrAttestContract) {
			t.Fatalf("End() with no members error = %v, want %v", gotErr, core.ErrAttestContract)
		}
		if got != nil {
			t.Fatalf("End() with no members = %q, want nil", got)
		}
	})
}

func TestCanonicalObjectBodySignsAndVerifiesThroughTheRealAttestPath(t *testing.T) {
	t.Parallel()

	privateKey := deterministicPrivateKey(t, "canonical-object")
	body := builtBody{commit: "abcdef", count: 42}

	envelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{Body: body, Signer: privateKey})
	if gotErr != nil {
		t.Fatalf("Sign() error = %v, want nil", gotErr)
	}

	trusted := mustTrustedKeys(t, mustPublicKey(t, privateKey))
	proof, verifyErr := attest.Verify(attest.VerifyRequest[testDomain]{Body: body, Envelope: envelope, TrustedKeys: trusted})
	if verifyErr != nil {
		t.Fatalf("Verify() error = %v, want nil", verifyErr)
	}
	retained, retainedErr := proof.Envelope()
	if retainedErr != nil || retained != envelope {
		t.Fatalf("verified envelope = (%+v, %v), want %+v", retained, retainedErr, envelope)
	}

	// The digest attest signed must be the digest of the exact bytes the
	// builder emits, not of a second projection produced for the test.
	canonical, canonicalErr := body.canonical()
	if canonicalErr != nil {
		t.Fatalf("canonical() error = %v, want nil", canonicalErr)
	}
	wantDigest := core.NewSHA256Digest(sha256.Sum256(canonical))
	if envelope.BodySHA256 != wantDigest {
		t.Fatalf("signed digest = %v, want %v", envelope.BodySHA256, wantDigest)
	}
	wantLength, lengthErr := core.NewByteCount(uint64(len(canonical)))
	if lengthErr != nil {
		t.Fatalf("NewByteCount() error = %v, want nil", lengthErr)
	}
	if envelope.BodyLength != wantLength {
		t.Fatalf("envelope body length = %v, want %v", envelope.BodyLength, wantLength)
	}

	// One changed member must break verification against the original envelope.
	mutated := builtBody{commit: "abcdef", count: 43}
	gotRejected, verifyErr := attest.Verify(attest.VerifyRequest[testDomain]{Body: mutated, Envelope: envelope, TrustedKeys: trusted})
	if gotRejected != (attest.Verified[testDomain]{}) {
		t.Fatalf("mutated body proof = %+v, want zero", gotRejected)
	}
	if !errors.Is(verifyErr, core.ErrAttestVerification) {
		t.Fatalf("Verify() over a mutated body error = %v, want %v", verifyErr, core.ErrAttestVerification)
	}
}

func TestCanonicalObjectStringExtentIncludesEscapesAndClosingByte(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		value       string
		escapeBytes int
	}{
		{name: "plain UTF8 byte", value: "x", escapeBytes: 1},
		{name: "short control escape", value: "\n", escapeBytes: 2},
		{name: "six byte control escape", value: "\x00", escapeBytes: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Derive framing from the independent standard-library string encoder.
			empty, err := core.MarshalCanonicalJSONDocument(struct {
				V string `json:"v"`
			}{})
			if err != nil {
				t.Fatalf("framing oracle error = %v, want nil", err)
			}
			room := attest.CanonicalBodyMaximumBytes - len(empty)
			for _, boundary := range []struct {
				name  string
				delta int
			}{
				{name: "one byte below maximum", delta: -1},
				{name: "exact maximum including closing brace", delta: 0},
				{name: "one byte above maximum", delta: 1},
				{name: "encoded member itself exceeds remaining storage", delta: 2},
			} {
				delta := boundary.delta
				t.Run(boundary.name, func(t *testing.T) {
					t.Parallel()
					extent := room + delta
					input := strings.Repeat(tc.value, extent/tc.escapeBytes) + strings.Repeat("x", extent%tc.escapeBytes)
					want, err := core.MarshalCanonicalJSONDocument(struct {
						V string `json:"v"`
					}{V: input})
					if err != nil || len(want) != attest.CanonicalBodyMaximumBytes+delta {
						t.Fatalf("oracle extent = (%d, %v), want %d", len(want), err, attest.CanonicalBodyMaximumBytes+delta)
					}
					object := attest.BeginCanonicalObject(nil)
					object.String("v", input)
					got, gotErr := object.End()
					if delta > 0 {
						if !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
							t.Fatalf("oversized End() = (%d bytes, %v), want nil and %v", len(got), gotErr, core.ErrAttestContract)
						}
						return
					}
					if gotErr != nil || !bytes.Equal(got, want) {
						t.Fatalf("bounded End() = (%d bytes, %v), want %d exact oracle bytes", len(got), gotErr, len(want))
					}
				})
			}
		})
	}
}

func TestCanonicalObjectPreservesFirstTypedRefusalAndSuppressesCallbacks(t *testing.T) {
	t.Parallel()
	calls := 0
	object := attest.BeginCanonicalObject(nil)
	object.Value("first", erroringMember{})
	object.Value("later", observedMember{calls: &calls})
	object.Bool("INVALID", true)
	got, gotErr := object.End()
	if !errors.Is(gotErr, fixtureErrorMarshal) || !errors.Is(gotErr, core.ErrAttestContract) || got != nil {
		t.Fatalf("End() = (%q, %v), want nil and original typed marshal refusal", got, gotErr)
	}
	if calls != 0 {
		t.Fatalf("callbacks after terminal refusal = %d, want 0", calls)
	}
}

func TestCanonicalObjectReusedScalarsAllocateNoHeap(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	want, err := core.MarshalCanonicalJSONDocument(struct {
		Signed   int64  `json:"signed"`
		Unsigned uint64 `json:"unsigned"`
		Flag     bool   `json:"flag"`
	}{Signed: math.MinInt64, Unsigned: math.MaxUint64, Flag: true})
	if err != nil {
		t.Fatalf("scalar oracle error = %v, want nil", err)
	}
	destination := make([]byte, 0, len(want))
	gotAllocations := testing.AllocsPerRun(100, func() {
		object := attest.BeginCanonicalObject(destination[:0])
		object.Int64("signed", math.MinInt64)
		object.Uint64("unsigned", math.MaxUint64)
		object.Bool("flag", true)
		destination, err = object.End()
		if err != nil {
			t.Fatalf("End() error = %v, want nil", err)
		}
	})
	if gotAllocations != 0 || !bytes.Equal(destination, want) {
		t.Fatalf("reused scalars = (%g allocs, %q), want (0, %q)", gotAllocations, destination, want)
	}
}

func TestCanonicalObjectDestinationPrefixConsumesTheDocumentBudget(t *testing.T) {
	t.Parallel()
	member, err := core.MarshalCanonicalJSONDocument(struct {
		V bool `json:"v"`
	}{V: true})
	if err != nil {
		t.Fatalf("prefix fixture encoding error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		extent  int
		wantErr error
	}{
		{name: "prefix leaves one byte of slack", extent: attest.CanonicalBodyMaximumBytes - 1},
		{name: "prefix and member exactly consume document budget", extent: attest.CanonicalBodyMaximumBytes},
		{name: "prefix leaves no room for closing byte", extent: attest.CanonicalBodyMaximumBytes + 1, wantErr: core.ErrAttestContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prefix := bytes.Repeat([]byte{' '}, tc.extent-len(member))
			want := append(bytes.Clone(prefix), member...)
			object := attest.BeginCanonicalObject(prefix)
			object.Bool("v", true)
			got, err := object.End()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("prefixed End() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != nil {
					t.Fatalf("rejected prefixed object = %d bytes, want nil", len(got))
				}
				return
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("prefixed object = %d bytes, want %d exact prefix and member bytes", len(got), len(want))
			}
		})
	}
}
