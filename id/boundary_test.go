package id

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/deliri/primitive/v2026/testserial"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func escapedIdentityJSON(text string) []byte {
	var result strings.Builder
	result.WriteByte('"')
	for _, c := range []byte(text) {
		result.WriteString("\\u00")
		var pair [2]byte
		hex.Encode(pair[:], []byte{c})
		result.Write(pair[:])
	}
	result.WriteByte('"')
	return []byte(result.String())
}

func TestIDJSONBoundedProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	u, ue := NewUUIDv7(identitySeedRequest(t))
	l, le := NewULID(identitySeedRequest(t))
	if ue != nil || le != nil {
		t.Fatal(errors.Join(ue, le))
	}
	for _, fixture := range []struct {
		name    string
		text    string
		maximum int
		check   func(testing.TB, []byte)
	}{
		{"uuid", u.String(), UUIDv7JSONMaximumBytes, func(t testing.TB, b []byte) { checkUUIDv7JSON(t, b, u) }},
		{"ulid", l.String(), ULIDJSONMaximumBytes, func(t testing.TB, b []byte) { checkULIDJSON(t, b, l) }},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			canonical, err := core.MarshalCanonicalJSONString(fixture.text)
			if err != nil {
				t.Fatal(err)
			}
			escaped := escapedIdentityJSON(fixture.text)
			for _, tc := range []struct {
				name string
				data []byte
			}{
				{"canonical", canonical},
				{"one below input ceiling", append(bytes.Repeat([]byte(" "), fixture.maximum-len(canonical)-1), canonical...)},
				{"exact input ceiling", append(bytes.Repeat([]byte(" "), fixture.maximum-len(canonical)), canonical...)},
				{"one above input ceiling", append(bytes.Repeat([]byte(" "), fixture.maximum-len(canonical)+1), canonical...)},
				{"all Unicode escapes at ceiling", escaped},
				{"escaped token plus one byte", append(bytes.Clone(escaped), ' ')},
				{"empty input", nil},
				{"neutral null", []byte("null")},
				{"array", []byte("[]")},
				{"object", []byte("{}")},
				{"number", []byte("1")},
				{"concatenated documents", append(bytes.Clone(canonical), canonical...)},
				{"trailing garbage", append(bytes.Clone(canonical), 'x')},
				{"unpaired surrogate", []byte("\"\\ud800\"")},
				{"invalid UTF8", []byte{'"', 0xff, '"'}},
			} {
				t.Run(tc.name, func(t *testing.T) { t.Parallel(); fixture.check(t, tc.data) })
			}
		})
	}
}

func TestIDRequestAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		nanoseconds int64
		valid       bool
	}{
		{"one nanosecond before epoch", -1, false},
		{"minimum signed observation", math.MinInt64, false},
		{"neutral epoch", 0, true},
		{"positive submillisecond", 1, true},
		{"last nanosecond before next stamp", int64(temporal.NanosecondsPerMillisecond) - 1, true},
		{"next stamp", int64(temporal.NanosecondsPerMillisecond), true},
		{"maximum observation", math.MaxInt64, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed := make([]byte, core.SecretMaterialMinimumBytes)
			seed[entropyBytes-1] = 1
			request := identityRequestForTest(t, tc.nanoseconds, seed)
			err := request.Validate()
			u, ue := NewUUIDv7(request)
			l, le := NewULID(request)
			if (err == nil) != tc.valid || (ue == nil) != tc.valid || (le == nil) != tc.valid {
				t.Fatalf("request/uuid/ulid = (%v,%v,%v), want admission=%t", err, ue, le, tc.valid)
			}
			if !tc.valid {
				if !errors.Is(err, core.ErrIDContract) || !errors.Is(ue, core.ErrIDContract) || !errors.Is(le, core.ErrIDContract) || u != (UUIDv7{}) || l != (ULID{}) {
					t.Fatalf("request refusal=(%v,%v,%v,%v,%v), want typed errors and zero identities", err, u, ue, l, le)
				}
				return
			}
			expected := uint64(tc.nanoseconds / int64(temporal.NanosecondsPerMillisecond))
			var actual uint64
			for _, b := range l.value[:timestampBytes] {
				actual = actual<<8 | uint64(b)
			}
			if actual != expected || !bytes.Equal(u.value[:timestampBytes], l.value[:timestampBytes]) {
				t.Fatalf("stamp=%d, want %d", actual, expected)
			}
		})
	}
}

