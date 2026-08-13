package hostfacts

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type hostfactsOffWireEnum interface {
	comparable
	core.OffWireEnum
	IsValid() bool
	String() string
}

func TestHostfactsOffWireEnumsExhaustClosedDomains(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func(*testing.T)
		name string
	}{
		{name: "operations reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) Operation { return Operation(raw) }, []Operation{
				OperationOpenRoot,
				OperationDiskCapacity,
				OperationGoMemory,
				OperationPhysicalMemory,
				OperationCgroupMembership,
				OperationCgroupMount,
				OperationCgroupLimit,
				OperationTreeWalk,
				OperationGoOOMBanner,
				OperationTerminalGeometry,
				OperationDiskRotation,
				OperationLogicalCPUCount,
			})
		}},
		{name: "disk rotations reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) DiskRotation { return DiskRotation(raw) }, []DiskRotation{
				DiskRotationUnsupported,
				DiskRotationUnavailable,
				DiskRotationRotational,
				DiskRotationNonRotational,
			})
		}},
		{name: "terminal attachments reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) TerminalAttachment { return TerminalAttachment(raw) }, []TerminalAttachment{
				TerminalAttachmentTerminal,
				TerminalAttachmentNotTerminal,
				TerminalAttachmentTerminalWithoutGeometry,
			})
		}},
		{name: "disk pressure states reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) DiskPressureState { return DiskPressureState(raw) }, []DiskPressureState{
				DiskPressureDisabled,
				DiskPressureHealthy,
				DiskPressureReached,
			})
		}},
		{name: "memory pressure states reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) MemoryPressureState { return MemoryPressureState(raw) }, []MemoryPressureState{
				MemoryPressureDisabled,
				MemoryPressureHealthy,
				MemoryPressureReached,
			})
		}},
		{name: "missing path policies reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) MissingPathPolicy { return MissingPathPolicy(raw) }, []MissingPathPolicy{
				MissingPathReject,
				MissingPathIsEmpty,
			})
		}},
		{name: "workload limit states reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) WorkloadMemoryLimitState { return WorkloadMemoryLimitState(raw) }, []WorkloadMemoryLimitState{
				WorkloadMemoryLimitUnsupported,
				WorkloadMemoryLimitUnavailable,
				WorkloadMemoryLimitUnlimited,
				WorkloadMemoryLimitLimited,
			})
		}},
		{name: "workload limit sources reject every unadmitted uint8 value", run: func(t *testing.T) {
			proveHostfactsOffWireEnum(t, func(raw uint8) WorkloadMemoryLimitSource { return WorkloadMemoryLimitSource(raw) }, []WorkloadMemoryLimitSource{
				WorkloadMemoryLimitSourceNone,
				WorkloadMemoryLimitSourceCgroupV2,
				WorkloadMemoryLimitSourceCgroupV1,
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func proveHostfactsOffWireEnum[T hostfactsOffWireEnum](
	t *testing.T,
	fromRaw func(uint8) T,
	admitted []T,
) {
	t.Helper()

	wantAdmitted := make(map[T]struct{}, len(admitted))
	labels := make(map[string]T, len(admitted))
	unknownLabel := fromRaw(0).String()
	if unknownLabel == "" {
		t.Fatalf("%T zero String() = empty, want one safe diagnostic", fromRaw(0))
	}
	for _, value := range admitted {
		wantAdmitted[value] = struct{}{}
	}
	for raw := uint16(0); raw <= math.MaxUint8; raw++ {
		value := fromRaw(uint8(raw))
		_, wantValid := wantAdmitted[value]
		gotErr := value.Validate()
		if value.IsValid() != wantValid || (gotErr == nil) != wantValid {
			t.Fatalf(
				"%T(%d) validity = IsValid:%t Validate:%v, want %t",
				value,
				raw,
				value.IsValid(),
				gotErr,
				wantValid,
			)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrHostFactsContract) || value.String() != unknownLabel {
				t.Fatalf(
					"%T(%d) rejection = error:%v label:%q, want %v/%q",
					value,
					raw,
					gotErr,
					value.String(),
					core.ErrHostFactsContract,
					unknownLabel,
				)
			}
			continue
		}
		label := value.String()
		if label == "" || label == unknownLabel {
			t.Fatalf("%T(%d).String() = %q, want an admitted diagnostic", value, raw, label)
		}
		if prior, exists := labels[label]; exists {
			t.Fatalf("%T values %v and %v share label %q, want unique labels", value, prior, value, label)
		}
		labels[label] = value
	}
	proveHostfactsEnumStaysOffWire(t, admitted[0])
}

func proveHostfactsEnumStaysOffWire[T hostfactsOffWireEnum](t *testing.T, value T) {
	t.Helper()

	if _, implemented := any(value).(json.Marshaler); implemented {
		t.Fatalf("%T implements json.Marshaler, want an off-wire enum", value)
	}
	if _, implemented := any(&value).(json.Unmarshaler); implemented {
		t.Fatalf("*%T implements json.Unmarshaler, want an off-wire enum", value)
	}
}
