package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/taskmanager"
	"github.com/deliri/primitive/v2026/temporal"
)

type evidenceInputPressureClass uint8

const (
	evidenceInputPressureUnknown evidenceInputPressureClass = iota
	evidenceInputPressureValid
	evidenceInputPressureReject
	evidenceInputPressureBoundary
	evidenceInputPressureLimit
)

type evidenceInputPressureCase struct {
	mutate  func(testing.TB, *appendEvidenceInput)
	wantErr error
	name    string
	class   evidenceInputPressureClass
}

type evidenceSourceDisposition uint8

const (
	evidenceSourceRegular evidenceSourceDisposition = iota + 1
	evidenceSourceMissing
	evidenceSourceDirectory
	evidenceSourceEmpty
	evidenceSourceOversized
	evidenceSourceConfinedLink
	evidenceSourceEscapingLink
	evidenceSourceBrokenLink
	evidenceSourceHardLink
)

type evidencePreparationPressureClass uint8

const (
	evidencePreparationPressureUnknown evidencePreparationPressureClass = iota
	evidencePreparationPressureValid
	evidencePreparationPressureReject
	evidencePreparationPressureBoundary
	evidencePreparationPressureLimit
)

type evidencePreparationContext uint8

const (
	evidencePreparationContextActive evidencePreparationContext = iota + 1
	evidencePreparationContextCancelled
	evidencePreparationContextNil
)

type evidencePreparationPressureCase struct {
	mutate      func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest)
	wantErr     error
	name        string
	disposition evidenceSourceDisposition
	context     evidencePreparationContext
	class       evidencePreparationPressureClass
}

func TestAppendEvidenceInputPressuresEveryOwnedLocalSourceBoundary(t *testing.T) {
	t.Parallel()

	cases := evidenceInputPressureCases()
	var gotClassCounts [evidenceInputPressureLimit]uint8
	for _, tc := range cases {
		gotClassCounts[tc.class]++
	}
	wantClassCounts := [evidenceInputPressureLimit]uint8{
		evidenceInputPressureValid: 10, evidenceInputPressureReject: 10, evidenceInputPressureBoundary: 20,
	}
	if gotClassCounts != wantClassCounts {
		t.Fatalf("evidence input pressure counts = %v, want %v", gotClassCounts, wantClassCounts)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := taskEvidenceInputFixture(t)
			if tc.mutate != nil {
				tc.mutate(t, &input)
			}
			before := input
			gotErr := input.Validate()
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || input != before {
					t.Fatalf("appendEvidenceInput.Validate() = (%+v, %v), want unchanged and errors.Is(..., %v)", input, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || input != before {
				t.Fatalf("appendEvidenceInput.Validate() = (%+v, %v), want unchanged %+v and nil", input, gotErr, before)
			}
			job := jobDocument{
				Revision: commandDocumentRevisionV2, Operation: operationAppendEvidence, AppendEvidence: &input,
			}
			encoded, encodeErr := job.MarshalJSON()
			if encodeErr != nil || len(encoded) > commandDocumentMaxBytes {
				t.Fatalf("accepted evidence job MarshalJSON() = (%d bytes, %v), want bounded bytes and nil", len(encoded), encodeErr)
			}
		})
	}
}

