package standard

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCatalogSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact project package code and evidence records close", func(t *testing.T) {
		t.Parallel()
		catalog := fixtureCatalog(t)
		if gotErr := catalog.Validate(); gotErr != nil {
			t.Fatalf("Catalog.Validate() error = %v, want nil", gotErr)
		}
	})

	t.Run("negative project inventory cannot exceed package and unattributed facts", func(t *testing.T) {
		t.Parallel()
		catalog := fixtureCatalog(t)
		catalog.Project.Code.Inventory.Files++
		gotErr := catalog.Validate()
		if !errors.Is(gotErr, core.ErrStandardConflict) {
			t.Fatalf("Catalog.Validate() error = %v, want %v", gotErr, core.ErrStandardConflict)
		}
	})

	t.Run("neutral package without requests or observations reports exact zeros", func(t *testing.T) {
		t.Parallel()
		catalog := fixtureCatalog(t)
		got, gotErr := catalog.Packages[0].EvidenceSummary()
		want := EvidenceSummary{SurfaceCount: 1}
		if gotErr != nil || got != want {
			t.Fatalf("PackageSnapshot.EvidenceSummary() = (%+v, %v), want (%+v, nil)", got, gotErr, want)
		}
	})
}

func TestMarkdownReporterLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive project and package reports are nonvacuous and deterministic", func(t *testing.T) {
		t.Parallel()
		catalog := fixtureCatalog(t)
		var first bytes.Buffer
		var second bytes.Buffer
		if gotErr := WriteProjectMarkdown(&first, catalog.Project); gotErr != nil {
			t.Fatalf("WriteProjectMarkdown(first) error = %v, want nil", gotErr)
		}
		if gotErr := WriteProjectMarkdown(&second, catalog.Project); gotErr != nil {
			t.Fatalf("WriteProjectMarkdown(second) error = %v, want nil", gotErr)
		}
		if first.Len() == 0 || first.String() != second.String() {
			t.Fatalf("project report bytes = (%d, deterministic %t), want nonzero and true", first.Len(), first.String() == second.String())
		}
		first.Reset()
		if gotErr := WritePackageMarkdown(&first, catalog.Packages[0]); gotErr != nil {
			t.Fatalf("WritePackageMarkdown() error = %v, want nil", gotErr)
		}
		if first.Len() == 0 {
			t.Fatal("package report bytes = 0, want nonzero")
		}
	})

	t.Run("negative invalid record emits no report bytes", func(t *testing.T) {
		t.Parallel()
		catalog := fixtureCatalog(t)
		catalog.Packages[0].Code.Inventory.Files = 0
		var got bytes.Buffer
		gotErr := WritePackageMarkdown(&got, catalog.Packages[0])
		if !errors.Is(gotErr, core.ErrStandardContract) || got.Len() != 0 {
			t.Fatalf("WritePackageMarkdown(invalid) = (%d bytes, %v), want (0, errors.Is(..., %v))", got.Len(), gotErr, core.ErrStandardContract)
		}
	})

	t.Run("neutral absent evidence and source analysis remain explicit", func(t *testing.T) {
		t.Parallel()
		catalog := fixtureCatalog(t)
		var got bytes.Buffer
		if gotErr := WritePackageMarkdown(&got, catalog.Packages[0]); gotErr != nil {
			t.Fatalf("WritePackageMarkdown(neutral) error = %v, want nil", gotErr)
		}
		if !strings.Contains(got.String(), "**Requests:** 0") || !strings.Contains(got.String(), "No source-analysis observation.") {
			t.Fatalf("neutral package report = %q, want explicit zero requests and absent source analysis", got.String())
		}
	})
}
