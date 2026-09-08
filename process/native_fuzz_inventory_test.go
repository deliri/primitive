package process

import (
	"reflect"
	"slices"
	"testing"
)

type nativeFuzzDoor[Call any] struct {
	Call Call
	Fuzz func(*testing.F)
}

// Exported only in the test build so the external inventory can consume the
// names of these compiler-bound native and private-wire proofs.
func ProcessNativeFuzzDoorNames() []string {
	inventory := struct {
		Plan_UnmarshalJSON nativeFuzzDoor[func(*Plan, []byte) error]
		snapshotSighting   nativeFuzzDoor[func(ProcessIdentity, string) (ProcessSighting, bool)]
	}{
		Plan_UnmarshalJSON: nativeFuzzDoor[func(*Plan, []byte) error]{Call: (*Plan).UnmarshalJSON, Fuzz: FuzzPlanJSONExternalIngress},
		snapshotSighting:   nativeFuzzDoor[func(ProcessIdentity, string) (ProcessSighting, bool)]{Call: snapshotSighting, Fuzz: FuzzSnapshotSightingNativeIngress},
	}
	var names []string
	for field := range reflect.TypeOf(inventory).Fields() {
		names = append(names, field.Name)
	}
	slices.Sort(names)
	return names
}