func evidenceInputPressureCases() []evidenceInputPressureCase {
	valid := evidenceInputPressureValid
	reject := evidenceInputPressureReject
	boundary := evidenceInputPressureBoundary
	return []evidenceInputPressureCase{
		{name: "valid ordinary PNG screenshot", class: valid},
		{name: "valid JSON test transcript", class: valid, mutate: evidenceSetMediaAndSource("application/json", "taskops/evidence/test.json")},
		{name: "valid plain text transcript with charset", class: valid, mutate: evidenceSetMediaAndSource("text/plain; charset=utf-8", "taskops/evidence/test.txt")},
		{name: "valid vendor artifact media type", class: valid, mutate: evidenceSetMediaAndSource("application/vnd.witness.proof+json", "taskops/evidence/proof.witness")},
		{name: "valid Unicode filename remains a relative source", class: valid, mutate: evidenceSetSource("taskops/evidence/résultat.png")},
		{name: "valid filename containing spaces", class: valid, mutate: evidenceSetSource("taskops/evidence/browser proof.png")},
		{name: "valid deeply nested evidence path", class: valid, mutate: evidenceSetSource("taskops/evidence/run/package/test/output.txt")},
		{name: "valid single-component source", class: valid, mutate: evidenceSetSource("proof.png")},
		{name: "valid punctuation confined to filename", class: valid, mutate: evidenceSetSource("taskops/evidence/proof_01-final.png")},
		{name: "valid Unicode summary", class: valid, mutate: evidenceSetSummary("Résumé de preuve rendu correctement.")},

		{name: "reject empty source", class: reject, mutate: evidenceSetSource(""), wantErr: core.ErrPrimitiveContract},
		{name: "reject absolute source", class: reject, mutate: evidenceSetSource("/tmp/proof.png"), wantErr: core.ErrPrimitiveContract},
		{name: "reject parent escape source", class: reject, mutate: evidenceSetSource("../proof.png"), wantErr: core.ErrPrimitiveContract},
		{name: "reject embedded parent navigation", class: reject, mutate: evidenceSetSource("taskops/../proof.png"), wantErr: core.ErrPrimitiveContract},
		{name: "reject current-directory alias", class: reject, mutate: evidenceSetSource("./proof.png"), wantErr: core.ErrPrimitiveContract},
		{name: "reject doubled separator", class: reject, mutate: evidenceSetSource("taskops//proof.png"), wantErr: core.ErrPrimitiveContract},
		{name: "reject source containing NUL", class: reject, mutate: evidenceSetSource("taskops/proof\x00.png"), wantErr: core.ErrPrimitiveContract},
		{name: "reject unset content type", class: reject, mutate: evidenceUnsetMedia, wantErr: core.ErrPrimitiveContract},
		{name: "reject unset task identity", class: reject, mutate: evidenceUnsetTask, wantErr: core.ErrIDContract},
		{name: "reject unset expected revision", class: reject, mutate: evidenceUnsetRevision, wantErr: core.ErrTaskManagerContract},

		{name: "boundary one-rune summary is accepted", class: boundary, mutate: evidenceSetSummary("e")},
		{name: "boundary exact maximum summary is accepted", class: boundary, mutate: evidenceSetSummary(strings.Repeat("e", taskmanager.EvidenceSummaryMaximumRunes))},
		{name: "boundary one above maximum summary is rejected", class: boundary, mutate: evidenceSetSummary(strings.Repeat("e", taskmanager.EvidenceSummaryMaximumRunes+1)), wantErr: core.ErrTaskManagerContract},
		{name: "boundary empty summary is rejected", class: boundary, mutate: evidenceSetSummary(""), wantErr: core.ErrTaskManagerContract},
		{name: "boundary whitespace-only summary is rejected", class: boundary, mutate: evidenceSetSummary(" \t "), wantErr: core.ErrTaskManagerContract},
		{name: "boundary summary newline is rejected", class: boundary, mutate: evidenceSetSummary("proof\nsecond line"), wantErr: core.ErrTaskManagerContract},
		{name: "boundary note evidence kind is accepted", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindNote)},
		{name: "boundary test evidence kind is accepted", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindTest)},
		{name: "boundary benchmark evidence kind is accepted", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindBenchmark)},
		{name: "boundary screenshot evidence kind is accepted", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindScreenshot)},
		{name: "boundary Git inventory evidence kind is accepted", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindGitInventory)},
		{name: "boundary artifact evidence kind is accepted", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindArtifact)},
		{name: "boundary unknown evidence kind is rejected", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKindUnknown), wantErr: core.ErrTaskManagerContract},
		{name: "boundary future evidence kind is rejected", class: boundary, mutate: evidenceSetKind(taskmanager.EvidenceKind(math.MaxUint8)), wantErr: core.ErrTaskManagerContract},
		{name: "boundary revision floor is accepted", class: boundary, mutate: evidenceSetRevision(1)},
		{name: "boundary revision ceiling is accepted", class: boundary, mutate: evidenceSetRevision(math.MaxUint64)},
		{name: "boundary exact path-component ceiling is accepted", class: boundary, mutate: evidenceSetSource(evidencePathComponents(core.FilesystemPathMaximumComponents))},
		{name: "boundary one above path-component ceiling is rejected", class: boundary, mutate: evidenceSetSource(evidencePathComponents(core.FilesystemPathMaximumComponents + 1)), wantErr: core.ErrPrimitiveContract},
		{name: "boundary dot source cannot name a file", class: boundary, mutate: evidenceSetSource("."), wantErr: core.ErrTaskManagerContract},
		{name: "boundary media type with quoted parameter is accepted", class: boundary, mutate: evidenceSetMediaAndSource(`text/plain; note="a;b"`, "taskops/evidence/proof.txt")},
	}
}

