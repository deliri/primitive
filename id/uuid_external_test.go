package id_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

// testEntropy returns sixteen fresh bytes whose first ten place one low bit
// at the entropy tail, so every pinned spelling stays hand checkable.
func testEntropy() []byte {
	return []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
}

func testRequest(t *testing.T, milliseconds int64, entropy []byte) id.Request {
	t.Helper()
	observation, err := temporal.NewObservation(time.UnixMilli(milliseconds))
	if err != nil {
		t.Fatalf("temporal.NewObservation(%d ms) error = %v, want nil", milliseconds, err)
	}
	material, err := core.NewSecretMaterial(entropy)
	if err != nil {
		t.Fatalf("core.NewSecretMaterial(%d bytes) error = %v, want nil", len(entropy), err)
	}
	return id.Request{Observation: observation, Entropy: material}
}

func requireIDContract(t *testing.T, operation string, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrIDContract) {
		t.Fatalf("%s error = %v, want errors.Is %v", operation, err, core.ErrIDContract)
	}
	if !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("%s error = %v, want the %v parent to survive wrapping", operation, err, core.ErrPrimitiveContract)
	}
}

func TestNewUUIDv7PinsExactCanonicalSpellings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		wantText     string
		milliseconds int64
	}{
		{name: "epoch itself is the smallest stamp", milliseconds: 0, wantText: "00000000-0000-7000-8000-000000000001"},
		{name: "one millisecond after the epoch", milliseconds: 1, wantText: "00000000-0001-7000-8000-000000000001"},
		{name: "one below the thirty-two bit stamp boundary", milliseconds: 4294967295, wantText: "0000ffff-ffff-7000-8000-000000000001"},
		{name: "exactly the thirty-two bit stamp boundary", milliseconds: 4294967296, wantText: "00010000-0000-7000-8000-000000000001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.NewUUIDv7(testRequest(t, tc.milliseconds, testEntropy()))
			if err != nil {
				t.Fatalf("NewUUIDv7(%d ms) error = %v, want nil", tc.milliseconds, err)
			}
			if got.String() != tc.wantText {
				t.Fatalf("NewUUIDv7(%d ms).String() = %q, want %q", tc.milliseconds, got.String(), tc.wantText)
			}
			parsed, err := id.ParseUUIDv7(tc.wantText)
			if err != nil || parsed != got {
				t.Fatalf("ParseUUIDv7(%q) = (%v, %v), want the constructed value back", tc.wantText, parsed, err)
			}
		})
	}
}

func TestNewUUIDv7SetsVersionAndVariantOverHostileEntropy(t *testing.T) {
	t.Parallel()

	entropy := []byte{0xff, 0x00, 0xff, 0x00, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	got, err := id.NewUUIDv7(testRequest(t, 1, entropy))
	if err != nil {
		t.Fatalf("NewUUIDv7(saturating mark bytes) error = %v, want nil", err)
	}
	want := "00000000-0001-7f00-bf00-000000000001"
	if got.String() != want {
		t.Fatalf("NewUUIDv7(saturating mark bytes).String() = %q, want the version and variant marks to win: %q", got.String(), want)
	}
}

func TestNewUUIDv7IsPureAndConsumesExactlyTenEntropyBytes(t *testing.T) {
	t.Parallel()

	first, err := id.NewUUIDv7(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewUUIDv7(first) error = %v, want nil", err)
	}
	second, err := id.NewUUIDv7(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewUUIDv7(second) error = %v, want nil", err)
	}
	if first != second {
		t.Fatalf("NewUUIDv7 minted %v then %v from one request, want pure construction", first, second)
	}
	tailDiffers := testEntropy()
	for index := 10; index < len(tailDiffers); index++ {
		tailDiffers[index] = 0x99
	}
	third, err := id.NewUUIDv7(testRequest(t, 1, tailDiffers))
	if err != nil {
		t.Fatalf("NewUUIDv7(differing tail) error = %v, want nil", err)
	}
	if third != first {
		t.Fatalf("NewUUIDv7(differing tail) = %v, want %v: only the first ten entropy bytes are consumed", third, first)
	}
}

func TestNewUUIDv7RefusesHostileRequests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup func(t *testing.T) id.Request
		name  string
	}{
		{
			name: "instant one millisecond before the epoch",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return testRequest(t, -1, testEntropy())
			},
		},
		{
			name: "zero request carries no observation",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return id.Request{}
			},
		},
		{
			name: "entropy one byte above the exact minimum extent",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return testRequest(t, 1, append(testEntropy(), 0x01))
			},
		},
		{
			name: "entropy at the material maximum extent",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				oversize := make([]byte, 64)
				oversize[63] = 1
				return testRequest(t, 1, oversize)
			},
		},
		{
			name: "entropy destroyed before construction",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				request := testRequest(t, 1, testEntropy())
				if err := request.Entropy.Destroy(); err != nil {
					t.Fatalf("Entropy.Destroy() error = %v, want nil", err)
				}
				return request
			},
		},
		{
			name: "zero entropy handle",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				request := testRequest(t, 1, testEntropy())
				request.Entropy = core.SecretMaterial{}
				return request
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.NewUUIDv7(tc.setup(t))
			requireIDContract(t, "NewUUIDv7(hostile request)", err)
			if !got.IsZero() {
				t.Fatalf("NewUUIDv7(hostile request) value = %v, want the zero value", got)
			}
		})
	}
}

