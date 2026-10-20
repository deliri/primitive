package gotoolchain

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestModuleFileProjectionPreservesSourceRefusalsAtEveryBoundary(t *testing.T) {
	t.Parallel()
	for index, prefix := range []string{"", `{`, `{"Go":"1.27.2"`, `{"Replace":[`, `{"Replace":[{}`, `{"Replace":[]`, `{"Require":[`, `{"Go":"1.27.2"}`} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			got, err := readModuleFileObservation(t.Context(), io.MultiReader(strings.NewReader(prefix), moduleFileFailingReader{}))
			if got != (ModuleFileObservation{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("native module source boundary%d result%+v/%v want no observation and both source/output identities", index, got, err)
			}
		})
	}
}