func evidenceSetSource(value string) func(testing.TB, *appendEvidenceInput) {
	return func(_ testing.TB, input *appendEvidenceInput) { input.Source = value }
}

func evidenceSetSummary(value string) func(testing.TB, *appendEvidenceInput) {
	return func(_ testing.TB, input *appendEvidenceInput) { input.Summary = value }
}

func evidenceSetKind(value taskmanager.EvidenceKind) func(testing.TB, *appendEvidenceInput) {
	return func(_ testing.TB, input *appendEvidenceInput) { input.Kind = value }
}

func evidenceUnsetMedia(_ testing.TB, input *appendEvidenceInput) {
	input.ContentType = core.HTTPMediaType{}
}

func evidenceUnsetTask(_ testing.TB, input *appendEvidenceInput) { input.TaskID = id.UUIDv7{} }

func evidenceUnsetRevision(_ testing.TB, input *appendEvidenceInput) {
	input.ExpectedRevision = taskmanager.Revision{}
}

func evidenceSetRevision(value uint64) func(testing.TB, *appendEvidenceInput) {
	return func(t testing.TB, input *appendEvidenceInput) {
		t.Helper()
		revision, err := taskmanager.NewRevision(value)
		if err != nil {
			t.Fatalf("taskmanager.NewRevision(%d) error = %v, want nil", value, err)
		}
		input.ExpectedRevision = revision
	}
}

func evidenceSetMediaAndSource(mediaType, source string) func(testing.TB, *appendEvidenceInput) {
	return func(t testing.TB, input *appendEvidenceInput) {
		t.Helper()
		parsed, err := core.ParseHTTPMediaType(mediaType)
		if err != nil {
			t.Fatalf("core.ParseHTTPMediaType(%q) error = %v, want nil", mediaType, err)
		}
		input.ContentType = parsed
		input.Source = source
	}
}

func evidencePathComponents(count int) string {
	return strings.Repeat("p/", count-1) + "proof.png"
}

func taskEvidenceInputFixture(t testing.TB) appendEvidenceInput {
	t.Helper()
	contentType, err := core.ParseHTTPMediaType("image/png")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	revision, err := taskmanager.NewRevision(2)
	if err != nil {
		t.Fatalf("taskmanager.NewRevision() error = %v, want nil", err)
	}
	return appendEvidenceInput{
		TaskID: commandUUIDFixture(t, "0000ffff-ffff-7000-8000-000000000001"),
		Kind:   taskmanager.EvidenceKindScreenshot, Summary: "Rendered browser proof.",
		Source: "taskops/evidence/browser-proof.png", ContentType: contentType,
		ExpectedRevision: revision,
	}
}

func TestTaskEvidenceStorageConfigurationLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate  func(testing.TB, *configurationDocument)
		job     func(testing.TB) jobDocument
		wantErr error
		name    string
	}{
		{name: "positive append binds project and validated storage", job: taskEvidenceAppendJobFixture},
		{name: "positive provider object ceiling remains bounded and accepted", job: taskEvidenceAppendJobFixture, mutate: evidenceSetStorageMaximum(objectstore.GoogleCloudStorageObjectMaximumBytes)},
		{name: "negative append without storage is refused before source I/O", job: taskEvidenceAppendJobFixture, mutate: evidenceRemoveStorage, wantErr: core.ErrTaskManagerContract},
		{name: "negative malformed bucket delegates to the GCS owner", job: taskEvidenceAppendJobFixture, mutate: evidenceSetStorageBucket("Invalid_UPPER"), wantErr: core.ErrObjectStoreContract},
		{name: "negative whole-bucket prefix delegates to the GCS owner", job: taskEvidenceAppendJobFixture, mutate: evidenceSetStoragePrefix(""), wantErr: core.ErrObjectStoreContract},
		{name: "negative leaf-shaped prefix delegates to the GCS owner", job: taskEvidenceAppendJobFixture, mutate: evidenceSetStoragePrefix("task-evidence"), wantErr: core.ErrObjectStoreContract},
		{name: "negative zero upload bound is refused by the byte-count owner", job: taskEvidenceAppendJobFixture, mutate: evidenceZeroStorageMaximum, wantErr: core.ErrPrimitiveContract},
		{name: "negative one byte beyond provider ceiling is refused locally", job: taskEvidenceAppendJobFixture, mutate: evidenceSetStorageMaximum(objectstore.GoogleCloudStorageObjectMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
		{name: "neutral non-evidence operation needs no storage capability", job: taskEvidenceListJobFixture, mutate: evidenceRemoveStorage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			configuration := taskEvidenceConfigurationFixture(t, 64<<20)
			if tc.mutate != nil {
				tc.mutate(t, &configuration)
			}
			before := configuration
			job := tc.job(t)
			gotErr := validateJobConfiguration(configuration, job)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || configuration != before {
					t.Fatalf("validateJobConfiguration() = (%+v, %v), want unchanged and errors.Is(..., %v)", configuration, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || configuration != before {
				t.Fatalf("validateJobConfiguration() = (%+v, %v), want unchanged %+v and nil", configuration, gotErr, before)
			}
		})
	}
}

func taskEvidenceAppendJobFixture(t testing.TB) jobDocument {
	t.Helper()
	input := taskEvidenceInputFixture(t)
	return jobDocument{
		Revision: commandDocumentRevisionV2, Operation: operationAppendEvidence, AppendEvidence: &input,
	}
}

func taskEvidenceListJobFixture(testing.TB) jobDocument {
	return jobDocument{
		Revision: commandDocumentRevisionV2, Operation: operationListTasks,
		ListTasks: &listTasksInput{
			Collection: taskmanager.TaskCollectionActive, Order: taskmanager.PageOrderDescending, Limit: 1,
		},
	}
}

func evidenceRemoveStorage(_ testing.TB, configuration *configurationDocument) {
	configuration.EvidenceStorage = nil
}

func evidenceSetStorageBucket(value string) func(testing.TB, *configurationDocument) {
	return func(_ testing.TB, configuration *configurationDocument) {
		configuration.EvidenceStorage.Bucket = value
	}
}

func evidenceSetStoragePrefix(value string) func(testing.TB, *configurationDocument) {
	return func(_ testing.TB, configuration *configurationDocument) {
		configuration.EvidenceStorage.Prefix = value
	}
}

func evidenceZeroStorageMaximum(_ testing.TB, configuration *configurationDocument) {
	configuration.EvidenceStorage.MaximumBytes = core.ByteCount{}
}

func evidenceSetStorageMaximum(value uint64) func(testing.TB, *configurationDocument) {
	return func(t testing.TB, configuration *configurationDocument) {
		t.Helper()
		maximum, err := core.NewByteCount(value)
		if err != nil {
			t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
		}
		configuration.EvidenceStorage.MaximumBytes = maximum
	}
}

func TestTaskEvidenceSourcePreparationLayerTriad(t *testing.T) {
	t.Parallel()

	cases := evidencePreparationPressureCases()
	var gotClassCounts [evidencePreparationPressureLimit]uint8
	for _, tc := range cases {
		gotClassCounts[tc.class]++
	}
	wantClassCounts := [evidencePreparationPressureLimit]uint8{
		evidencePreparationPressureValid:    10,
		evidencePreparationPressureReject:   10,
		evidencePreparationPressureBoundary: 20,
	}
	if gotClassCounts != wantClassCounts {
		t.Fatalf("evidence preparation pressure counts = %v, want %v", gotClassCounts, wantClassCounts)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatalf("core.ParseAbsolutePath(tempdir) error = %v, want nil", err)
			}
			input := taskEvidenceInputFixture(t)
			input.Source = "evidence/proof.png"
			payload := []byte("bounded task evidence\n")
			configuration := taskEvidenceConfigurationFixture(t, uint64(len(payload)))
			instant, err := temporal.NewInstant(time.Unix(1_786_183_200, 0))
			if err != nil {
				t.Fatalf("temporal.NewInstant() error = %v, want nil", err)
			}
			request := taskEvidencePreparationRequest{
				WorkingDirectory: root, Configuration: configuration, Input: input, Instant: instant,
			}
			externalRoot := t.TempDir()
			scenario := evidenceSourceScenarioRequest{
				Root: root, ExternalRoot: externalRoot, Source: input.Source,
				Payload: payload, Disposition: tc.disposition,
			}
			if tc.mutate != nil {
				tc.mutate(t, &request, &scenario)
			}
			prepareEvidenceSourceScenario(t, scenario)
			beforeTree := taskEvidenceTreeInventory(t, root.String(), externalRoot)
			var ctx context.Context = t.Context()
			switch tc.context {
			case evidencePreparationContextCancelled:
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case evidencePreparationContextNil:
				ctx = nil
			}
			got, gotErr := prepareTaskEvidenceUpload(ctx, request)
			afterTree := taskEvidenceTreeInventory(t, root.String(), externalRoot)
			if !slices.Equal(afterTree, beforeTree) {
				t.Fatalf("evidence source trees after preparation = %v, want unchanged %v", afterTree, beforeTree)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (taskEvidenceUploadPlan{}) {
					t.Fatalf("prepareTaskEvidenceUpload() = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				provePreparationCreatedNoObjectReceipt(t, got)
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("prepareTaskEvidenceUpload() = (%+v, %v), want validated plan and nil", got, gotErr)
			}
			if got.Integrity.SHA256 != core.SHA256Of(scenario.Payload) || got.Integrity.Length.Uint64() != uint64(len(scenario.Payload)) ||
				got.ContentType != request.Input.ContentType || got.Source.String() != filepath.Join(root.String(), request.Input.Source) {
				t.Fatalf("prepared upload facts = %+v, want exact source, media type, extent, and SHA-256", got)
			}
			digest, digestErr := got.Integrity.SHA256.Hex()
			if digestErr != nil {
				t.Fatalf("prepared digest Hex() error = %v, want nil", digestErr)
			}
			wantName := request.Configuration.EvidenceStorage.Prefix + request.Input.TaskID.String() + "/" + digest + "/" + filepath.Base(request.Input.Source)
			if got.Name.String() != wantName {
				t.Fatalf("prepared object name = %q, want %q", got.Name.String(), wantName)
			}
		})
	}
}

func FuzzTaskEvidenceSourcePreparationSemanticClosure(f *testing.F) {
	f.Add([]byte("compiler-owned evidence source seed\n"))
	f.Add([]byte{0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		root, err := core.ParseAbsolutePath(t.TempDir())
		if err != nil {
			t.Fatalf("core.ParseAbsolutePath(tempdir) error = %v, want nil", err)
		}
		input := taskEvidenceInputFixture(t)
		input.Source = "fuzz/source.bin"
		path := filepath.Join(root.String(), input.Source)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("os.MkdirAll(fuzz source parent) error = %v, want nil", err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("os.WriteFile(fuzz source) error = %v, want nil", err)
		}
		const fuzzMaximumBytes = uint64(1 << 20)
		configuration := taskEvidenceConfigurationFixture(t, fuzzMaximumBytes)
		instant, err := temporal.NewInstant(time.Unix(1_786_183_200, 0))
		if err != nil {
			t.Fatalf("temporal.NewInstant(fuzz) error = %v, want nil", err)
		}
		before, err := os.Stat(path)
		if err != nil {
			t.Fatalf("os.Stat(fuzz source before) error = %v, want nil", err)
		}
		got, gotErr := prepareTaskEvidenceUpload(t.Context(), taskEvidencePreparationRequest{
			WorkingDirectory: root, Configuration: configuration, Input: input, Instant: instant,
		})
		after, statErr := os.Stat(path)
		if statErr != nil || before.Size() != after.Size() || before.Mode() != after.Mode() {
			t.Fatalf("fuzz source after preparation = (%v, %v), want unchanged size=%d mode=%v", after, statErr, before.Size(), before.Mode())
		}
		if len(data) == 0 || uint64(len(data)) > fuzzMaximumBytes {
			if !errors.Is(gotErr, core.ErrObjectStoreSize) || got != (taskEvidenceUploadPlan{}) {
				t.Fatalf("prepareTaskEvidenceUpload(rejected fuzz source) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrObjectStoreSize)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("prepareTaskEvidenceUpload(accepted fuzz source) = (%+v, %v), want validated plan and nil", got, gotErr)
		}
		if got.Integrity.SHA256 != core.SHA256Of(data) || got.Integrity.Length.Uint64() != uint64(len(data)) {
			t.Fatalf("accepted fuzz integrity = %+v, want SHA-256 and extent of exact external bytes", got.Integrity)
		}
		digest, err := got.Integrity.SHA256.Hex()
		if err != nil {
			t.Fatalf("accepted fuzz digest Hex() error = %v, want nil", err)
		}
		wantName := configuration.EvidenceStorage.Prefix + input.TaskID.String() + "/" + digest + "/source.bin"
		if got.Name.String() != wantName {
			t.Fatalf("accepted fuzz object name = %q, want %q", got.Name.String(), wantName)
		}
	})
}

func evidencePreparationPressureCases() []evidencePreparationPressureCase {
	valid := evidencePreparationPressureValid
	reject := evidencePreparationPressureReject
	boundary := evidencePreparationPressureBoundary
	active := evidencePreparationContextActive
	return []evidencePreparationPressureCase{
		{name: "valid ordinary file seals one exact plan", class: valid, context: active, disposition: evidenceSourceRegular},
		{name: "valid confined symbolic link seals its target", class: valid, context: active, disposition: evidenceSourceConfinedLink},
		{name: "valid hard link seals the shared inode bytes", class: valid, context: active, disposition: evidenceSourceHardLink},
		{name: "valid nested source remains rooted", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetSource("nested/run/proof.bin")},
		{name: "valid one-byte source matches one-byte bound", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetExtent(1, 1)},
		{name: "valid source one byte below bound stays exact", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetExtent(31, 32)},
		{name: "valid sixty-four-kibibyte source streams at its bound", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetExtent(64<<10, 64<<10)},
		{name: "valid punctuated leaf becomes the object leaf", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetSource("evidence/proof_01-final.bin")},
		{name: "valid JSON media type survives plan projection", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetMedia("application/json")},
		{name: "valid provider ceiling does not inflate a small source", class: valid, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetMaximum(objectstore.GoogleCloudStorageObjectMaximumBytes)},

		{name: "reject absent source before object facts exist", class: reject, context: active, disposition: evidenceSourceMissing, wantErr: core.ErrFilestoreSource},
		{name: "reject directory source before object facts exist", class: reject, context: active, disposition: evidenceSourceDirectory, wantErr: core.ErrFilestoreSource},
		{name: "reject escaping symbolic link before reading external bytes", class: reject, context: active, disposition: evidenceSourceEscapingLink, wantErr: core.ErrFilestoreSource},
		{name: "reject broken symbolic link before object facts exist", class: reject, context: active, disposition: evidenceSourceBrokenLink, wantErr: core.ErrFilestoreSource},
		{name: "reject empty source because evidence cannot prove zero bytes", class: reject, context: active, disposition: evidenceSourceEmpty, mutate: evidencePreparationSetMaximum(1), wantErr: core.ErrObjectStoreSize},
		{name: "reject one-byte oversize source without partial plan", class: reject, context: active, disposition: evidenceSourceOversized, mutate: evidencePreparationOneByteOversize, wantErr: core.ErrObjectStoreSize},
		{name: "reject pre-cancelled context before source inspection", class: reject, context: evidencePreparationContextCancelled, disposition: evidenceSourceRegular, wantErr: context.Canceled},
		{name: "reject nil context before observing the filesystem", class: reject, context: evidencePreparationContextNil, disposition: evidenceSourceRegular, wantErr: core.ErrNilContext},
		{name: "reject zero working directory before source resolution", class: reject, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationZeroRoot, wantErr: core.ErrPrimitiveContract},
		{name: "reject missing evidence storage before source resolution", class: reject, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationRemoveStorage, wantErr: core.ErrTaskManagerContract},

		{name: "boundary three-byte bucket is admitted", class: boundary, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetBucket("abc")},
		{name: "boundary bucket one below minimum is refused", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetBucket("ab"), wantErr: core.ErrObjectStoreContract},
		{name: "boundary 222-byte dotted bucket is admitted", class: boundary, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetBucket(strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 30))},
		{name: "boundary bucket one above provider maximum is refused", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetBucket(strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 31)), wantErr: core.ErrObjectStoreContract},
		{name: "boundary nested slash-terminated prefix is admitted", class: boundary, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetPrefix("proof/task/runs/")},
		{name: "boundary leaf-shaped prefix is refused", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetPrefix("proof"), wantErr: core.ErrObjectStoreContract},
		{name: "boundary empty prefix is refused", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetPrefix(""), wantErr: core.ErrObjectStoreContract},
		{name: "boundary zero maximum is refused before source I/O", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetMaximum(0), wantErr: core.ErrPrimitiveContract},
		{name: "boundary provider maximum admits bounded source", class: boundary, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetMaximum(objectstore.GoogleCloudStorageObjectMaximumBytes)},
		{name: "boundary one above provider maximum is refused", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetMaximum(objectstore.GoogleCloudStorageObjectMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
		{name: "boundary exact resolved filesystem component ceiling is admitted", class: boundary, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetResolvedComponentBoundary(0)},
		{name: "boundary one above resolved filesystem component ceiling is refused", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetResolvedComponentBoundary(1), wantErr: core.ErrPrimitiveContract},
		{name: "boundary dot source is refused as no file", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetSource("."), wantErr: core.ErrTaskManagerContract},
		{name: "boundary absolute source is refused before root crossing", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetSource("/proof.bin"), wantErr: core.ErrPrimitiveContract},
		{name: "boundary parent source is refused before root crossing", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationSetSource("../proof.bin"), wantErr: core.ErrPrimitiveContract},
		{name: "boundary zero instant is refused before source I/O", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationZeroInstant, wantErr: core.ErrTemporalContract},
		{name: "boundary zero media type is refused before source I/O", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationZeroMedia, wantErr: core.ErrPrimitiveContract},
		{name: "boundary zero task identity is refused before source I/O", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationZeroTask, wantErr: core.ErrIDContract},
		{name: "boundary zero expected revision is refused before source I/O", class: boundary, context: active, disposition: evidenceSourceMissing, mutate: evidencePreparationZeroRevision, wantErr: core.ErrTaskManagerContract},
		{name: "boundary quoted media parameter survives projection", class: boundary, context: active, disposition: evidenceSourceRegular, mutate: evidencePreparationSetMedia(`text/plain; note="a;b"`)},
	}
}

func evidencePreparationSetSource(value string) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(_ testing.TB, request *taskEvidencePreparationRequest, scenario *evidenceSourceScenarioRequest) {
		request.Input.Source = value
		scenario.Source = value
	}
}

func evidencePreparationSetResolvedComponentBoundary(delta int) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(t testing.TB, request *taskEvidencePreparationRequest, scenario *evidenceSourceScenarioRequest) {
		t.Helper()
		rootComponents := len(strings.Split(strings.Trim(request.WorkingDirectory.String(), string(filepath.Separator)), string(filepath.Separator)))
		sourceComponents := core.FilesystemPathMaximumComponents - rootComponents + delta
		if sourceComponents < 1 {
			t.Fatalf("resolved source component budget = %d, want positive", sourceComponents)
		}
		value := evidencePathComponents(sourceComponents)
		request.Input.Source = value
		scenario.Source = value
	}
}

func evidencePreparationSetExtent(payloadBytes, maximumBytes uint64) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(t testing.TB, request *taskEvidencePreparationRequest, scenario *evidenceSourceScenarioRequest) {
		t.Helper()
		scenario.Payload = []byte(strings.Repeat("e", int(payloadBytes)))
		request.Configuration.EvidenceStorage.MaximumBytes = evidencePreparationByteCount(t, maximumBytes)
	}
}

