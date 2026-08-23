package hostfacts

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoOOMBannerClassifierHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		data      string
		length    uint64
		chunk     int
		wantState GoOOMBannerState
	}{
		{name: "empty declared extent is absent", data: "", length: 0, chunk: 1, wantState: GoOOMBannerAbsent},
		{name: "unrelated one byte is absent", data: "x", length: 1, chunk: 1, wantState: GoOOMBannerAbsent},
		{name: "plain banner is present", data: GoOOMPlainBanner, length: uint64(len(GoOOMPlainBanner)), chunk: len(GoOOMPlainBanner), wantState: GoOOMBannerPresent},
		{name: "prefixed banner is present", data: GoOOMPrefixedBanner, length: uint64(len(GoOOMPrefixedBanner)), chunk: len(GoOOMPrefixedBanner), wantState: GoOOMBannerPresent},
		{name: "plain banner split every byte is present", data: GoOOMPlainBanner, length: uint64(len(GoOOMPlainBanner)), chunk: 1, wantState: GoOOMBannerPresent},
		{name: "prefixed banner split every byte is present", data: GoOOMPrefixedBanner, length: uint64(len(GoOOMPrefixedBanner)), chunk: 1, wantState: GoOOMBannerPresent},
		{name: "plain banner after prefix noise is present", data: "noise\n" + GoOOMPlainBanner, length: uint64(len("noise\n" + GoOOMPlainBanner)), chunk: 3, wantState: GoOOMBannerPresent},
		{name: "prefixed banner before suffix noise is present", data: GoOOMPrefixedBanner + "\nnoise", length: uint64(len(GoOOMPrefixedBanner + "\nnoise")), chunk: 5, wantState: GoOOMBannerPresent},
		{name: "both banners remain present", data: GoOOMPlainBanner + GoOOMPrefixedBanner, length: uint64(len(GoOOMPlainBanner + GoOOMPrefixedBanner)), chunk: 7, wantState: GoOOMBannerPresent},
		{name: "banner at maximum extent tail is present", data: strings.Repeat("x", GoOOMMaximumEvidenceBytes-len(GoOOMPlainBanner)) + GoOOMPlainBanner, length: GoOOMMaximumEvidenceBytes, chunk: 4093, wantState: GoOOMBannerPresent},
		{name: "one byte short plain banner is absent", data: GoOOMPlainBanner[:len(GoOOMPlainBanner)-1], length: uint64(len(GoOOMPlainBanner) - 1), chunk: 2, wantState: GoOOMBannerAbsent},
		{name: "one byte short prefixed banner is absent", data: GoOOMPrefixedBanner[:len(GoOOMPrefixedBanner)-1], length: uint64(len(GoOOMPrefixedBanner) - 1), chunk: 2, wantState: GoOOMBannerAbsent},
		{name: "wrong capitalization is absent", data: "Fatal error: out of memory", length: uint64(len("Fatal error: out of memory")), chunk: 4, wantState: GoOOMBannerAbsent},
		{name: "embedded NUL near miss is absent", data: "fatal error:\x00 out of memory", length: uint64(len("fatal error:\x00 out of memory")), chunk: 4, wantState: GoOOMBannerAbsent},
		{name: "declared extent hides trailing banner", data: "prefix" + GoOOMPlainBanner, length: uint64(len("prefix")), chunk: 2, wantState: GoOOMBannerAbsent},
		{name: "short source fails", data: "short", length: 6, chunk: 2, wantErr: io.ErrUnexpectedEOF},
		{name: "empty source with positive extent fails", data: "", length: 1, chunk: 1, wantErr: io.ErrUnexpectedEOF},
		{name: "declared maximum with short source fails", data: "x", length: GoOOMMaximumEvidenceBytes, chunk: 1, wantErr: io.ErrUnexpectedEOF},
		{name: "overlapping fatal prefix recovers", data: "fatal fatal error: out of memory", length: uint64(len("fatal fatal error: out of memory")), chunk: 2, wantState: GoOOMBannerPresent},
		{name: "repeated banner prefix near miss is absent", data: strings.Repeat("fatal error: out of memorx", 20), length: uint64(len(strings.Repeat("fatal error: out of memorx", 20))), chunk: 3, wantState: GoOOMBannerAbsent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := &chunkReader{data: []byte(tc.data), maximum: tc.chunk}
			got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
				Source: source,
				Length: mustByteLength(t, tc.length),
			})
			if tc.wantErr != nil {
				if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ClassifyGoOOMBanner() = (%v, %v), want (zero, %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ClassifyGoOOMBanner() error = %v, want nil", gotErr)
			}
			if got.State() != tc.wantState || got.BytesExamined().Uint64() != tc.length {
				t.Fatalf(
					"ClassifyGoOOMBanner() = state %v bytes %d, want state %v bytes %d",
					got.State(),
					got.BytesExamined().Uint64(),
					tc.wantState,
					tc.length,
				)
			}
			if got.Validate() != nil {
				t.Fatalf("ClassifyGoOOMBanner().Validate() error = %v, want nil", got.Validate())
			}
		})
	}
}

