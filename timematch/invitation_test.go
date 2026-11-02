package timematch

import (
	"bytes"
	"errors"
	"go/ast"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func invitationFixture(t testing.TB) Invitation {
	t.Helper()
	event := intent(t, "primary", "Website review", "2026-11-02T15:00:00Z", "2026-11-02T15:30:00Z")
	organizer, err := core.ParseOAuthEmail("host@example.test")
	if err != nil {
		t.Fatalf("organizer error = %v, want nil", err)
	}
	attendee, err := core.ParseOAuthEmail("guest@example.test")
	if err != nil {
		t.Fatalf("attendee error = %v, want nil", err)
	}
	return Invitation{UID: event.ID, Summary: event.Summary, Interval: event.Interval, Stamp: at(t, "2026-11-01T19:00:00.999999999Z"), Organizer: organizer, Attendees: []core.OAuthEmail{attendee}, Method: InvitationRequest}
}

func TestSecretCopyBytesCustody(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"!", "~", "access-token", "refresh-token_123", strings.Repeat("A", 32768)} {
		t.Run(strconv.Itoa(len(raw))+"_"+raw[:1], func(t *testing.T) {
			t.Parallel()
			input := []byte(raw)
			value, err := ParseSecret(input)
			if err != nil {
				t.Fatalf("ParseSecret() error = %v, want nil", err)
			}
			defer func() {
				if err := value.Close(); err != nil {
					t.Errorf("Close() error = %v, want nil", err)
				}
			}()
			clear(input)
			first, err := value.CopyBytes()
			if err != nil || string(first) != raw {
				t.Fatalf("CopyBytes() exact = %t, error = %v, want true,nil", string(first) == raw, err)
			}
			defer clear(first)
			first[0] = '?'
			second, err := value.CopyBytes()
			if err != nil || string(second) != raw {
				t.Fatalf("CopyBytes() after foreign mutation exact = %t, error = %v, want true,nil", string(second) == raw, err)
			}
			defer clear(second)
			if err := value.Close(); err != nil {
				t.Fatalf("Close() error = %v, want nil", err)
			}
			if string(second) != raw {
				t.Fatalf("owned copy equal after source close=%t,want true independent custody", string(second) == raw)
			}
			refused, err := value.CopyBytes()
			if refused != nil || !errors.Is(err, core.ErrTimeMatchContract) {
				t.Fatalf("closed CopyBytes() = (%d bytes,%v), want nil,contract", len(refused), err)
			}
		})
	}
	var zero Secret
	if got, err := zero.CopyBytes(); got != nil || !errors.Is(err, core.ErrTimeMatchContract) {
		t.Fatalf("zero CopyBytes() = (%d bytes,%v), want nil,contract", len(got), err)
	}
}