func evidencePreparationSetMaximum(maximumBytes uint64) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(t testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
		t.Helper()
		if maximumBytes == 0 {
			request.Configuration.EvidenceStorage.MaximumBytes = core.ByteCount{}
			return
		}
		request.Configuration.EvidenceStorage.MaximumBytes = evidencePreparationByteCount(t, maximumBytes)
	}
}

func evidencePreparationOneByteOversize(t testing.TB, request *taskEvidencePreparationRequest, scenario *evidenceSourceScenarioRequest) {
	t.Helper()
	request.Configuration.EvidenceStorage.MaximumBytes = evidencePreparationByteCount(t, uint64(len(scenario.Payload)-1))
}

func evidencePreparationSetBucket(value string) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
		request.Configuration.EvidenceStorage.Bucket = value
	}
}

func evidencePreparationSetPrefix(value string) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
		request.Configuration.EvidenceStorage.Prefix = value
	}
}

func evidencePreparationSetMedia(value string) func(testing.TB, *taskEvidencePreparationRequest, *evidenceSourceScenarioRequest) {
	return func(t testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
		t.Helper()
		mediaType, err := core.ParseHTTPMediaType(value)
		if err != nil {
			t.Fatalf("core.ParseHTTPMediaType(%q) error = %v, want nil", value, err)
		}
		request.Input.ContentType = mediaType
	}
}