func TestGoOOMBannerEverySplitPositionPreservesPresence(t *testing.T) {
	t.Parallel()

	for _, banner := range []string{GoOOMPlainBanner, GoOOMPrefixedBanner} {
		for split := 1; split < len(banner); split++ {
			reader := io.MultiReader(
				bytes.NewReader([]byte(banner[:split])),
				bytes.NewReader([]byte(banner[split:])),
			)
			got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
				Source: reader,
				Length: mustByteLength(t, uint64(len(banner))),
			})
			if gotErr != nil || got.State() != GoOOMBannerPresent {
				t.Fatalf("ClassifyGoOOMBanner(split %d of %d) = (%v, %v), want present", split, len(banner), got, gotErr)
			}
		}
	}
}

func TestGoOOMBannerRequestAndReaderFailureBoundaries(t *testing.T) {
	t.Parallel()

	overMaximum := GoOOMBannerRequest{
		Source: bytes.NewReader(nil),
		Length: mustByteLength(t, GoOOMMaximumEvidenceBytes+1),
	}
	if gotErr := overMaximum.Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("GoOOMBannerRequest(over maximum).Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}
	if gotErr := (GoOOMBannerRequest{}).Validate(); !errors.Is(gotErr, core.ErrHostFactsContract) {
		t.Fatalf("GoOOMBannerRequest{}.Validate() error = %v, want %v", gotErr, core.ErrHostFactsContract)
	}

	native := errors.New("source failed")
	got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: errorReader{err: native},
		Length: mustByteLength(t, 1),
	})
	if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, core.ErrHostFactsObservation) || !errors.Is(gotErr, native) {
		t.Fatalf("ClassifyGoOOMBanner(native failure) = (%v, %v), want zero and both observation/native identities", got, gotErr)
	}

	got, gotErr = ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: zeroReader{},
		Length: mustByteLength(t, 1),
	})
	if got != (GoOOMBannerEvidence{}) || !errors.Is(gotErr, io.ErrNoProgress) {
		t.Fatalf("ClassifyGoOOMBanner(no progress) = (%v, %v), want (zero, %v)", got, gotErr, io.ErrNoProgress)
	}

	got, gotErr = ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
		Source: invalidCountReader{},
		Length: mustByteLength(t, 1),
	})
	var failure Failure
	if got != (GoOOMBannerEvidence{}) ||
		!errors.As(gotErr, &failure) ||
		failure.Operation != OperationGoOOMBanner {
		t.Fatalf("ClassifyGoOOMBanner(invalid count) = (%v, %v), want one typed banner-operation failure", got, gotErr)
	}
	if _, ok := errors.AsType[Failure](failure.Cause); ok {
		t.Fatalf("ClassifyGoOOMBanner(invalid count) cause = %v, want leaf cause without nested Failure", failure.Cause)
	}
}

func TestGoOOMBannerEvidenceJSONHostileTable(t *testing.T) {
	t.Parallel()

	valid := GoOOMBannerEvidence{
		examined: mustByteLength(t, GoOOMMaximumEvidenceBytes),
		state:    GoOOMBannerPresent,
	}
	wire, err := json.Marshal(valid)
	if err != nil {
		t.Fatalf("json.Marshal(valid evidence) error = %v, want nil", err)
	}
	var roundTrip GoOOMBannerEvidence
	if err := json.Unmarshal(wire, &roundTrip); err != nil || roundTrip != valid {
		t.Fatalf("json.Unmarshal(valid evidence) = (%v, %v), want (%v, nil)", roundTrip, err, valid)
	}

	rejected := []string{
		"",
		"null",
		"{}",
		`{"bytes_examined":0}`,
		`{"state":"absent"}`,
		`{"bytes_examined":0,"state":null}`,
		`{"bytes_examined":null,"state":"absent"}`,
		`{"bytes_examined":0,"state":"unknown"}`,
		`{"bytes_examined":0,"state":"present","extra":0}`,
		`{"state":"present","bytes_examined":0}`,
		`{"bytes_examined":00,"state":"absent"}`,
		`{"bytes_examined":-1,"state":"absent"}`,
		`{"bytes_examined":1.0,"state":"absent"}`,
		`{"bytes_examined":"1","state":"absent"}`,
		`{"bytes_examined":1048577,"state":"present"}`,
		"{\"bytes_examined\":0,\"state\":\"absent\"}\n",
		`{"bytes_examined":0,"state":"absent","state":"present"}`,
		`{"bytes_examined":0,"bytes_examined":1,"state":"absent"}`,
		`{"bytes_examined":18446744073709551615,"state":"absent"}`,
		`[]`,
	}
	for _, candidate := range rejected {
		before := valid
		gotErr := before.UnmarshalJSON([]byte(candidate))
		if !errors.Is(gotErr, core.ErrJSONContract) || before != valid {
			t.Fatalf("GoOOMBannerEvidence.UnmarshalJSON(%q) = (%v, %v), want unchanged %v and %v", candidate, before, gotErr, valid, core.ErrJSONContract)
		}
	}
}

func TestGoOOMBannerStateJSONExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		state := GoOOMBannerState(raw)
		wire, gotErr := state.MarshalJSON()
		wantValid := state == GoOOMBannerAbsent || state == GoOOMBannerPresent
		if !wantValid {
			if wire != nil || !errors.Is(gotErr, core.ErrHostFactsEvidence) {
				t.Fatalf("GoOOMBannerState(%d).MarshalJSON() = (%s, %v), want (nil, %v)", raw, wire, gotErr, core.ErrHostFactsEvidence)
			}
			continue
		}
		if gotErr != nil {
			t.Fatalf("GoOOMBannerState(%d).MarshalJSON() error = %v, want nil", raw, gotErr)
		}
		var roundTrip GoOOMBannerState
		if gotErr := roundTrip.UnmarshalJSON(wire); gotErr != nil || roundTrip != state {
			t.Fatalf("GoOOMBannerState.UnmarshalJSON(%s) = (%v, %v), want (%v, nil)", wire, roundTrip, gotErr, state)
		}
	}

	before := GoOOMBannerPresent
	for _, wire := range []string{"", "null", `""`, `"unknown"`, `"Present"`, `"present" `, `"present\"\u0000"`} {
		gotErr := before.UnmarshalJSON([]byte(wire))
		if !errors.Is(gotErr, core.ErrJSONContract) || before != GoOOMBannerPresent {
			t.Fatalf("GoOOMBannerState.UnmarshalJSON(%q) = (%v, %v), want unchanged present and %v", wire, before, gotErr, core.ErrJSONContract)
		}
	}
}

func FuzzGoOOMBannerClassifier(f *testing.F) {
	f.Add([]byte(GoOOMPlainBanner), uint32(1))
	f.Add([]byte(GoOOMPrefixedBanner), uint32(7))
	f.Add([]byte("fatal error: out of memorx"), uint32(3))
	f.Add([]byte{}, uint32(1))

	f.Fuzz(func(t *testing.T, data []byte, chunk uint32) {
		if len(data) > GoOOMMaximumEvidenceBytes {
			return
		}
		maximum := int(chunk%uint32(goOOMBufferBytes)) + 1
		got, gotErr := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
			Source: &chunkReader{data: append([]byte(nil), data...), maximum: maximum},
			Length: mustByteLength(t, uint64(len(data))),
		})
		if gotErr != nil {
			t.Fatalf("ClassifyGoOOMBanner(%d bytes) error = %v, want nil", len(data), gotErr)
		}
		wantPresent := bytes.Contains(data, []byte(GoOOMPlainBanner)) ||
			bytes.Contains(data, []byte(GoOOMPrefixedBanner))
		if gotPresent := got.State() == GoOOMBannerPresent; gotPresent != wantPresent {
			t.Fatalf("ClassifyGoOOMBanner(%d bytes).State() = %v, want present %t", len(data), got.State(), wantPresent)
		}
		if got.BytesExamined().Uint64() != uint64(len(data)) || got.Validate() != nil {
			t.Fatalf("ClassifyGoOOMBanner(%d bytes) emitted invalid extent/state: %v", len(data), got)
		}
	})
}

func BenchmarkClassifyGoOOMBanner1KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkGoOOMBanner(b, 1<<10)
}

func BenchmarkClassifyGoOOMBanner1MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkGoOOMBanner(b, 1<<20)
}

func benchmarkGoOOMBanner(b *testing.B, size int) {
	b.Helper()
	data := bytes.Repeat([]byte{'x'}, size)
	length := mustByteLength(b, uint64(size))
	b.ResetTimer()

	for b.Loop() {
		_, err := ClassifyGoOOMBanner(context.Background(), GoOOMBannerRequest{
			Source: bytes.NewReader(data),
			Length: length,
		})
		if err != nil {
			b.Fatalf("ClassifyGoOOMBanner(%d bytes) error = %v", size, err)
		}
	}
}

type chunkReader struct {
	data    []byte
	maximum int
}

func (r *chunkReader) Read(destination []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	count := min(len(destination), len(r.data), r.maximum)
	copy(destination, r.data[:count])
	r.data = r.data[count:]
	return count, nil
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

type zeroReader struct{}

func (zeroReader) Read([]byte) (int, error) {
	return 0, nil
}

type invalidCountReader struct{}

func (invalidCountReader) Read(destination []byte) (int, error) {
	return len(destination) + 1, nil
}
