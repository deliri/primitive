package runprotocol

import (
	"bytes"
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
)

func runReceiptFixture(t testing.TB) RunReceipt {
	t.Helper()
	ref := func(name string, content []byte) ArtifactReference {
		count, err := core.NewByteCount(uint64(len(content)))
		if err != nil {
			t.Fatal(err)
		}
		return ArtifactReference{Path: fixturePath(t, name), Digest: core.SHA256Of(content), Bytes: count}
	}
	results := ref("test-results.jsonl", []byte("native result fixture"))
	return RunReceipt{
		Run: fixtureIdentifier(t, "run-1792542374974150000-02d87feccfb2"), Producer: fixtureIdentifier(t, "stoker"),
		Caller: fixtureIdentifier(t, "codex-receipt-proof"), Worker: fixtureIdentifier(t, "mac"),
		Source: SourceCoordinate{Repository: fixtureRepository(t, "github.com/offGridSoft/example"),
			Commit: fixtureCommit(t, strings.Repeat("a", 40)), Tree: core.SHA256Of([]byte("exact committed source"))},
		Invocation: ref("request.json", []byte("invocation fixture")), Execution: ref("receipt.json", []byte("native facts fixture")),
		Results: &results, ResultCount: 1,
		Accounting:  ExecutionAccounting{Attempts: []ExecutionAttempt{{Sequence: 1, Planned: 1, Passed: 1, Cache: CacheDisabled}}},
		CompletedAt: temporal.InstantFromNanoseconds(1792542374974150000), Outcome: OutcomePassed, Terminal: TerminalCompleted,
	}
}

func runReceiptCodec() jsonio.OwnedCodecPolicy {
	return jsonio.OwnedCodecPolicy{Members: jsonio.MembersDeclaredOnly, Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullRefused}
}