func TestParseUUIDv7AdmitsOnlyCanonicalText(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		text string
	}{
		{name: "zero stamp with only the marks set", text: "00000000-0000-7000-8000-000000000000"},
		{name: "smallest nonzero entropy", text: "00000000-0000-7000-8000-000000000001"},
		{name: "one millisecond stamp", text: "00000000-0001-7000-8000-000000000001"},
		{name: "variant nibble at its nine spelling", text: "00000000-0000-7000-9000-000000000000"},
		{name: "variant nibble at its a spelling", text: "00000000-0000-7000-a000-000000000000"},
		{name: "variant nibble at its b ceiling", text: "00000000-0000-7000-b000-000000000000"},
		{name: "every hex digit in canonical case", text: "01234567-89ab-7cde-8f01-23456789abcd"},
		{name: "saturated stamp and entropy", text: "ffffffff-ffff-7fff-bfff-ffffffffffff"},
		{name: "entropy nibbles beside the version mark", text: "00000000-0000-7fff-8000-000000000000"},
		{name: "entropy nibbles beside the variant mark", text: "00000000-0000-7000-8fff-000000000000"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.ParseUUIDv7(tc.text)
			if err != nil {
				t.Fatalf("ParseUUIDv7(%q) error = %v, want nil", tc.text, err)
			}
			if got.String() != tc.text {
				t.Fatalf("ParseUUIDv7(%q).String() = %q, want the admitted text back", tc.text, got.String())
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("ParseUUIDv7(%q).Validate() error = %v, want nil", tc.text, err)
			}
		})
	}

	invalid := []struct {
		name string
		text string
	}{
		{name: "empty text", text: ""},
		{name: "one byte below the canonical extent", text: "00000000-0000-7000-8000-00000000000"},
		{name: "one byte above the canonical extent", text: "00000000-0000-7000-8000-0000000000011"},
		{name: "uppercase hex digit", text: "00000000-0000-7000-8000-00000000000A"},
		{name: "braced spelling", text: "{0000000-0000-7000-8000-000000000001}"},
		{name: "urn prefixed spelling truncated to extent", text: "urn:uuid:00000000-0000-7000-8000-000"},
		{name: "hyphenless spelling padded to extent", text: "000000000000700080000000000000010000"},
		{name: "first dash shifted right by one", text: "000000000-000-7000-8000-000000000001"},
		{name: "second dash shifted left by one", text: "00000000-000-07000-8000-000000000001"},
		{name: "third dash replaced by a digit", text: "00000000-0000-700008000-000000000001"},
		{name: "fourth dash replaced by a digit", text: "00000000-0000-7000-80000000000000001"},
		{name: "version nibble one below seven", text: "00000000-0000-6000-8000-000000000001"},
		{name: "version nibble one above seven", text: "00000000-0000-8000-8000-000000000001"},
		{name: "version four spelling", text: "00000000-0000-4000-8000-000000000001"},
		{name: "variant nibble one below its floor", text: "00000000-0000-7000-7000-000000000001"},
		{name: "variant nibble one above its ceiling", text: "00000000-0000-7000-c000-000000000001"},
		{name: "non hex letter inside a group", text: "00000000-0000-7000-8000-00000000000g"},
		{name: "space padded tail", text: "00000000-0000-7000-8000-00000000000 "},
		{name: "multibyte rune at the exact extent", text: "00000000-0000-7000-8000-0000000000é"},
		{name: "ulid spelling offered to the uuid door", text: "00000000010000000000000001"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.ParseUUIDv7(tc.text)
			requireIDContract(t, "ParseUUIDv7(hostile text)", err)
			if !got.IsZero() {
				t.Fatalf("ParseUUIDv7(%q) value = %v, want the zero value", tc.text, got)
			}
		})
	}
}

func TestUUIDv7ZeroValueNeverValidates(t *testing.T) {
	t.Parallel()

	var zero id.UUIDv7
	if !zero.IsZero() || zero.IsValid() {
		t.Fatalf("zero UUIDv7 IsZero() = %t IsValid() = %t, want true and false", zero.IsZero(), zero.IsValid())
	}
	requireIDContract(t, "zero UUIDv7 Validate()", zero.Validate())
	if zero.String() != "" {
		t.Fatalf("zero UUIDv7 String() = %q, want empty", zero.String())
	}
}

func TestUUIDv7StringOrderIsTimeOrder(t *testing.T) {
	t.Parallel()

	milliseconds := []int64{0, 1, 4294967295, 4294967296, time.Unix(0, math.MaxInt64).UnixMilli()}
	previous := ""
	for _, stamp := range milliseconds {
		value, err := id.NewUUIDv7(testRequest(t, stamp, testEntropy()))
		if err != nil {
			t.Fatalf("NewUUIDv7(%d ms) error = %v, want nil", stamp, err)
		}
		if got := value.String(); got <= previous {
			t.Fatalf("NewUUIDv7(%d ms).String() = %q, want lexicographically after %q", stamp, got, previous)
		} else {
			previous = got
		}
	}
}