func evidencePreparationZeroRoot(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
	request.WorkingDirectory = core.AbsolutePath{}
}

func evidencePreparationRemoveStorage(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
	request.Configuration.EvidenceStorage = nil
}

func evidencePreparationZeroInstant(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
	request.Instant = temporal.Instant{}
}

func evidencePreparationZeroMedia(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
	request.Input.ContentType = core.HTTPMediaType{}
}

func evidencePreparationZeroTask(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
	request.Input.TaskID = id.UUIDv7{}
}

func evidencePreparationZeroRevision(_ testing.TB, request *taskEvidencePreparationRequest, _ *evidenceSourceScenarioRequest) {
	request.Input.ExpectedRevision = taskmanager.Revision{}
}

func evidencePreparationByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	count, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return count
}

func taskEvidenceTreeInventory(t testing.TB, roots ...string) []string {
	t.Helper()
	var inventory []string
	for rootIndex, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			fact := fmt.Sprintf("%d:%s:%s:%d", rootIndex, relative, info.Mode(), info.Size())
			if info.Mode().IsRegular() {
				bytes, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				digest, err := core.SHA256Of(bytes).Hex()
				if err != nil {
					return err
				}
				fact += ":" + digest
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Readlink(path)
				if err != nil {
					return err
				}
				fact += ":" + target
			}
			inventory = append(inventory, fact)
			return nil
		})
		if err != nil {
			t.Fatalf("filepath.WalkDir(%q) error = %v, want nil", root, err)
		}
	}
	return inventory
}