func TestInvitationAdmissionAndNoPartialRefusal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(*Invitation)
		valid  bool
	}{
		{name: "request initial", valid: true},
		{name: "request first update", change: func(i *Invitation) { i.Sequence = 1 }, valid: true},
		{name: "request maximum sequence", change: func(i *Invitation) { i.Sequence = math.MaxInt32 }, valid: true},
		{name: "cancel first update", change: func(i *Invitation) { i.Method = InvitationCancel; i.Sequence = 1 }, valid: true},
		{name: "cancel maximum sequence", change: func(i *Invitation) { i.Method = InvitationCancel; i.Sequence = math.MaxInt32 }, valid: true},
		{name: "stamp epoch", change: func(i *Invitation) { i.Stamp = temporal.InstantFromNanoseconds(0) }, valid: true},
		{name: "stamp before epoch", change: func(i *Invitation) { i.Stamp = temporal.InstantFromNanoseconds(-1) }, valid: true},
		{name: "stamp minimum instant", change: func(i *Invitation) { i.Stamp = temporal.InstantFromNanoseconds(math.MinInt64) }, valid: true},
		{name: "stamp maximum instant", change: func(i *Invitation) { i.Stamp = temporal.InstantFromNanoseconds(math.MaxInt64) }, valid: true},
		{name: "multiple recipients", change: func(i *Invitation) { i.Attendees = append(i.Attendees, i.Organizer) }, valid: true},
		{name: "nil attendees", change: func(i *Invitation) { i.Attendees = nil }},
		{name: "empty attendees", change: func(i *Invitation) { i.Attendees = []core.OAuthEmail{} }},
		{name: "invalid attendee first", change: func(i *Invitation) { i.Attendees = []core.OAuthEmail{{}, i.Organizer} }},
		{name: "invalid attendee last prevents first write", change: func(i *Invitation) { i.Attendees = append(i.Attendees, core.OAuthEmail{}) }},
		{name: "organizer missing", change: func(i *Invitation) { i.Organizer = core.OAuthEmail{} }},
		{name: "uid missing", change: func(i *Invitation) { i.UID = EventID{} }},
		{name: "summary missing", change: func(i *Invitation) { i.Summary = EventSummary{} }},
		{name: "stamp missing", change: func(i *Invitation) { i.Stamp = temporal.Instant{} }},
		{name: "start missing", change: func(i *Invitation) { i.Interval.Start = temporal.Instant{} }},
		{name: "end missing", change: func(i *Invitation) { i.Interval.End = temporal.Instant{} }},
		{name: "equal bounds", change: func(i *Invitation) { i.Interval.End = i.Interval.Start }},
		{name: "reversed bounds", change: func(i *Invitation) { i.Interval.Start, i.Interval.End = i.Interval.End, i.Interval.Start }},
		{name: "start one ns after second", change: func(i *Invitation) { i.Interval.Start = temporal.InstantFromNanoseconds(1) }},
		{name: "end one ns before second", change: func(i *Invitation) { i.Interval.End = temporal.InstantFromNanoseconds(1793633399999999999) }},
		{name: "negative fractional start", change: func(i *Invitation) { i.Interval.Start = temporal.InstantFromNanoseconds(-1) }},
		{name: "sequence above RFC integer", change: func(i *Invitation) { i.Sequence = uint32(math.MaxInt32) + 1 }},
		{name: "sequence uint32 maximum", change: func(i *Invitation) { i.Sequence = math.MaxUint32 }},
		{name: "cancel zero sequence", change: func(i *Invitation) { i.Method = InvitationCancel }},
		{name: "method zero", change: func(i *Invitation) { i.Method = 0 }},
		{name: "method immediately above domain", change: func(i *Invitation) { i.Method = 3 }},
		{name: "method uint8 maximum", change: func(i *Invitation) { i.Method = 255 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			value := invitationFixture(t)
			if tc.change != nil {
				tc.change(&value)
			}
			var output bytes.Buffer
			err := WriteInvitation(&output, value)
			if tc.valid {
				if err != nil || output.Len() == 0 {
					t.Fatalf("WriteInvitation()=(%d bytes,%v), want message,nil", output.Len(), err)
				}
			} else if !errors.Is(err, core.ErrTimeMatchContract) || output.Len() != 0 {
				t.Fatalf("WriteInvitation()=(%d bytes,%v), want zero bytes,contract", output.Len(), err)
			}
		})
	}
}