func TestIDEveryTextPositionAttacksAllByteValues(t *testing.T) {
	t.Parallel()
	u, ue := NewUUIDv7(identitySeedRequest(t))
	l, le := NewULID(identitySeedRequest(t))
	if ue != nil || le != nil {
		t.Fatal(errors.Join(ue, le))
	}
	for _, tc := range []struct {
		name, text string
		check      func(testing.TB, string)
	}{
		{"uuid", u.String(), checkUUIDText},
		{"ulid", l.String(), checkULIDText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for position := range len(tc.text) {
				t.Run(strconv.Itoa(position), func(t *testing.T) {
					t.Parallel()
					value := []byte(tc.text)
					for b := range 256 {
						value[position] = byte(b)
						tc.check(t, string(value))
					}
				})
			}
		})
	}
}

func TestULIDEveryBitAndHalfBoundary(t *testing.T) {
	t.Parallel()
	for position := range identityBytes * 8 {
		t.Run(strconv.Itoa(position), func(t *testing.T) {
			t.Parallel()
			var raw [identityBytes]byte
			raw[position/8] = 1 << uint(position%8)
			checkRawULID(t, raw)
			for i := range raw {
				raw[i] = ^raw[i]
			}
			checkRawULID(t, raw)
		})
	}
}

func TestIDAppendersPreserveOwnedBufferBoundaries(t *testing.T) {
	t.Parallel()
	u, ue := NewUUIDv7(identitySeedRequest(t))
	l, le := NewULID(identitySeedRequest(t))
	if ue != nil || le != nil {
		t.Fatal(errors.Join(ue, le))
	}
	for _, value := range []struct {
		name, text string
		append     func([]byte) ([]byte, error)
		valid      bool
	}{
		{"uuid", u.String(), u.AppendText, true},
		{"ulid", l.String(), l.AppendText, true},
		{"unset uuid", "", (UUIDv7{}).AppendText, false},
		{"unset ulid", "", (ULID{}).AppendText, false},
	} {
		t.Run(value.name, func(t *testing.T) {
			t.Parallel()
			for _, extent := range []struct {
				name  string
				slack int
			}{
				{"one byte short", -1}, {"exact capacity", 0}, {"one byte spare", 1},
			} {
				t.Run(extent.name, func(t *testing.T) {
					t.Parallel()
					prefix := []byte("prefix")
					width := len(value.text)
					if width == 0 {
						width = uuidTextBytes
					}
					capacity := len(prefix) + width + extent.slack
					backing := bytes.Repeat([]byte{0xa5}, capacity+1)
					copy(backing, prefix)
					before := bytes.Clone(backing)
					destination := backing[:len(prefix):capacity]
					got, err := value.append(destination)
					if !value.valid {
						if got != nil || !errors.Is(err, core.ErrIDContract) || !bytes.Equal(backing, before) {
							t.Fatalf("unset append=(%q,%v), storage preserved=%t; want nil/ID error/true", got, err, bytes.Equal(backing, before))
						}
						return
					}
					if err != nil || string(got) != string(prefix)+value.text || !bytes.Equal(backing[:len(prefix)], prefix) || backing[capacity] != before[capacity] {
						t.Fatalf("append=%q,%v; prefix or guard damaged", got, err)
					}
					if cap(destination)-len(destination) >= len(value.text) && &got[0] != &backing[0] {
						t.Fatalf("append storage=%p, want caller storage=%p with %d spare bytes", &got[0], &backing[0], cap(destination)-len(destination))
					}
				})
			}
		})
	}
}