type evidenceSourceScenarioRequest struct {
	Root         core.AbsolutePath
	ExternalRoot string
	Source       string
	Payload      []byte
	Disposition  evidenceSourceDisposition
}

func prepareEvidenceSourceScenario(t testing.TB, request evidenceSourceScenarioRequest) {
	t.Helper()
	path := filepath.Join(request.Root.String(), request.Source)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(source parent) error = %v, want nil", err)
	}
	switch request.Disposition {
	case evidenceSourceMissing:
		return
	case evidenceSourceEmpty:
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("os.WriteFile(empty source) error = %v, want nil", err)
		}
	case evidenceSourceDirectory:
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatalf("os.Mkdir(source) error = %v, want nil", err)
		}
	case evidenceSourceConfinedLink:
		target := filepath.Join(request.Root.String(), "evidence", "target.png")
		if err := os.WriteFile(target, request.Payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(confined target) error = %v, want nil", err)
		}
		if err := os.Symlink("target.png", path); err != nil {
			t.Fatalf("os.Symlink(confined target) error = %v, want nil", err)
		}
	case evidenceSourceEscapingLink:
		external := filepath.Join(request.ExternalRoot, "external.png")
		if err := os.WriteFile(external, request.Payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(external target) error = %v, want nil", err)
		}
		if err := os.Symlink(external, path); err != nil {
			t.Fatalf("os.Symlink(external target) error = %v, want nil", err)
		}
	case evidenceSourceBrokenLink:
		if err := os.Symlink("absent-target.png", path); err != nil {
			t.Fatalf("os.Symlink(absent target) error = %v, want nil", err)
		}
	case evidenceSourceHardLink:
		target := filepath.Join(request.Root.String(), "evidence", "hard-link-target.png")
		if err := os.WriteFile(target, request.Payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(hard-link target) error = %v, want nil", err)
		}
		if err := os.Link(target, path); err != nil {
			t.Fatalf("os.Link(hard-link target) error = %v, want nil", err)
		}
	default:
		if err := os.WriteFile(path, request.Payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(source) error = %v, want nil", err)
		}
	}
}