func TestUUIDv7JSONRoundTripIsCanonical(t *testing.T) {
	t.Parallel()

	value, err := id.NewUUIDv7(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewUUIDv7(1 ms) error = %v, want nil", err)
	}
	gotJSON, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("UUIDv7.MarshalJSON() error = %v, want nil", err)
	}
	wantJSON := `"00000000-0001-7000-8000-000000000001"`
	if string(gotJSON) != wantJSON {
		t.Fatalf("UUIDv7.MarshalJSON() = %s, want %s", gotJSON, wantJSON)
	}
	var decoded id.UUIDv7
	if err := decoded.UnmarshalJSON(gotJSON); err != nil {
		t.Fatalf("UUIDv7.UnmarshalJSON(canonical) error = %v, want nil", err)
	}
	if decoded != value {
		t.Fatalf("UUIDv7 JSON round trip = %v, want %v", decoded, value)
	}
}

func TestUUIDv7JSONRefusesHostileTokensAndReceivers(t *testing.T) {
	t.Parallel()

	var zero id.UUIDv7
	if _, err := zero.MarshalJSON(); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("zero UUIDv7 MarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
	survivor, err := id.NewUUIDv7(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewUUIDv7(1 ms) error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data string
	}{
		{name: "numeric token", data: "123"},
		{name: "object token", data: "{}"},
		{name: "null token", data: "null"},
		{name: "uppercase spelling inside a string", data: `"00000000-0001-7000-8000-00000000000A"`},
		{name: "version four spelling inside a string", data: `"00000000-0001-4000-8000-000000000001"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decoded := survivor
			if err := decoded.UnmarshalJSON([]byte(tc.data)); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("UUIDv7.UnmarshalJSON(%s) error = %v, want errors.Is %v", tc.data, err, core.ErrJSONContract)
			}
			if decoded != survivor {
				t.Fatalf("UUIDv7.UnmarshalJSON(%s) mutated the receiver to %v, want %v unchanged", tc.data, decoded, survivor)
			}
		})
	}
	var absent *id.UUIDv7
	if err := absent.UnmarshalJSON([]byte(`"00000000-0001-7000-8000-000000000001"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil UUIDv7 receiver UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func TestUUIDv7AppendTextSpellsTheOneCanonicalForm(t *testing.T) {
	t.Parallel()

	value, err := id.NewUUIDv7(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewUUIDv7(1 ms) error = %v, want nil", err)
	}
	cases := []struct {
		name        string
		destination func() []byte
		wantPrefix  string
	}{
		{name: "nil destination receives exactly the spelling", destination: func() []byte { return nil }},
		{name: "empty destination with capacity receives exactly the spelling", destination: func() []byte { return make([]byte, 0, 64) }},
		{name: "existing prefix survives in front of the spelling", destination: func() []byte { return []byte(`"id":"`) }, wantPrefix: `"id":"`},
		{name: "full destination grows past its capacity", destination: func() []byte { return append(make([]byte, 0, 2), 'x', 'y') }, wantPrefix: "xy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := value.AppendText(tc.destination())
			if err != nil {
				t.Fatalf("UUIDv7.AppendText() error = %v, want nil", err)
			}
			want := tc.wantPrefix + value.String()
			if string(got) != want {
				t.Fatalf("UUIDv7.AppendText() = %q, want %q", got, want)
			}
			parsed, err := id.ParseUUIDv7(string(got[len(tc.wantPrefix):]))
			if err != nil || parsed != value {
				t.Fatalf("ParseUUIDv7(appended spelling) = (%v, %v), want (%v, nil)", parsed, err, value)
			}
		})
	}
}

func TestUUIDv7AppendTextIntoSufficientCapacityDoesNotAllocate(t *testing.T) {
	t.Parallel()

	value, err := id.NewUUIDv7(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewUUIDv7(1 ms) error = %v, want nil", err)
	}
	destination := make([]byte, 0, 64)
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			appended, appendErr := value.AppendText(destination[:0])
			if appendErr != nil || len(appended) == 0 {
				b.Fatalf("UUIDv7.AppendText() = (%d bytes, %v), want the spelling and nil", len(appended), appendErr)
			}
		}
	})
	if got := result.AllocsPerOp(); got != 0 {
		t.Fatalf("UUIDv7.AppendText() into sufficient capacity allocs/op = %d, want 0", got)
	}
}

func TestUUIDv7AppendTextRefusesTheUnsetValue(t *testing.T) {
	t.Parallel()

	var zero id.UUIDv7
	got, err := zero.AppendText([]byte("prefix"))
	requireIDContract(t, "zero UUIDv7 AppendText()", err)
	if got != nil {
		t.Fatalf("zero UUIDv7 AppendText() = %q, want nil", got)
	}
}
