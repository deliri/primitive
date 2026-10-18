package process

import (
	"context"
	"io"
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
		QuotePOSIXArgument   nativeFuzzDoor[func(Argument) (Argument, error)]
		WriteServiceDocument nativeFuzzDoor[func(context.Context, io.Writer, ServiceDocumentRequest) error]
		Plan_UnmarshalJSON   nativeFuzzDoor[func(*Plan, []byte) error]
		snapshotSighting     nativeFuzzDoor[func(ProcessIdentity, string) (ProcessSighting, bool)]
	}{
		QuotePOSIXArgument:   nativeFuzzDoor[func(Argument) (Argument, error)]{Call: QuotePOSIXArgument, Fuzz: FuzzPOSIXArgumentSemanticClosure},
		WriteServiceDocument: nativeFuzzDoor[func(context.Context, io.Writer, ServiceDocumentRequest) error]{Call: WriteServiceDocument, Fuzz: FuzzServiceDocumentProjection},
		Plan_UnmarshalJSON:   nativeFuzzDoor[func(*Plan, []byte) error]{Call: (*Plan).UnmarshalJSON, Fuzz: FuzzPlanJSONExternalIngress},
		snapshotSighting:     nativeFuzzDoor[func(ProcessIdentity, string) (ProcessSighting, bool)]{Call: snapshotSighting, Fuzz: FuzzSnapshotSightingNativeIngress},
	}
	var names []string
	for field := range reflect.TypeOf(inventory).Fields() {
		names = append(names, field.Name)
	}
	slices.Sort(names)
	return names
}