// These are schema/projection tests. The evidence refs are synthetic; validating
// them does not claim that their files exist or that execution occurred.
func TestRunReceiptLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		edit    func(*RunReceipt)
		wantErr error
	}{
		{name: "positive requestless receipt preserves exact source and attribution", edit: func(*RunReceipt) {}},
		{name: "positive retry retains original failed attempt", edit: func(r *RunReceipt) {
			r.Accounting.Attempts = []ExecutionAttempt{{Sequence: 1, Planned: 1, Failed: 1, Cache: CacheDisabled}, {Sequence: 2, Planned: 1, Passed: 1, Cache: CacheDisabled}}
		}},
		{name: "positive native result count has no arbitrary extent quota", edit: func(r *RunReceipt) { r.ResultCount = math.MaxUint64 }},
		{name: "neutral successful command need not invent a test occurrence", edit: func(r *RunReceipt) { r.ResultCount = 0; r.Results = nil }},
		{name: "negative declared result stream needs a digest", edit: func(r *RunReceipt) { r.Results.Digest = core.SHA256Digest{} }, wantErr: core.ErrRunProtocolContract},
		{name: "positive maximum denominator does not overflow", edit: func(r *RunReceipt) {
			r.Accounting.Attempts[0].Planned = math.MaxUint32
			r.Accounting.Attempts[0].Passed = math.MaxUint32
		}},
		{name: "positive oldest representable timestamp", edit: func(r *RunReceipt) { r.CompletedAt = temporal.InstantFromNanoseconds(math.MinInt64) }},
		{name: "positive newest representable timestamp", edit: func(r *RunReceipt) { r.CompletedAt = temporal.InstantFromNanoseconds(math.MaxInt64) }},
		{name: "neutral epoch timestamp is observed rather than unset", edit: func(r *RunReceipt) { r.CompletedAt = temporal.InstantFromNanoseconds(0) }},
		{name: "neutral empty test stream does not invent test results", edit: func(r *RunReceipt) {
			r.Results = nil
			r.ResultCount = 0
			r.Outcome = OutcomeNotRun
			r.Terminal = TerminalNotRun
			r.Accounting.Attempts[0].Passed = 0
			r.Accounting.Attempts[0].NotRun = 1
		}},
		{name: "negative missing run identity", edit: func(r *RunReceipt) { r.Run = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing producer identity", edit: func(r *RunReceipt) { r.Producer = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing caller attribution", edit: func(r *RunReceipt) { r.Caller = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing actual worker", edit: func(r *RunReceipt) { r.Worker = Identifier{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing repository", edit: func(r *RunReceipt) { r.Source.Repository = RepositoryIdentity{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing committed revision", edit: func(r *RunReceipt) { r.Source.Commit = core.BuildCommit{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing source tree digest", edit: func(r *RunReceipt) { r.Source.Tree = core.SHA256Digest{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing invocation reference", edit: func(r *RunReceipt) { r.Invocation = ArtifactReference{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing native execution reference", edit: func(r *RunReceipt) { r.Execution = ArtifactReference{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative missing result stream reference", edit: func(r *RunReceipt) { r.Results = nil }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative missing observed timestamp", edit: func(r *RunReceipt) { r.CompletedAt = temporal.Instant{} }, wantErr: core.ErrRunProtocolContract},
		{name: "negative absent attempt cannot stand for success", edit: func(r *RunReceipt) { r.Accounting.Attempts = nil }, wantErr: core.ErrRunProtocolContract},
		{name: "negative unclosed denominator", edit: func(r *RunReceipt) { r.Accounting.Attempts[0].Planned = 2 }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative missing first attempt sequence", edit: func(r *RunReceipt) { r.Accounting.Attempts[0].Sequence = 2 }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative retry changes declared scope", edit: func(r *RunReceipt) {
			r.Accounting.Attempts = append(r.Accounting.Attempts, ExecutionAttempt{Sequence: 2, Planned: 2, Passed: 2, Cache: CacheDisabled})
		}, wantErr: core.ErrRunProtocolConflict},
		{name: "negative invocation aliases execution bytes", edit: func(r *RunReceipt) { r.Invocation.Path = r.Execution.Path }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative invocation aliases result stream", edit: func(r *RunReceipt) { r.Invocation.Path = r.Results.Path }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative execution aliases result stream", edit: func(r *RunReceipt) { r.Execution.Path = r.Results.Path }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative result artifact cannot claim zero occurrences", edit: func(r *RunReceipt) { r.ResultCount = 0 }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative pass cannot hide skipped execution units", edit: func(r *RunReceipt) { r.Accounting.Attempts[0].Passed = 0; r.Accounting.Attempts[0].Skipped = 1 }, wantErr: core.ErrRunProtocolConflict},
		{name: "negative pass cannot erase latest failed retry", edit: func(r *RunReceipt) {
			r.Accounting.Attempts = append(r.Accounting.Attempts, ExecutionAttempt{Sequence: 2, Planned: 1, Failed: 1, Cache: CacheDisabled})
		}, wantErr: core.ErrRunProtocolConflict},
		{name: "negative zero receipt emits no fabricated metadata", edit: func(r *RunReceipt) { *r = RunReceipt{} }, wantErr: core.ErrRunProtocolContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := runReceiptFixture(t)
			tc.edit(&want)
			if err := want.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("RunReceipt.Validate() error=%v, want %v", err, tc.wantErr)
			}
			encoded, err := jsonio.MarshalValidated(want, runReceiptCodec())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || len(encoded) != 0 {
					t.Fatalf("rejected receipt projection=(%q,%v), want no bytes and %v", encoded, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := jsonio.DecodeOwned[RunReceipt](encoded, runReceiptCodec())
			if err != nil || !sameRunReceipt(got, want) {
				t.Fatalf("receipt projection=(%+v,%v), want %+v", got, err, want)
			}
		})
	}
}

func sameRunReceiptResults(a, b *ArtifactReference) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameRunReceipt(a, b RunReceipt) bool {
	return a.Run == b.Run && a.Producer == b.Producer && a.Caller == b.Caller && a.Worker == b.Worker && a.Source == b.Source &&
		a.Invocation == b.Invocation && a.Execution == b.Execution && sameRunReceiptResults(a.Results, b.Results) && a.ResultCount == b.ResultCount &&
		slices.Equal(a.Accounting.Attempts, b.Accounting.Attempts) && a.CompletedAt == b.CompletedAt && a.Outcome == b.Outcome && a.Terminal == b.Terminal
}

// agreeingAttempt is an independent oracle: one execution unit whose count
// explains outcome. Outcomes without a per-unit count use one unit not run.
func agreeingAttempt(outcome Outcome) ExecutionAttempt {
	attempt := ExecutionAttempt{Sequence: 1, Planned: 1, Cache: CacheDisabled}
	switch outcome {
	case OutcomePassed:
		attempt.Passed = 1
	case OutcomeFailed:
		attempt.Failed = 1
	case OutcomeSkipped:
		attempt.Skipped = 1
	case OutcomeUnavailable:
		attempt.Unavailable = 1
	case OutcomeTimedOut:
		attempt.Expired = 1
	case OutcomeCancelled:
		attempt.Cancelled = 1
	default:
		attempt.NotRun = 1
	}
	return attempt
}

// A receipt's outcome must be explained by its latest attempt's own counts:
// each row pairs an outcome with counts that agree and with counts that do not.
func TestRunReceiptOutcomeAgreesWithLatestAttempt(t *testing.T) {
	t.Parallel()
	attempt := func(edit func(*ExecutionAttempt)) ExecutionAttempt {
		a := ExecutionAttempt{Sequence: 1, Planned: 2, Cache: CacheDisabled}
		edit(&a)
		return a
	}
	for _, row := range []struct {
		name    string
		outcome Outcome
		state   TerminalState
		attempt ExecutionAttempt
		agree   bool
	}{
		{"failed with a failure", OutcomeFailed, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Passed, a.Failed = 1, 1 }), true},
		{"failed with no failure", OutcomeFailed, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Passed, a.Skipped = 1, 1 }), false},
		{"skipped with only skips", OutcomeSkipped, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Skipped = 2 }), true},
		{"skipped while something passed", OutcomeSkipped, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Passed, a.Skipped = 1, 1 }), false},
		{"skipped while something failed", OutcomeSkipped, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Failed, a.Skipped = 1, 1 }), false},
		{"skipped with nothing skipped", OutcomeSkipped, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.NotRun = 2 }), false},
		{"unavailable with an unavailable unit", OutcomeUnavailable, TerminalUnavailable, attempt(func(a *ExecutionAttempt) { a.Passed, a.Unavailable = 1, 1 }), true},
		{"unavailable with none", OutcomeUnavailable, TerminalUnavailable, attempt(func(a *ExecutionAttempt) { a.Passed, a.NotRun = 1, 1 }), false},
		{"timed out with an expired unit", OutcomeTimedOut, TerminalTimedOut, attempt(func(a *ExecutionAttempt) { a.Passed, a.Expired = 1, 1 }), true},
		{"timed out with none", OutcomeTimedOut, TerminalTimedOut, attempt(func(a *ExecutionAttempt) { a.Passed, a.NotRun = 1, 1 }), false},
		{"cancelled with a cancelled unit", OutcomeCancelled, TerminalCancelled, attempt(func(a *ExecutionAttempt) { a.Passed, a.Cancelled = 1, 1 }), true},
		{"cancelled with none", OutcomeCancelled, TerminalCancelled, attempt(func(a *ExecutionAttempt) { a.Passed, a.NotRun = 1, 1 }), false},
		{"not run with a unit not run", OutcomeNotRun, TerminalNotRun, attempt(func(a *ExecutionAttempt) { a.Passed, a.NotRun = 1, 1 }), true},
		{"not run with none", OutcomeNotRun, TerminalNotRun, attempt(func(a *ExecutionAttempt) { a.Passed, a.Failed = 1, 1 }), false},
		{"infrastructure failure short of all passing", OutcomeInfrastructureFailed, TerminalFailed, attempt(func(a *ExecutionAttempt) { a.NotRun = 2 }), true},
		{"infrastructure failure while everything passed", OutcomeInfrastructureFailed, TerminalFailed, attempt(func(a *ExecutionAttempt) { a.Passed = 2 }), false},
		{"setup failure while everything passed", OutcomeSetupFailed, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Passed = 2 }), false},
		{"non-accepting while everything passed", OutcomeNonAccepting, TerminalCompleted, attempt(func(a *ExecutionAttempt) { a.Passed = 2 }), false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			r := runReceiptFixture(t)
			r.Outcome, r.Terminal = row.outcome, row.state
			r.Accounting.Attempts[0] = row.attempt
			err := r.Validate()
			if row.agree && err != nil || !row.agree && !errors.Is(err, core.ErrRunProtocolConflict) {
				t.Fatalf("receipt %s with %+v = %v, want agreement %t", row.outcome, row.attempt, err, row.agree)
			}
		})
	}
}

