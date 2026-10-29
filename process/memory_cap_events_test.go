package process

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestMemoryCapObservationValidationAndRoundTrip(t *testing.T) {
	t.Parallel()
	limit, _ := core.NewByteLength(256 << 20)
	zero, _ := core.NewByteLength(0)
	duration, _ := temporal.DurationFromNanoseconds(0)
	for _, row := range []struct {
		name  string
		fact  *MemoryCapObservation
		valid bool
	}{
		{name: "uncapped", valid: true},
		{name: "unknown", fact: &MemoryCapObservation{Limit: limit}, valid: true},
		{name: "observed-zero-is-only-a-snapshot", fact: &MemoryCapObservation{Limit: limit, State: MemoryCapObserved}, valid: true},
		{name: "local-positive-with-successful-parent", fact: &MemoryCapObservation{Limit: limit, State: MemoryCapObserved, LocalOOMEvents: 1, LocalOOMKills: 1}, valid: true},
		{name: "kill-alone-is-not-local-limit", fact: &MemoryCapObservation{Limit: limit, State: MemoryCapObserved, LocalOOMKills: 1}, valid: true},
		{name: "unknown-counters", fact: &MemoryCapObservation{Limit: limit, LocalOOMEvents: 1}},
		{name: "unknown-kills", fact: &MemoryCapObservation{Limit: limit, LocalOOMKills: 1}},
		{name: "no-limit", fact: &MemoryCapObservation{Limit: zero, State: MemoryCapObserved}},
		{name: "foreign-state", fact: &MemoryCapObservation{Limit: limit, State: 255}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			value := ResultObservation{CPUTime: duration, StdinBytes: zero, StdoutBytes: zero, StderrBytes: zero, MemoryCap: row.fact}
			err := value.Validate()
			if !row.valid {
				if !errors.Is(err, core.ErrProcessContract) {
					t.Fatalf("contradiction not owned: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, err := core.MarshalCanonicalJSONDocument(value)
			if err != nil {
				t.Fatal(err)
			}
			got, err := core.DecodeStrictJSON[ResultObservation](bytes.NewReader(raw), core.DefaultStrictJSONLimits())
			if err != nil {
				t.Fatal(err)
			}
			if (got.MemoryCap == nil) != (value.MemoryCap == nil) || got.MemoryCap != nil && *got.MemoryCap != *value.MemoryCap {
				t.Fatalf("lost exact cap snapshot: %+v", got)
			}
		})
	}
}

// Independent whole-record reference belongs only to the fuzz proof. Product
// parsing streams once in fixed memory even across arbitrarily long new keys.
func referenceMemoryEvents(raw string) (memoryEvents, bool) {
	var result memoryEvents
	if !strings.HasSuffix(raw, "\n") {
		return result, false
	}
	oom, kills := false, false
	for line := range strings.SplitSeq(strings.TrimSuffix(raw, "\n"), "\n") {
		key, value, ok := strings.Cut(line, " ")
		if !ok || key == "" || value == "" {
			return memoryEvents{}, false
		}
		for _, c := range key {
			if (c < 'a' || c > 'z') && c != '_' {
				return memoryEvents{}, false
			}
		}
		for _, c := range value {
			if c < '0' || c > '9' {
				return memoryEvents{}, false
			}
		}
		count, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return memoryEvents{}, false
		}
		switch key {
		case "oom":
			if oom {
				return memoryEvents{}, false
			}
			oom = true
			result.oom = count
		case "oom_kill":
			if kills {
				return memoryEvents{}, false
			}
			kills = true
			result.kills = count
		}
	}
	return result, oom && kills
}
func FuzzMemoryCapLocalEvents(f *testing.F) {
	for _, raw := range []string{"oom 0\noom_kill 0\n", "low 0\nhigh 1\nmax 4\noom 2\noom_kill 1\noom_group_kill 0\n", "oom 18446744073709551615\noom_kill 1\n", "oom 1\noom 2\noom_kill 1\n", "oom 1\noom_kill 2", ""} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		want, wantOK := referenceMemoryEvents(raw)
		got, ok := readMemoryEvents(strings.NewReader(raw))
		if ok != wantOK || ok && got != want {
			t.Fatalf("native parser %+v/%v reference %+v/%v", got, ok, want, wantOK)
		}
		// One-byte fragmented transport is the same actual decoder, no mirrored
		// production algorithm and no dependency on bufio's refill boundaries.
		fragmented, fragmentOK := readMemoryEvents(&memoryEventByteReader{source: strings.NewReader(raw)})
		if fragmentOK != ok || fragmentOK && fragmented != got {
			t.Fatalf("fragmentation changed facts")
		}
	})
}

type memoryEventByteReader struct{ source io.Reader }

func (r *memoryEventByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.source.Read(p)
}
