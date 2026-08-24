package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/taskmanager"
)

func TestShippedTaskCommandExamplesAreExecutableContracts(t *testing.T) {
	t.Parallel()
	workingDirectory, err := process.WorkingDirectory()
	if err != nil {
		t.Fatalf("process.WorkingDirectory() error = %v, want nil", err)
	}
	exampleDirectory, err := workingDirectory.ResolveText("../../examples/taskmanager")
	if err != nil {
		t.Fatalf("working directory ResolveText(examples) error = %v, want nil", err)
	}
	configurationPath, err := exampleDirectory.Resolve(configurationFileName)
	if err != nil {
		t.Fatalf("example directory Resolve(configuration) error = %v, want nil", err)
	}
	configuration, err := readDocument[configurationDocument](t.Context(), configurationPath, configurationMaxBytes)
	if err != nil {
		t.Fatalf("readDocument(configuration example) error = %v, want nil", err)
	}
	if err := configuration.Validate(); err != nil {
		t.Fatalf("configuration example Validate() error = %v, want nil", err)
	}

	cases := []struct {
		name      string
		file      string
		operation operation
	}{
		{name: "list active projects", file: "list_projects.json", operation: operationListProjects},
		{name: "get configured project", file: "get_project.json", operation: operationGetProject},
		{name: "create project", file: "create_project.json", operation: operationCreateProject},
		{name: "list phases", file: "list_phases.json", operation: operationListPhases},
		{name: "create phase", file: "create_phase.json", operation: operationCreatePhase},
		{name: "list active tasks", file: "list_active_tasks.json", operation: operationListTasks},
		{name: "list completed tasks", file: "list_completed_tasks.json", operation: operationListTasks},
		{name: "get task detail", file: "get_task.json", operation: operationGetTask},
		{name: "create task", file: "create_task.json", operation: operationCreateTask},
		{name: "update task", file: "update_task.json", operation: operationUpdateTask},
		{name: "complete task", file: "complete_task.json", operation: operationCompleteTask},
		{name: "list evidence", file: "list_evidence.json", operation: operationListEvidence},
		{name: "append object-store evidence", file: "append_evidence.json", operation: operationAppendEvidence},
		{name: "list Git commits", file: "list_git_commits.json", operation: operationListGitCommits},
		{name: "append Git commit", file: "append_git_commit.json", operation: operationAppendGitCommit},
	}
	var gotCoverage [operationLimit]uint8
	for _, tc := range cases {
		gotCoverage[tc.operation]++
	}
	wantCoverage := [operationLimit]uint8{
		operationListProjects: 1, operationGetProject: 1, operationCreateProject: 1,
		operationListPhases: 1, operationCreatePhase: 1, operationListTasks: 2,
		operationGetTask: 1, operationCreateTask: 1, operationUpdateTask: 1,
		operationCompleteTask: 1, operationListEvidence: 1, operationAppendEvidence: 1,
		operationListGitCommits: 1, operationAppendGitCommit: 1,
	}
	if gotCoverage != wantCoverage {
		t.Fatalf("shipped example operation coverage = %v, want %v", gotCoverage, wantCoverage)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, pathErr := exampleDirectory.Resolve(tc.file)
			if pathErr != nil {
				t.Fatalf("example directory Resolve(%s) error = %v, want nil", tc.file, pathErr)
			}
			job, readErr := readDocument[jobDocument](t.Context(), path, commandDocumentMaxBytes)
			if readErr != nil {
				t.Fatalf("readDocument(%s) error = %v, want nil", tc.file, readErr)
			}
			if job.Operation != tc.operation {
				t.Fatalf("job operation = %v, want %v", job.Operation, tc.operation)
			}
			encoded, encodeErr := job.MarshalJSON()
			if encodeErr != nil {
				t.Fatalf("job MarshalJSON() error = %v, want nil", encodeErr)
			}
			limits, limitsErr := documentLimits(commandDocumentMaxBytes)
			if limitsErr != nil {
				t.Fatalf("documentLimits() error = %v, want nil", limitsErr)
			}
			roundTrip, decodeErr := core.DecodeStrictJSON[jobDocument](bytes.NewReader(encoded), limits)
			if decodeErr != nil {
				t.Fatalf("DecodeStrictJSON(canonical job) error = %v, want nil", decodeErr)
			}
			second, secondErr := roundTrip.MarshalJSON()
			if secondErr != nil || !bytes.Equal(second, encoded) {
				t.Fatalf("second canonical job = (%s, %v), want (%s, nil)", second, secondErr, encoded)
			}
		})
	}
}

