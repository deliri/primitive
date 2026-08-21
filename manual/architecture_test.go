package manual_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestArchitectureDeclaresManualCoreEdge(t *testing.T) {
	t.Parallel()
	contract := core.DirectImportContract{Importer: core.PackageManual, Imported: core.PackageCore}
	if err := contract.Validate(); err != nil {
		t.Fatalf("DirectImportContract.Validate() error = %v, want nil", err)
	}
	if got := core.PrimitiveArchitecture().ContainsDirectImport(contract); !got {
		t.Fatalf("PrimitiveArchitecture().ContainsDirectImport() = %v, want true", got)
	}
}