func TestInvitationNativeWireConservation(t *testing.T) {
	t.Parallel()
	for _, summary := range []string{"Website review", `Slashes\commas,and;semicolons`, strings.Repeat("é", 38), strings.Repeat("💙", 80), strings.Repeat("a", 4096)} {
		t.Run(strconv.Itoa(len(summary)), func(t *testing.T) {
			t.Parallel()
			value := invitationFixture(t)
			var err error
			value.Summary, err = ParseEventSummary(summary)
			if err != nil {
				t.Fatalf("ParseEventSummary() error=%v,want nil", err)
			}
			value.Stamp = temporal.InstantFromNanoseconds(-1)
			address := "guest+tag?subject=oops#fragment%value@example.test"
			email, err := core.ParseOAuthEmail(address)
			if err != nil {
				t.Fatalf("ParseOAuthEmail() error=%v,want nil", err)
			}
			value.Attendees = []core.OAuthEmail{email}
			var wire bytes.Buffer
			if err := WriteInvitation(&wire, value); err != nil {
				t.Fatalf("WriteInvitation() error=%v,want nil", err)
			}
			for _, line := range strings.Split(wire.String(), "\r\n") {
				if len(line) > 75 || !utf8.ValidString(line) {
					t.Fatalf("physical line=(%d bytes,utf8 %t),want <=75,true", len(line), utf8.ValidString(line))
				}
			}
			unfolded := strings.ReplaceAll(wire.String(), "\r\n ", "")
			wantSummary := strings.NewReplacer(`\`, `\\`, ",", `\,`, ";", `\;`).Replace(summary)
			for _, want := range []string{"METHOD:REQUEST\r\n", "DTSTAMP:19691231T235959Z\r\n", "SUMMARY:" + wantSummary + "\r\n", "ATTENDEE:mailto:guest%2Btag%3Fsubject%3Doops%23fragment%25value@example.test\r\n", "DTSTART:20261102T150000Z\r\n", "DTEND:20261102T153000Z\r\n", "STATUS:CONFIRMED\r\n"} {
				if !strings.Contains(unfolded, want) {
					t.Fatalf("invitation lacks %q, want conserved property", want)
				}
			}
			value.Method = InvitationCancel
			value.Sequence = 1
			var cancel bytes.Buffer
			if err := WriteInvitation(&cancel, value); err != nil {
				t.Fatalf("cancel WriteInvitation() error=%v,want nil", err)
			}
			text := strings.ReplaceAll(cancel.String(), "\r\n ", "")
			if !strings.Contains(text, "METHOD:CANCEL\r\n") || !strings.Contains(text, "STATUS:CANCELLED\r\n") || !strings.Contains(text, "UID:"+value.UID.value+"\r\n") || !strings.Contains(text, "SEQUENCE:1\r\n") {
				t.Fatalf("cancellation message identity/sequence preserved=%t/%t,want true/true with cancel method and status", strings.Contains(text, "UID:"+value.UID.value+"\r\n"), strings.Contains(text, "SEQUENCE:1\r\n"))
			}
		})
	}
}

type invitationCutWriter struct {
	remaining         int
	output            bytes.Buffer
	cause             error
	callsAfterFailure int
	failed            bool
}

func (w *invitationCutWriter) Write(data []byte) (int, error) {
	if w.failed {
		w.callsAfterFailure++
		return 0, w.cause
	}
	n := min(len(data), w.remaining)
	if _, err := w.output.Write(data[:n]); err != nil {
		return 0, err
	}
	w.remaining -= n
	if n < len(data) {
		w.failed = true
		return n, w.cause
	}
	return n, nil
}

func TestInvitationEveryPartialWriterFailure(t *testing.T) {
	t.Parallel()
	value := invitationFixture(t)
	var full bytes.Buffer
	if err := WriteInvitation(&full, value); err != nil {
		t.Fatalf("full WriteInvitation() error=%v,want nil", err)
	}
	cause := errors.New("owned invitation sink failed")
	for limit := 0; limit < full.Len(); limit++ {
		for _, writerErr := range []error{cause, nil} {
			w := &invitationCutWriter{remaining: limit, cause: writerErr}
			err := WriteInvitation(w, value)
			want := writerErr
			if want == nil {
				want = io.ErrShortWrite
			}
			if !errors.Is(err, want) || !bytes.Equal(w.output.Bytes(), full.Bytes()[:limit]) || w.callsAfterFailure != 0 {
				t.Fatalf("cut %d err=%v,callsAfterFailure=%d, want %v,0 and conserved prefix", limit, err, w.callsAfterFailure, want)
			}
		}
	}
	if err := WriteInvitation(nil, value); !errors.Is(err, core.ErrTimeMatchContract) {
		t.Fatalf("nil writer error=%v,want contract", err)
	}
}

func TestInvitationMethodExhaustive(t *testing.T) {
	t.Parallel()
	for n := range 256 {
		err := InvitationMethod(n).Validate()
		want := n == 1 || n == 2
		if (err == nil) != want || !want && !errors.Is(err, core.ErrTimeMatchContract) {
			t.Fatalf("method%d error=%v,want valid=%t", n, err, want)
		}
	}
}

func TestTimeMatchTimeExecutionStaysInTemporal(t *testing.T) {
	t.Parallel()
	for name, file := range productionFiles(t) {
		for _, spec := range file.Imports {
			if spec.Path.Value == `"time"` {
				t.Fatalf("%s imports native time, want Temporal execution", name)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if selector.Sel.Name == "Time" || selector.Sel.Name == "UTC" || selector.Sel.Name == "UnixNano" {
				t.Errorf("%s borrows native time through %s, want Temporal projection", name, selector.Sel.Name)
			}
			return true
		})
	}
}

func TestInvitationTemporalSecondBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		ns             int64
		floor, ceiling string
		overflow       bool
	}{
		{name: "one before epoch", ns: -1, floor: "19691231T235959Z", ceiling: "19700101T000000Z"},
		{name: "epoch", ns: 0, floor: "19700101T000000Z", ceiling: "19700101T000000Z"},
		{name: "one after epoch", ns: 1, floor: "19700101T000000Z", ceiling: "19700101T000001Z"},
		{name: "negative second exact", ns: -1000000000, floor: "19691231T235959Z", ceiling: "19691231T235959Z"},
		{name: "negative second minus one", ns: -1000000001, floor: "19691231T235958Z", ceiling: "19691231T235959Z"},
		{name: "positive second minus one", ns: 999999999, floor: "19700101T000000Z", ceiling: "19700101T000001Z"},
		{name: "minimum representable instant", ns: math.MinInt64, floor: "16770921T001243Z", ceiling: "16770921T001244Z"},
		{name: "maximum representable instant", ns: math.MaxInt64, floor: "22620411T234716Z", overflow: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			instant := temporal.InstantFromNanoseconds(tc.ns)
			floor, err := icsStampTime(instant)
			if err != nil || floor != tc.floor {
				t.Fatalf("icsStampTime()=(%q,%v),want %q,nil", floor, err, tc.floor)
			}
			ceiling, err := icsCeilingTime(instant)
			if tc.overflow {
				if ceiling != "" || !errors.Is(err, core.ErrTemporalOverflow) {
					t.Fatalf("icsCeilingTime()=(%q,%v),want empty,overflow", ceiling, err)
				}
				return
			}
			if err != nil || ceiling != tc.ceiling {
				t.Fatalf("icsCeilingTime()=(%q,%v),want %q,nil", ceiling, err, tc.ceiling)
			}
		})
	}
}

func FuzzInvitationSummaryWireConservation(f *testing.F) {
	seed := invitationFixture(f)
	f.Add(seed.Summary.String())
	for _, text := range []string{"Review", `One\two;three,four`, strings.Repeat("💙", 50), "bad\nheader", "\xff"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		value := invitationFixture(t)
		summary, err := ParseEventSummary(text)
		if err != nil {
			if !errors.Is(err, core.ErrTimeMatchContract) {
				t.Fatalf("summary refusal=%v,want contract", err)
			}
			return
		}
		value.Summary = summary
		var output, again bytes.Buffer
		if err := WriteInvitation(&output, value); err != nil {
			t.Fatalf("WriteInvitation accepted summary error=%v,want nil", err)
		}
		if err := WriteInvitation(&again, value); err != nil || !slices.Equal(output.Bytes(), again.Bytes()) {
			t.Fatalf("repeat WriteInvitation error=%v,want nil and canonical bytes", err)
		}
		for _, line := range strings.Split(output.String(), "\r\n") {
			if len(line) > 75 || !utf8.ValidString(line) {
				t.Fatalf("folded line bytes=%d,validUTF8=%t,want <=75,true", len(line), utf8.ValidString(line))
			}
		}
		unfolded := strings.ReplaceAll(output.String(), "\r\n ", "")
		wireText := strings.SplitN(strings.SplitN(unfolded, "SUMMARY:", 2)[1], "\r\n", 2)[0]
		wantedWire := strings.NewReplacer(`\`, `\\`, ",", `\,`, ";", `\;`).Replace(text)
		if wireText != wantedWire {
			t.Fatalf("encoded summary=%q,want exact escaped %q", wireText, wantedWire)
		}
		if got := unescapeICSText(wireText); got != text {
			t.Fatalf("decoded summary=%q,want %q", got, text)
		}
	})
}
