package release

import (
	"math"
	"testing"
)

func TestPublicationRoleAtExhaustsSlotsAndIntegerEdges(t *testing.T) {
	t.Parallel()

	seen := make(map[PublicationRole]struct{}, PublicationObjectCount)
	for index := range PublicationObjectCount {
		role, ok := PublicationRoleAt(index)
		if !ok || role.Validate() != nil {
			t.Fatalf("PublicationRoleAt(%d) = (%v, %t), want one valid role", index, role, ok)
		}
		gotIndex, gotErr := role.Index()
		if gotErr != nil || gotIndex != index {
			t.Fatalf("PublicationRoleAt(%d).Index() = (%d, %v), want (%d, nil)", index, gotIndex, gotErr, index)
		}
		if _, duplicate := seen[role]; duplicate {
			t.Fatalf("PublicationRoleAt(%d) repeated role %v", index, role)
		}
		seen[role] = struct{}{}
	}
	if len(seen) != PublicationObjectCount {
		t.Fatalf("PublicationRoleAt valid-domain cardinality = %d, want %d", len(seen), PublicationObjectCount)
	}

	for _, index := range []int{math.MinInt, -PublicationObjectCount - 1, -1, PublicationObjectCount, PublicationObjectCount + 1, math.MaxInt} {
		got, ok := PublicationRoleAt(index)
		if got != PublicationRoleUnknown || ok {
			t.Fatalf("PublicationRoleAt(%d) = (%v, %t), want (%v, false)", index, got, ok, PublicationRoleUnknown)
		}
	}
}