func TestIDUsesCorrectedWallAndPreservesCallerEntropy(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		wall  int64
		valid bool
	}{
		{"correction before epoch", -1, false},
		{"correction to epoch", 0, true},
		{"correction beyond carrier", math.MaxInt64, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := identitySeedRequest(t)
			original, err := request.Entropy.CopyBytes()
			if err != nil {
				t.Fatal(err)
			}
			defer clear(original)
			request.Observation, err = request.Observation.WithWall(temporal.InstantFromNanoseconds(tc.wall))
			if err != nil {
				t.Fatal(err)
			}
			u, ue := NewUUIDv7(request)
			l, le := NewULID(request)
			if (ue == nil) != tc.valid || (le == nil) != tc.valid {
				t.Fatalf("corrected wall = (%v,%v), want valid=%t", ue, le, tc.valid)
			}
			if tc.valid {
				var ms uint64
				for _, b := range l.value[:timestampBytes] {
					ms = ms<<8 | uint64(b)
				}
				if ms != uint64(tc.wall/int64(temporal.NanosecondsPerMillisecond)) || !bytes.Equal(u.value[:timestampBytes], l.value[:timestampBytes]) {
					t.Fatalf("corrected stamp=%d, want %d; UUID/ULID agree=%t", ms, uint64(tc.wall/int64(temporal.NanosecondsPerMillisecond)), bytes.Equal(u.value[:timestampBytes], l.value[:timestampBytes]))
				}
			} else if u != (UUIDv7{}) || l != (ULID{}) || !errors.Is(ue, core.ErrIDContract) || !errors.Is(le, core.ErrIDContract) {
				t.Fatalf("corrected-wall refusal=(%v,%v,%v,%v), want zero identities and ID errors", u, ue, l, le)
			}
			after, err := request.Entropy.CopyBytes()
			if err != nil {
				t.Fatal(err)
			}
			defer clear(after)
			if !bytes.Equal(original, after) {
				t.Fatalf("caller entropy preserved=%t, want true", bytes.Equal(original, after))
			}
		})
	}
}

func TestIdentityEntropyBitOwnership(t *testing.T) {
	t.Parallel()
	for bit := range core.SecretMaterialMinimumBytes * 8 {
		t.Run(strconv.Itoa(bit), func(t *testing.T) {
			t.Parallel()
			raw := bytes.Repeat([]byte{0xff}, core.SecretMaterialMinimumBytes)
			request := identityRequestForTest(t, 0, raw)
			u, ue := NewUUIDv7(request)
			l, le := NewULID(request)
			againU, aue := NewUUIDv7(request)
			againL, ale := NewULID(request)
			if ue != nil || le != nil || aue != nil || ale != nil || u != againU || l != againL {
				t.Fatalf("repeated UUID=(%v,%v), ULID=(%v,%v), errors=(%v,%v,%v,%v); want equal values and nil", u, againU, l, againL, ue, le, aue, ale)
			}
			changed := bytes.Clone(raw)
			changed[bit/8] ^= 1 << uint(bit%8)
			other := identityRequestForTest(t, 0, changed)
			gotU, gue := NewUUIDv7(other)
			gotL, gle := NewULID(other)
			if gue != nil || gle != nil {
				t.Fatal(errors.Join(gue, gle))
			}
			used := bit < entropyBytes*8
			uuidUsed := used && !(bit/8 == 0 && bit%8 >= 4) && !(bit/8 == 2 && bit%8 >= 6)
			if (gotU != u) != uuidUsed || (gotL != l) != used {
				t.Fatalf("bit %d changed UUID=%t ULID=%t, want %t/%t", bit, gotU != u, gotL != l, uuidUsed, used)
			}
			expected := l.value
			if used {
				expected[timestampBytes+bit/8] ^= 1 << uint(bit%8)
			}
			if gotL.value != expected {
				t.Fatalf("ULID after bit %d=%x, want %x", bit, gotL.value, expected)
			}
			expected[6] = expected[6]&0x0f | 0x70
			expected[8] = expected[8]&0x3f | 0x80
			if gotU.value != uuid.UUID(expected) {
				t.Fatalf("UUID after bit %d=%x, want %x", bit, gotU.value, expected)
			}
			if err := request.Entropy.Destroy(); err != nil {
				t.Fatal(err)
			}
			if u != againU || l != againL || u.Validate() != nil || l.Validate() != nil {
				t.Fatalf("post-destroy UUID=(%v,%v), ULID=(%v,%v); want retained valid values", u, u.Validate(), l, l.Validate())
			}
		})
	}
}

func TestIdentityValidationAndZeroProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	good, err := NewUUIDv7(identitySeedRequest(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name        string
		value       UUIDv7
		valid, zero bool
	}{
		{"valid marks", good, true, false},
		{"unset", UUIDv7{}, false, true},
		{"wrong version", UUIDv7{value: uuid.UUID{6: 0x60, 8: 0x80}}, false, false},
		{"wrong variant", UUIDv7{value: uuid.UUID{6: 0x70, 8: 0xc0}}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			validation := tc.value.Validate()
			wire, err := tc.value.MarshalJSON()
			if (validation == nil) != tc.valid || tc.value.IsValid() != tc.valid || tc.value.IsZero() != tc.zero {
				t.Fatalf("UUID validity=(%v,%t,%t), want valid=%t zero=%t", validation, tc.value.IsValid(), tc.value.IsZero(), tc.valid, tc.zero)
			}
			if !tc.valid && (tc.value.String() != "" || wire != nil || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrIDContract) || !errors.Is(validation, core.ErrIDContract)) {
				t.Fatalf("invalid UUID projections=(%q,%q,%v,%v), want empty text, nil JSON and typed refusal", tc.value.String(), wire, err, validation)
			}
			if tc.valid && (err != nil || len(wire) == 0) {
				t.Fatalf("valid UUID JSON=(%q,%v), want canonical output", wire, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		raw  [identityBytes]byte
	}{
		{"neutral unset", [identityBytes]byte{}},
		{"single lowest bit", [identityBytes]byte{identityBytes - 1: 1}},
		{"single timestamp bit", [identityBytes]byte{0: 0x80}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			checkRawULID(t, tc.raw)
			value := ULID{value: tc.raw}
			valid := tc.raw != ([identityBytes]byte{})
			raw, err := value.Bytes()
			wire, jsonErr := value.MarshalJSON()
			validation := value.Validate()
			if value.IsZero() == valid || value.IsValid() != valid || (validation == nil) != valid {
				t.Fatalf("ULID validity=(%v,%t,%t), want valid=%t", validation, value.IsValid(), value.IsZero(), valid)
			}
			if !valid && (raw != tc.raw || wire != nil || value.String() != "" || !errors.Is(err, core.ErrIDContract) || !errors.Is(jsonErr, core.ErrJSONContract) || !errors.Is(jsonErr, core.ErrIDContract) || !errors.Is(validation, core.ErrIDContract)) {
				t.Fatalf("unset ULID projections=(%x,%q,%q,%v,%v,%v), want zero/nil/empty and typed refusal", raw, wire, value.String(), err, jsonErr, validation)
			}
			if valid && (err != nil || raw != tc.raw || jsonErr != nil) {
				t.Fatalf("ULID projections=(%x,%v,%v), want %x and nil", raw, err, jsonErr, tc.raw)
			}
		})
	}
}

func TestIdentityNilJSONReceiversRefuseAllTokenClasses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil}, {"null", []byte("null")}, {"string", []byte("\"x\"")}, {"overfull", bytes.Repeat([]byte(" "), UUIDv7JSONMaximumBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var u *UUIDv7
			var l *ULID
			for _, err := range []error{u.UnmarshalJSON(tc.data), l.UnmarshalJSON(tc.data)} {
				if !errors.Is(err, core.ErrIDContract) || !errors.Is(err, core.ErrJSONContract) {
					t.Fatalf("nil receiver error=%v, want both identities", err)
				}
			}
		})
	}
}

