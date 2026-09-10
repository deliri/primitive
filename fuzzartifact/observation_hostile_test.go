package fuzzartifact

import (
	"errors"
	"io"
	"io/fs"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// The accounting domain has five independent facts: successful callback,
// refused callback, unsupported regular entry, child directory, other mode.
// Exhausting their feasible combinations replaces invented classifier quotas.
func TestDirectoryProducerAccountingLayerTriad(t *testing.T) {
	t.Parallel()
	type primaryClass uint8
	const (
		neutral primaryClass = iota + 1
		refusal
		boundary
	)
	cases := []struct {
		name                                                     string
		mode                                                     fs.FileMode
		text                                                     string
		wantMatched, wantDirectories, wantOther, wantUnsupported uint64
	}{
		{name: "regular generated name is delivered", text: generatedNameForPosition(t, ArtifactCorpus, 1).String(), wantMatched: 1},
		{name: "foreign regular name is never delivered", text: "foreign", wantUnsupported: 1},
		{name: "generated directory is not a file", mode: fs.ModeDir, text: generatedNameForPosition(t, ArtifactCorpus, 2).String(), wantDirectories: 1},
		{name: "generated symlink is not followed", mode: fs.ModeSymlink, text: generatedNameForPosition(t, ArtifactCorpus, 3).String(), wantOther: 1},
		{name: "generated socket is not opened", mode: fs.ModeSocket, text: generatedNameForPosition(t, ArtifactCorpus, 4).String(), wantOther: 1},
		{name: "generated named pipe is not opened", mode: fs.ModeNamedPipe, text: generatedNameForPosition(t, ArtifactCorpus, 5).String(), wantOther: 1},
		{name: "generated block device is not opened", mode: fs.ModeDevice, text: generatedNameForPosition(t, ArtifactCorpus, 6).String(), wantOther: 1},
		{name: "generated character device is not opened", mode: fs.ModeDevice | fs.ModeCharDevice, text: generatedNameForPosition(t, ArtifactCorpus, 7).String(), wantOther: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls uint64
			current := newFinder(FindRequest{Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(name GeneratedName) error {
				calls++
				if name.String() != tc.text || name.Kind() != ArtifactCorpus || name.Validate() != nil {
					t.Fatalf("visitor=%+v, want admitted source name %q", name, tc.text)
				}
				return nil
			}})
			directive, err := current.visit(filestore.WalkEntry{Entry: modeEntry{name: tc.text, mode: tc.mode}})
			wantDirective := filestore.WalkContinue
			if tc.wantDirectories != 0 {
				wantDirective = filestore.WalkSkipDirectory
			}
			if err != nil || directive != wantDirective || calls != tc.wantMatched {
				t.Fatalf("visit directive/error/calls=%v/%v/%d, want %v/nil/%d", directive, err, calls, wantDirective, tc.wantMatched)
			}
			// Snapshot the producer before final classification; no invented evidence.
			facts := current.observation
			if facts.matched != tc.wantMatched || facts.delivered != calls || facts.ignoredDirectories != tc.wantDirectories || facts.nonRegular != tc.wantOther || facts.unsupportedRegular != tc.wantUnsupported {
				t.Fatalf("producer=%+v, want source-derived counters", facts)
			}
			got, err := current.finish()
			class := boundary
			wantState := ObservationComplete
			var wantErr error
			if tc.wantMatched == 0 {
				class = neutral
			}
			if tc.wantUnsupported != 0 {
				class = refusal
				wantState = ObservationUnsupportedFormat
				wantErr = core.ErrFuzzArtifactFormat
			}
			if got.State() != wantState || !errors.Is(err, wantErr) || got.Validate() != nil || got.matched != facts.matched || got.nonRegular != facts.nonRegular || got.ignoredDirectories != facts.ignoredDirectories || got.unsupportedRegular != facts.unsupportedRegular {
				t.Fatalf("classification=%+v/%v primary=%v, want state %v error %v and unchanged producer facts", got, err, class, wantState, wantErr)
			}
			partial, partialErr := current.partial(io.ErrClosedPipe)
			if partial.State() != ObservationPartial || !errors.Is(partialErr, io.ErrClosedPipe) || partial.Validate() != nil || partial.matched != facts.matched || partial.unsupportedRegular != facts.unsupportedRegular {
				t.Fatalf("one-fact native failure mutation=%+v/%v, want partial with unchanged facts", partial, partialErr)
			}
		})
	}
}

// This is a direct unit fixture for the classifier's fs.DirEntry handoff,
// not proof that a fake device was created. Find's real directory tests prove
// the production Filestore path; these exhaust otherwise platform-specific modes.
type modeEntry struct {
	name string
	mode fs.FileMode
}

