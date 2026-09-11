package github

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"math"
	"strings"
	"testing"
)

func TestGitHubNominalCustodyBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input                    string
		wantReferenceErr, wantAgentErr error
	}{
		{name: "empty has no nominal identity", wantReferenceErr: core.ErrGitHubContract, wantAgentErr: core.ErrGitHubContract},
		{name: "one ASCII byte is a complete nominal value", input: "x"},
		{name: "agent one below custody", input: strings.Repeat("x", 255)},
		{name: "agent exact custody", input: strings.Repeat("x", 256)},
		{name: "agent one above custody remains a reference", input: strings.Repeat("x", 257), wantAgentErr: core.ErrGitHubContract},
		{name: "reference one below custody", input: strings.Repeat("x", 1023), wantAgentErr: core.ErrGitHubContract},
		{name: "reference exact custody", input: strings.Repeat("x", 1024), wantAgentErr: core.ErrGitHubContract},
		{name: "reference one above custody", input: strings.Repeat("x", 1025), wantReferenceErr: core.ErrGitHubContract, wantAgentErr: core.ErrGitHubContract},
		{name: "reference preserves padding but HTTP identity refuses it", input: " x ", wantAgentErr: core.ErrGitHubContract},
		{name: "control bytes are never nominal identity", input: "x\x00", wantReferenceErr: core.ErrGitHubContract, wantAgentErr: core.ErrGitHubContract},
		{name: "reference requires UTF8 while HTTP identity preserves obs text", input: string([]byte{255}), wantReferenceErr: core.ErrGitHubContract},
		{name: "multibyte identity custody counts bytes", input: strings.Repeat("é", 128)},
		{name: "multibyte agent one byte above custody", input: strings.Repeat("é", 128) + "x", wantAgentErr: core.ErrGitHubContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ref, refErr := ParseReference(tc.input)
			agent, agentErr := ParseUserAgent(tc.input)
			if !errors.Is(refErr, tc.wantReferenceErr) || !errors.Is(agentErr, tc.wantAgentErr) {
				t.Fatalf("nominal errors=%v/%v, want %v/%v", refErr, agentErr, tc.wantReferenceErr, tc.wantAgentErr)
			}
			if refErr == nil {
				if ref.String() != tc.input {
					t.Fatalf("reference=%q, want %q", ref.String(), tc.input)
				}
			} else if ref != (Reference{}) {
				t.Fatalf("refusal reference=%+v, want zero", ref)
			}
			if agentErr == nil {
				if agent.String() != tc.input {
					t.Fatalf("agent=%q, want %q", agent.String(), tc.input)
				}
			} else if agent != (UserAgent{}) {
				t.Fatalf("refusal agent=%+v, want zero", agent)
			}
		})
	}
}
func TestTagPageSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	repo := parsedRepository(t, "owner/repository")
	ref, err := ParseReference("v1")
	if err != nil {
		t.Fatal(err)
	}
	tag := Tag{Name: ref, Commit: parsedCommit(t)}
	cases := []struct {
		name                                          string
		page, next                                    uint32
		count                                         int
		missingRepository, missingName, missingCommit bool
		wantErr                                       error
	}{
		{name: "empty first page preserves neutral observation", page: 1},
		{name: "nonempty page preserves one exact next page", page: 1, next: 2, count: 1},
		{name: "last representable page can finish", page: math.MaxUint32},
		{name: "penultimate page can nominate maximum page", page: math.MaxUint32 - 1, next: math.MaxUint32},
		{name: "page zero cannot be an observation", wantErr: core.ErrGitHubResponse},
		{name: "same page cannot be next", page: 1, next: 1, wantErr: core.ErrGitHubResponse},
		{name: "next cannot skip a page", page: 1, next: 3, wantErr: core.ErrGitHubResponse},
		{name: "maximum page cannot wrap to first", page: math.MaxUint32, next: 1, wantErr: core.ErrGitHubResponse},
		{name: "one below provider count ceiling", page: 1, count: 99},
		{name: "exact provider count ceiling", page: 1, count: 100},
		{name: "one above provider count ceiling", page: 1, count: 101, wantErr: core.ErrGitHubResponse},
		{name: "missing repository cannot bind page", page: 1, missingRepository: true, wantErr: core.ErrGitHubResponse},
		{name: "missing tag name cannot be provider fact", page: 1, count: 1, missingName: true, wantErr: core.ErrGitHubResponse},
		{name: "missing tag commit cannot be provider fact", page: 1, count: 1, missingCommit: true, wantErr: core.ErrGitHubResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			page := TagPage{Repository: repo, Page: tc.page, NextPage: tc.next}
			for range tc.count {
				page.Tags = append(page.Tags, tag)
			}
			if tc.missingRepository {
				page.Repository = Repository{}
			}
			if tc.missingName {
				page.Tags[0].Name = Reference{}
			}
			if tc.missingCommit {
				page.Tags[0].Commit = core.BuildCommit{}
			}
			if err := page.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("page Validate()=%v, want %v", err, tc.wantErr)
			}
		})
	}
}