func TestIdentityAllocationBudgets(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	u, ue := NewUUIDv7(identitySeedRequest(t))
	l, le := NewULID(identitySeedRequest(t))
	if ue != nil || le != nil {
		t.Fatal(errors.Join(ue, le))
	}
	uuidText := u.String()
	ulidText := l.String()
	uuidBuffer := make([]byte, 0, len(uuidText))
	ulidBuffer := make([]byte, 0, len(ulidText))
	for _, tc := range []struct {
		name string
		run  func() (bool, error)
	}{
		{"UUID parser", func() (bool, error) { got, err := ParseUUIDv7(uuidText); return got == u, err }},
		{"ULID parser", func() (bool, error) { got, err := ParseULID(ulidText); return got == l, err }},
		{"UUID append exact capacity", func() (bool, error) { got, err := u.AppendText(uuidBuffer[:0]); return string(got) == uuidText, err }},
		{"ULID append exact capacity", func() (bool, error) { got, err := l.AppendText(ulidBuffer[:0]); return string(got) == ulidText, err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			allocations := testing.AllocsPerRun(100, func() {
				ok, err := tc.run()
				if err != nil || !ok {
					t.Fatalf("measured work = (%t,%v), want exact output", ok, err)
				}
			})
			if allocations != 0 {
				t.Fatalf("allocations=%g, want zero", allocations)
			}
		})
	}
}

// Any scheduling order must return the exact identity or a typed zero refusal.
// Completion is joined; the test makes no claim about which goroutine wins.
func TestIdentityConstructionRacesOwnedDestruction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		nanos int64
	}{
		{"epoch", 0}, {"maximum observation", math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for range 32 {
				raw := make([]byte, core.SecretMaterialMinimumBytes)
				raw[entropyBytes-1] = 1
				request := identityRequestForTest(t, tc.nanos, raw)
				wantU, ue := NewUUIDv7(request)
				wantL, le := NewULID(request)
				if ue != nil || le != nil {
					t.Fatal(errors.Join(ue, le))
				}
				start := make(chan struct{})
				var group sync.WaitGroup
				var gotU UUIDv7
				var gotL ULID
				var uErr, lErr, destroyErr error
				group.Go(func() { <-start; gotU, uErr = NewUUIDv7(request) })
				group.Go(func() { <-start; gotL, lErr = NewULID(request) })
				group.Go(func() { <-start; destroyErr = request.Entropy.Destroy() })
				close(start)
				group.Wait()
				if destroyErr != nil {
					t.Fatal(destroyErr)
				}
				if uErr == nil {
					if gotU != wantU {
						t.Fatalf("concurrent UUID=%v, want %v", gotU, wantU)
					}
				} else if gotU != (UUIDv7{}) || !errors.Is(uErr, core.ErrIDContract) {
					t.Fatalf("concurrent UUID refusal=(%v,%v), want zero/ID error", gotU, uErr)
				}
				if lErr == nil {
					if gotL != wantL {
						t.Fatalf("concurrent ULID=%v, want %v", gotL, wantL)
					}
				} else if gotL != (ULID{}) || !errors.Is(lErr, core.ErrIDContract) {
					t.Fatalf("concurrent ULID refusal=(%v,%v), want zero/ID error", gotL, lErr)
				}
				afterU, auErr := NewUUIDv7(request)
				afterL, alErr := NewULID(request)
				if afterU != (UUIDv7{}) || afterL != (ULID{}) || !errors.Is(auErr, core.ErrIDContract) || !errors.Is(alErr, core.ErrIDContract) {
					t.Fatalf("post-destroy mint=(%v,%v,%v,%v), want zero identities and ID errors", afterU, auErr, afterL, alErr)
				}
			}
		})
	}
}