func (e modeEntry) Name() string               { return e.name }
func (e modeEntry) IsDir() bool                { return e.mode.IsDir() }
func (e modeEntry) Type() fs.FileMode          { return e.mode }
func (e modeEntry) Info() (fs.FileInfo, error) { return nil, fs.ErrInvalid }

func TestObservationSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	base := Observation{kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}
	cases := []struct {
		name    string
		mutate  func(*Observation)
		wantErr error
	}{
		{name: "complete zero contains no invented output", mutate: func(*Observation) {}},
		{name: "complete exact acknowledgments", mutate: func(o *Observation) { o.matched = 3; o.delivered = 3 }},
		{name: "neutral child-only directory remains complete", mutate: func(o *Observation) { o.ignoredDirectories = 1 }},
		{name: "neutral other-mode directory remains complete", mutate: func(o *Observation) { o.nonRegular = 1 }},
		{name: "unsupported count requires unsupported result", mutate: func(o *Observation) { o.unsupportedRegular = 1; o.state = ObservationUnsupportedFormat }},
		{name: "cancellation after acknowledgment remains partial", mutate: func(o *Observation) { o.matched = 1; o.delivered = 1; o.state = ObservationPartial }},
		{name: "refused callback remains an unacknowledged match", mutate: func(o *Observation) { o.matched = 1; o.state = ObservationPartial }},
		{name: "native failure after directory fact remains partial", mutate: func(o *Observation) { o.ignoredDirectories = 1; o.state = ObservationPartial }},
		{name: "failed scan has no directory facts", mutate: func(o *Observation) { o.state = ObservationFailed }},
		{name: "saturated completed counts remain representable", mutate: func(o *Observation) {
			o.matched = math.MaxUint64
			o.delivered = math.MaxUint64
			o.nonRegular = math.MaxUint64
		}},
		{name: "unknown state is refused", mutate: func(o *Observation) { o.state = ObservationUnknown }, wantErr: core.ErrFuzzArtifactContract},
		{name: "future state is refused", mutate: func(o *Observation) { o.state = ObservationState(255) }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unknown kind is refused", mutate: func(o *Observation) { o.kind = ArtifactUnknown }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unknown format preserves owning identity", mutate: func(o *Observation) { o.format = CacheFormatUnknown }, wantErr: core.ErrFuzzArtifactContract},
		{name: "acknowledgment without match is impossible", mutate: func(o *Observation) { o.delivered = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "two failed callbacks imply continuation after refusal", mutate: func(o *Observation) { o.state = ObservationPartial; o.matched = 2 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "complete cannot contain an unacknowledged callback", mutate: func(o *Observation) { o.matched = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "complete cannot contain unsupported regular file", mutate: func(o *Observation) { o.unsupportedRegular = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unsupported result must contain unsupported fact", mutate: func(o *Observation) { o.state = ObservationUnsupportedFormat }, wantErr: core.ErrFuzzArtifactContract},
		{name: "unsupported result cannot hide callback failure", mutate: func(o *Observation) { o.state = ObservationUnsupportedFormat; o.unsupportedRegular = 1; o.matched = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "partial cannot invent progress", mutate: func(o *Observation) { o.state = ObservationPartial }, wantErr: core.ErrFuzzArtifactContract},
		{name: "failed cannot retain a match", mutate: func(o *Observation) { o.state = ObservationFailed; o.matched = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "failed cannot retain directory evidence", mutate: func(o *Observation) { o.state = ObservationFailed; o.ignoredDirectories = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "failed cannot retain other-mode evidence", mutate: func(o *Observation) { o.state = ObservationFailed; o.nonRegular = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "failed cannot retain format evidence", mutate: func(o *Observation) { o.state = ObservationFailed; o.unsupportedRegular = 1 }, wantErr: core.ErrFuzzArtifactContract},
		{name: "maximum callback deficit must not wrap", mutate: func(o *Observation) { o.state = ObservationPartial; o.matched = math.MaxUint64 }, wantErr: core.ErrFuzzArtifactContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := base
			tc.mutate(&got)
			if err := got.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Observation.Validate(%+v)=%v, want %v", got, err, tc.wantErr)
			}
		})
	}
}

func TestObservationStateExhaustsClosedDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			state := ObservationState(raw)
			wantValid := state == ObservationComplete || state == ObservationPartial || state == ObservationFailed || state == ObservationUnsupportedFormat
			err := state.Validate()
			if (err == nil) != wantValid || state.IsValid() != wantValid {
				t.Fatalf("state %d validity=%v/%v, want %v", raw, err, state.IsValid(), wantValid)
			}
			if !wantValid && (!errors.Is(err, core.ErrFuzzArtifactContract) || state.String() != core.UnknownEnumDiagnostic) {
				t.Fatalf("unknown state=%v/%v, want typed rejection and unknown diagnostic", state, err)
			}
		})
	}
}

func TestEntryCountersSaturateWithoutWrapping(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		in, want uint64
	}{
		{name: "empty counter advances", want: 1},
		{name: "one before maximum reaches maximum", in: math.MaxUint64 - 1, want: math.MaxUint64},
		{name: "maximum never wraps to empty", in: math.MaxUint64, want: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.in
			incrementSaturating(&got)
			if (EntryCount{value: got}).Uint64() != tc.want {
				t.Fatalf("increment(%d)=%d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// Exhaust all combinations of the four emitted entry categories, followed by
// normal completion or an I/O failure. These are direct producer/finalizer
// tests; the native Find tables separately prove the filesystem boundary.
func TestProducerClassifierCombinationMatrixLayerTriad(t *testing.T) {
	t.Parallel()
	type primaryClass uint8
	const (
		contradiction primaryClass = iota + 1
		typedRefusal
		neutral
		boundary
	)
	rows := []struct {
		name        string
		mask        uint8
		interrupted bool
		primary     primaryClass
	}{}
	for mask := range uint8(16) {
		for _, interrupted := range []bool{false, true} {
			class := boundary
			if mask == 0 {
				class = neutral
			}
			if mask&2 != 0 || interrupted {
				class = typedRefusal
			}
			if mask == 0 && interrupted {
				class = contradiction
			}
			rows = append(rows, struct {
				name        string
				mask        uint8
				interrupted bool
				primary     primaryClass
			}{
				name: strconv.Itoa(int(mask)) + " categories interrupted=" + strconv.FormatBool(interrupted), mask: mask, interrupted: interrupted, primary: class})
		}
	}
	for _, tc := range rows {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			current := newFinder(FindRequest{Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(GeneratedName) error { calls++; return nil }})
			source := []modeEntry{
				{name: generatedNameForPosition(t, ArtifactCorpus, 1).String()},
				{name: "foreign"},
				{name: generatedNameForPosition(t, ArtifactCorpus, 2).String(), mode: fs.ModeDir},
				{name: generatedNameForPosition(t, ArtifactCorpus, 3).String(), mode: fs.ModeSymlink},
			}
			for index, entry := range source {
				if tc.mask&(1<<index) == 0 {
					continue
				}
				if _, err := current.visit(filestore.WalkEntry{Entry: entry}); err != nil {
					t.Fatalf("producer error=%v, want admitted directory facts", err)
				}
			}
			before := current.observation
			if before.matched != uint64(tc.mask&1) || before.delivered != uint64(calls) || before.unsupportedRegular != uint64((tc.mask>>1)&1) || before.ignoredDirectories != uint64((tc.mask>>2)&1) || before.nonRegular != uint64((tc.mask>>3)&1) {
				t.Fatalf("producer facts=%+v, want exact membership mask %d", before, tc.mask)
			}
			got, err := current.finish()
			wantState := ObservationComplete
			if tc.mask&2 != 0 {
				wantState = ObservationUnsupportedFormat
			}
			if tc.interrupted {
				got, err = current.partial(io.ErrUnexpectedEOF)
				wantState = ObservationPartial
			}
			if got.state != wantState || got.matched != before.matched || got.delivered != before.delivered || got.nonRegular != before.nonRegular || got.unsupportedRegular != before.unsupportedRegular || got.ignoredDirectories != before.ignoredDirectories {
				t.Fatalf("classified=%+v/%v primary=%v, want state %v and unchanged producer facts", got, err, tc.primary, wantState)
			}
			if tc.interrupted && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("native failure=%v, want %v", err, io.ErrUnexpectedEOF)
			}
			if tc.mask&2 != 0 && !errors.Is(err, core.ErrFuzzArtifactFormat) {
				t.Fatalf("format refusal=%v, want %v", err, core.ErrFuzzArtifactFormat)
			}
			if tc.primary == contradiction {
				if !errors.Is(err, core.ErrFuzzArtifactContract) || got.Validate() == nil {
					t.Fatalf("invented partial progress=%+v/%v, want schema contradiction", got, err)
				}
			} else if got.Validate() != nil {
				t.Fatalf("admitted classification=%+v/%v, want valid observation", got, err)
			}
			if !tc.interrupted && tc.mask&2 == 0 && err != nil {
				t.Fatalf("complete result error=%v, want nil", err)
			}
		})
	}
}