func TestReceiptOutcomeGuardRefusesUnknown(t *testing.T) {
	t.Parallel()
	latest := ExecutionAttempt{Sequence: 1, Planned: 1, NotRun: 1, Cache: CacheDisabled}
	if err := latest.Validate(); err != nil {
		t.Fatalf("nonpassing native attempt validation = %v, want nil", err)
	}
	if got := receiptOutcomeAgrees(outcomeLimit, latest); got {
		t.Fatalf("unknown outcome agreement with a nonpassing attempt = %t, want false", got)
	}
}

func TestRunReceiptOutcomeTerminalDomainExhaustive(t *testing.T) {
	t.Parallel()
	// Independent oracle for the existing domain: failed assertions are completed
	// execution, while infrastructure failure has the failed terminal state.
	wantTerminals := [...]TerminalState{TerminalUnknown, TerminalCompleted, TerminalCompleted, TerminalCompleted,
		TerminalUnavailable, TerminalTimedOut, TerminalCancelled, TerminalCompleted, TerminalFailed, TerminalNotRun, TerminalCompleted}
	for outcome := 0; outcome <= math.MaxUint8; outcome++ {
		for terminal := 0; terminal <= math.MaxUint8; terminal++ {
			r := runReceiptFixture(t)
			r.Outcome, r.Terminal = Outcome(outcome), TerminalState(terminal)
			r.Accounting.Attempts[0] = agreeingAttempt(Outcome(outcome))
			err := r.Validate()
			wantErr := error(core.ErrRunProtocolConflict)
			if outcome == 0 || outcome >= len(wantTerminals) || terminal == 0 || terminal > int(TerminalNotRun) {
				wantErr = core.ErrRunProtocolContract
			} else if TerminalState(terminal) == wantTerminals[outcome] {
				wantErr = nil
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("receipt outcome=%d terminal=%d error=%v, want %v", outcome, terminal, err, wantErr)
			}
		}
	}
}