func provePreparationCreatedNoObjectReceipt(t testing.TB, got taskEvidenceUploadPlan) {
	t.Helper()
	if got.Name.String() != "" || got.Bucket.String() != "" || got.Integrity != (objectstore.Integrity{}) {
		t.Fatalf("refused preparation retained upload facts = %+v, want zero", got)
	}
}

func taskEvidenceConfigurationFixture(t testing.TB, maximumValue uint64) configurationDocument {
	t.Helper()
	authority, err := core.ParseHTTPEndpoint("https://admin.example.com")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	identity, err := exchange.ParseBasicAuthorizationIdentity("agent")
	if err != nil {
		t.Fatalf("exchange.ParseBasicAuthorizationIdentity() error = %v, want nil", err)
	}
	maximum, err := core.NewByteCount(maximumValue)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", maximumValue, err)
	}
	projectID := commandUUIDFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6")
	return configurationDocument{
		Revision: commandDocumentRevisionV2, Authority: authority, Username: identity, ProjectID: &projectID,
		PasswordSecret: googleSecretReference{Project: "example-task-project", Secret: "task-manager-admin-password"},
		EvidenceStorage: &evidenceStorageReference{
			Bucket: "example-task-proof", Prefix: "task-evidence/", MaximumBytes: maximum,
		},
	}
}
