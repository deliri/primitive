package reviewcontrol

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
)

func TestReviewSubjectRelationshipLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact project source package and file relationships close", func(t *testing.T) {
		t.Parallel()
		if gotErr := reviewSubject(t).Validate(); gotErr != nil {
			t.Fatalf("Subject.Validate(exact relationships) error = %v, want nil", gotErr)
		}
	})

	t.Run("negative project and source repository disagreement is refused", func(t *testing.T) {
		t.Parallel()
		got := reviewSubject(t)
		other, err := projectstandards.NewRepositoryIdentity("github.com/deliri/other")
		if err != nil {
			t.Fatalf("NewRepositoryIdentity(other) error = %v, want nil", err)
		}
		got.Source.Repository = other
		if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrReviewControlSubjectMismatch) {
			t.Fatalf("Subject.Validate(repository disagreement) error = %v, want %v", gotErr, core.ErrReviewControlSubjectMismatch)
		}
	})

	t.Run("neutral zero subject creates no admitted source identity", func(t *testing.T) {
		t.Parallel()
		got := Subject{}
		if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrReviewControlContract) || got != (Subject{}) {
			t.Fatalf("Subject.Validate(zero) = (%+v, %v), want unchanged zero and %v", got, gotErr, core.ErrReviewControlContract)
		}
	})
}

func TestSourceLocationExhaustsPointSpanAndApproximationBoundaries(t *testing.T) {
	t.Parallel()
	file := reviewSubject(t).File
	cases := []struct {
		name      string
		location  SourceLocation
		wantValid bool
	}{
		{name: "whole file is an exact location", location: SourceLocation{Path: file}, wantValid: true},
		{name: "whole file may be approximate", location: SourceLocation{Path: file, Approximate: true}, wantValid: true},
		{name: "line without column is admitted", location: SourceLocation{Path: file, Line: 1}, wantValid: true},
		{name: "line and column point is admitted", location: SourceLocation{Path: file, Line: 1, Column: 1}, wantValid: true},
		{name: "same line span is admitted", location: SourceLocation{Path: file, Line: 1, Column: 1, EndLine: 1, EndColumn: 2}, wantValid: true},
		{name: "multi line span is admitted", location: SourceLocation{Path: file, Line: 1, Column: 9, EndLine: 2, EndColumn: 1}, wantValid: true},
		{name: "column without line is refused", location: SourceLocation{Path: file, Column: 1}},
		{name: "end line without start line is refused", location: SourceLocation{Path: file, EndLine: 1, EndColumn: 1}},
		{name: "end column without end line is refused", location: SourceLocation{Path: file, Line: 1, EndColumn: 1}},
		{name: "end line without end column is refused", location: SourceLocation{Path: file, Line: 1, Column: 1, EndLine: 1}},
		{name: "span ending before start line is refused", location: SourceLocation{Path: file, Line: 2, Column: 1, EndLine: 1, EndColumn: 9}},
		{name: "span ending before start column is refused", location: SourceLocation{Path: file, Line: 2, Column: 2, EndLine: 2, EndColumn: 1}},
		{name: "span requires an exact start column", location: SourceLocation{Path: file, Line: 1, EndLine: 2, EndColumn: 1}},
		{name: "missing path is refused even for whole document location", location: SourceLocation{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.location.Validate()
			if (gotErr == nil) != tc.wantValid {
				t.Fatalf("SourceLocation.Validate(%+v) error = %v, want valid=%t", tc.location, gotErr, tc.wantValid)
			}
		})
	}
}

func TestContractRejectsDuplicateProofRequirementIdentity(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	packet.Contract.RequiredProof = append(packet.Contract.RequiredProof, packet.Contract.RequiredProof[0])
	if gotErr := packet.Contract.Validate(); !errors.Is(gotErr, core.ErrReviewControlContract) {
		t.Fatalf("Contract.Validate(duplicate proof rule) error = %v, want %v", gotErr, core.ErrReviewControlContract)
	}
}