func TestJobDocumentRejectsMalformedAndAmbiguousIngress(t *testing.T) {
	t.Parallel()
	limits, err := documentLimits(commandDocumentMaxBytes)
	if err != nil {
		t.Fatalf("documentLimits() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data string
	}{
		{name: "empty", data: ""},
		{name: "null", data: "null"},
		{name: "array", data: "[]"},
		{name: "empty object", data: "{}"},
		{name: "unsupported revision", data: `{"revision":2,"operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":100}}`},
		{name: "unknown operation", data: `{"revision":1,"operation":"destroy_world","list_projects":{"lifecycle":"active","order":"descending","limit":100}}`},
		{name: "missing payload", data: `{"revision":1,"operation":"list_projects"}`},
		{name: "mismatched payload", data: `{"revision":1,"operation":"create_project","list_projects":{"lifecycle":"active","order":"descending","limit":100}}`},
		{name: "two payloads", data: `{"revision":1,"operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":100},"list_phases":{"order":"ascending","limit":100}}`},
		{name: "unknown field", data: `{"revision":1,"operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":100},"surprise":true}`},
		{name: "duplicate operation", data: `{"revision":1,"operation":"list_projects","operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":100}}`},
		{name: "case folded duplicate", data: `{"revision":1,"operation":"list_projects","Operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":100}}`},
		{name: "zero page limit", data: `{"revision":1,"operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":0}}`},
		{name: "completed typo", data: `{"revision":1,"operation":"list_tasks","list_tasks":{"collection":"complete","order":"descending","limit":100}}`},
		{name: "trailing document", data: `{"revision":1,"operation":"list_projects","list_projects":{"lifecycle":"active","order":"descending","limit":100}} {}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := core.DecodeStrictJSON[jobDocument](strings.NewReader(tc.data), limits)
			if gotErr == nil || (!errors.Is(gotErr, core.ErrJSONContract) && !errors.Is(gotErr, core.ErrTaskManagerContract)) {
				t.Fatalf("DecodeStrictJSON(rejected job) error = %v, want typed JSON or task-manager refusal", gotErr)
			}
			if got.Operation != operationUnknown || got.payloadCount() != 0 {
				t.Fatalf("DecodeStrictJSON(rejected job) = %+v, want zero", got)
			}
		})
	}
}

func TestCommandContentLimitsFailLocallyAtExactBoundaries(t *testing.T) {
	t.Parallel()
	type textCase struct {
		parse   func(string) (string, error)
		name    string
		value   string
		maximum int
	}
	cases := []textCase{
		{name: "title one below", value: strings.Repeat("t", taskmanager.TitleMaximumRunes-1), maximum: taskmanager.TitleMaximumRunes, parse: parseCommandTitle},
		{name: "title exact", value: strings.Repeat("t", taskmanager.TitleMaximumRunes), maximum: taskmanager.TitleMaximumRunes, parse: parseCommandTitle},
		{name: "title one above", value: strings.Repeat("t", taskmanager.TitleMaximumRunes+1), maximum: taskmanager.TitleMaximumRunes, parse: parseCommandTitle},
		{name: "description one below", value: strings.Repeat("d", taskmanager.DescriptionMaximumRunes-1), maximum: taskmanager.DescriptionMaximumRunes, parse: parseCommandDescription},
		{name: "description exact", value: strings.Repeat("d", taskmanager.DescriptionMaximumRunes), maximum: taskmanager.DescriptionMaximumRunes, parse: parseCommandDescription},
		{name: "description one above", value: strings.Repeat("d", taskmanager.DescriptionMaximumRunes+1), maximum: taskmanager.DescriptionMaximumRunes, parse: parseCommandDescription},
		{name: "evidence summary one below", value: strings.Repeat("e", taskmanager.EvidenceSummaryMaximumRunes-1), maximum: taskmanager.EvidenceSummaryMaximumRunes, parse: parseCommandEvidenceSummary},
		{name: "evidence summary exact", value: strings.Repeat("e", taskmanager.EvidenceSummaryMaximumRunes), maximum: taskmanager.EvidenceSummaryMaximumRunes, parse: parseCommandEvidenceSummary},
		{name: "evidence summary one above", value: strings.Repeat("e", taskmanager.EvidenceSummaryMaximumRunes+1), maximum: taskmanager.EvidenceSummaryMaximumRunes, parse: parseCommandEvidenceSummary},
		{name: "repository one below", value: strings.Repeat("r", taskmanager.RepositoryMaximumRunes-1), maximum: taskmanager.RepositoryMaximumRunes, parse: parseCommandRepository},
		{name: "repository exact", value: strings.Repeat("r", taskmanager.RepositoryMaximumRunes), maximum: taskmanager.RepositoryMaximumRunes, parse: parseCommandRepository},
		{name: "repository one above", value: strings.Repeat("r", taskmanager.RepositoryMaximumRunes+1), maximum: taskmanager.RepositoryMaximumRunes, parse: parseCommandRepository},
		{name: "commit summary one below", value: strings.Repeat("c", taskmanager.CommitSummaryMaximumRunes-1), maximum: taskmanager.CommitSummaryMaximumRunes, parse: parseCommandCommitSummary},
		{name: "commit summary exact", value: strings.Repeat("c", taskmanager.CommitSummaryMaximumRunes), maximum: taskmanager.CommitSummaryMaximumRunes, parse: parseCommandCommitSummary},
		{name: "commit summary one above", value: strings.Repeat("c", taskmanager.CommitSummaryMaximumRunes+1), maximum: taskmanager.CommitSummaryMaximumRunes, parse: parseCommandCommitSummary},
		{name: "title leading whitespace normalized", value: "  title", maximum: taskmanager.TitleMaximumRunes, parse: parseCommandTitle},
		{name: "title trailing whitespace normalized", value: "title  ", maximum: taskmanager.TitleMaximumRunes, parse: parseCommandTitle},
		{name: "description empty admitted", value: "", maximum: taskmanager.DescriptionMaximumRunes, parse: parseCommandDescription},
		{name: "title newline rejected", value: "title\nbody", maximum: taskmanager.TitleMaximumRunes, parse: parseCommandTitle},
		{name: "summary control rejected", value: "summary\x00", maximum: taskmanager.EvidenceSummaryMaximumRunes, parse: parseCommandEvidenceSummary},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := tc.parse(tc.value)
			wantErr := len([]rune(strings.TrimSpace(tc.value))) > tc.maximum ||
				(strings.ContainsAny(strings.TrimSpace(tc.value), "\x00\r\n"))
			if wantErr && !errors.Is(gotErr, core.ErrTaskManagerContract) {
				t.Fatalf("parse(boundary input) error = %v, want %v", gotErr, core.ErrTaskManagerContract)
			}
			if wantErr && got != "" {
				t.Fatalf("parse(rejected boundary input) result = %q, want zero", got)
			}
			if !wantErr && gotErr != nil {
				t.Fatalf("parse(boundary input) error = %v, want nil", gotErr)
			}
			if !wantErr && got != strings.TrimSpace(tc.value) {
				t.Fatalf("parse(accepted boundary input) result = %q, want normalized %q", got, strings.TrimSpace(tc.value))
			}
		})
	}
}

func parseCommandTitle(value string) (string, error) {
	parsed, err := commandTitle("title", value)
	return parsed.String(), err
}

func parseCommandDescription(value string) (string, error) {
	parsed, err := commandDescription("description", value)
	return parsed.String(), err
}

func parseCommandEvidenceSummary(value string) (string, error) {
	parsed, err := commandEvidenceSummary(value)
	return parsed.String(), err
}

func parseCommandRepository(value string) (string, error) {
	parsed, err := commandRepository(value)
	return parsed.String(), err
}

func parseCommandCommitSummary(value string) (string, error) {
	parsed, err := commandCommitSummary(value)
	return parsed.String(), err
}

func TestSchemaPublishesEveryCompilerOwnedAgentLimit(t *testing.T) {
	t.Parallel()
	schema := currentSchema()
	if err := schema.Validate(); err != nil {
		t.Fatalf("currentSchema().Validate() error = %v, want nil", err)
	}
	wantStdout, err := schema.MarshalJSON()
	if err != nil {
		t.Fatalf("currentSchema().MarshalJSON() error = %v, want nil", err)
	}
	wantStdout = append(wantStdout, '\n')
	var stdout, stderr bytes.Buffer
	gotExit := run(t.Context(), []string{schemaArgument}, commandStreams{
		stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr,
	})
	if gotExit != exitCodeSuccess {
		t.Fatalf("run(schema) exit = %v, want %v", gotExit, exitCodeSuccess)
	}
	if gotStdout := stdout.Bytes(); !bytes.Equal(gotStdout, wantStdout) {
		t.Fatalf("run(schema) stdout = %q, want %q", gotStdout, wantStdout)
	}
	if gotStderr, wantStderr := stderr.String(), ""; gotStderr != wantStderr {
		t.Fatalf("run(schema) stderr = %q, want %q", gotStderr, wantStderr)
	}
}