func TestRunReceiptJSONHostileAdmission(t *testing.T) {
	t.Parallel()
	encoded, err := jsonio.MarshalValidated(runReceiptFixture(t), runReceiptCodec())
	if err != nil {
		t.Fatal(err)
	}
	good := string(encoded)
	for _, tc := range []struct{ name, input string }{
		{"empty document", ""}, {"whitespace document", " \t\n"}, {"null root", "null"}, {"array root", "[]"},
		{"missing every required fact", "{}"},
		{"duplicate root member", strings.Replace(good, `"producer":`, `"producer":"foreign","producer":`, 1)},
		{"unknown root member", strings.Replace(good, `"producer":`, `"future":true,"producer":`, 1)},
		{"null accounting", strings.Replace(good, `"accounting":{`, `"accounting":null,"unused":{`, 1)},
		{"wrong identity type", strings.Replace(good, `"worker":"mac"`, `"worker":42`, 1)},
		{"unknown outcome", strings.Replace(good, `"outcome_passed"`, `"future_outcome"`, 1)},
		{"unknown terminal", strings.Replace(good, `"terminal_completed"`, `"future_terminal"`, 1)},
		{"truncated final member", good[:len(good)-1]},
		{"second root cannot be absorbed", good + good},
		{"trailing nonwhitespace", good + " false"},
		{"malformed committed revision", strings.Replace(good, strings.Repeat("a", 40), "not-a-commit", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.DecodeOwned[RunReceipt]([]byte(tc.input), runReceiptCodec())
			if !errors.Is(err, core.ErrJSONContract) || !sameRunReceipt(got, RunReceipt{}) {
				t.Fatalf("receipt admission=(%+v,%v), want zero receipt and typed JSON refusal", got, err)
			}
		})
	}
}

func TestRunReceiptStreamLayerTriad(t *testing.T) {
	t.Parallel()
	encoded, err := jsonio.MarshalValidated(runReceiptFixture(t), runReceiptCodec())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                       string
		source                     io.Reader
		wantReceipts, wantRefusals int
	}{
		{name: "positive receipt streams without a result population", source: bytes.NewReader(encoded), wantReceipts: 1},
		{name: "negative truncated suffix preserves prefix but refuses completion", source: io.MultiReader(bytes.NewReader(encoded), strings.NewReader("\n{")), wantReceipts: 1, wantRefusals: 1},
		{name: "neutral absent receipt cannot become an empty success", source: strings.NewReader("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			count, refusals := 0, 0
			for got, err := range jsonio.Objects[RunReceipt](t.Context(), jsonio.ObjectSourceRequest{Source: tc.source}) {
				if err != nil {
					if !errors.Is(err, core.ErrJSONContract) || !sameRunReceipt(got, RunReceipt{}) {
						t.Fatalf("stream refusal=(%+v,%v), want zero receipt and typed refusal", got, err)
					}
					refusals++
					continue
				}
				if !sameRunReceipt(got, runReceiptFixture(t)) {
					t.Fatalf("stream receipt=%+v, want exact typed facts", got)
				}
				count++
			}
			if count != tc.wantReceipts || refusals != tc.wantRefusals {
				t.Fatalf("stream=(%d receipts,%d refusals), want (%d,%d)", count, refusals, tc.wantReceipts, tc.wantRefusals)
			}
		})
	}
}

func FuzzRunReceiptJSONSemanticClosure(f *testing.F) {
	seed, err := jsonio.MarshalValidated(runReceiptFixture(f), runReceiptCodec())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Add(append(bytes.Clone(seed), '{'))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := jsonio.DecodeOwned[RunReceipt](data, runReceiptCodec())
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !sameRunReceipt(got, RunReceipt{}) {
				t.Fatalf("rejected receipt=(%+v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("admitted receipt.Validate()=%v, want nil", err)
		}
		canonical, err := jsonio.MarshalValidated(got, runReceiptCodec())
		if err != nil {
			t.Fatal(err)
		}
		roundTrip, err := jsonio.DecodeOwned[RunReceipt](canonical, runReceiptCodec())
		if err != nil || !sameRunReceipt(roundTrip, got) {
			t.Fatalf("receipt round trip=(%+v,%v), want %+v", roundTrip, err, got)
		}
		second, err := jsonio.MarshalValidated(roundTrip, runReceiptCodec())
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("second projection=(%q,%v), want %q", second, err, canonical)
		}
	})
}
