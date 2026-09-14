package permit

import (
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/id"
)

// A repository slug must never be assignable to the server's project identity.
// Runtime value tests alone cannot prove the compiler-visible nominal type.
func TestReportScopeUsesAuthorityIssuedProjectIdentity(t *testing.T) {
	t.Parallel()
	field, ok := reflect.TypeFor[ReportScope]().FieldByName("Project")
	want := reflect.TypeFor[id.ULID]()
	if !ok || field.Type != want {
		t.Fatalf("ReportScope.Project type = %v (present %v), want %v", field.Type, ok, want)
	}
}
