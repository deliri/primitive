package hostfacts

import (
	"encoding/json"

	"github.com/deliri/primitive/v2026/core"
)

var (
	_ core.Validatable = Operation(0)
	_ core.Validatable = Failure{}

	_ core.Validatable = DiskPressurePolicy{}
	_ core.Validatable = DiskAssessmentRequest{}
	_ core.Validatable = DiskCapacity{}
	_ core.Validatable = DiskPressureState(0)
	_ core.Validatable = DiskAssessment{}
	_ core.Validatable = DiskRotation(0)
	_ core.Validatable = DiskRotationRequest{}

	_ core.Validatable = Percent{}
	_ core.Validatable = GoMemoryPressurePolicy{}
	_ core.Validatable = GoMemoryAssessmentRequest{}
	_ core.Validatable = GoMemorySnapshot{}
	_ core.Validatable = MemoryPressureState(0)
	_ core.Validatable = GoMemoryAssessment{}
	_ core.Validatable = PhysicalMemory{}
	_ core.Validatable = LogicalCPUCount{}

	_ core.Validatable = WorkloadMemoryLimitState(0)
	_ core.Validatable = WorkloadMemoryLimitSource(0)
	_ core.Validatable = WorkloadMemoryLimit{}

	_ core.Validatable = MissingPathPolicy(0)
	_ core.Validatable = TreeUsageRequest{}
	_ core.Validatable = RegularFileCount{}
	_ core.Validatable = TreeUsage{}

	_ core.Validatable = TerminalColumns(0)
	_ core.Validatable = TerminalAttachment(0)
	_ core.Validatable = TerminalGeometryRequest{}
	_ core.Validatable = TerminalGeometry{}

	_ core.Validatable            = GoOOMBannerState(0)
	_ core.Validatable            = GoOOMBannerRequest{}
	_ core.Validatable            = GoOOMBannerEvidence{}
	_ core.Validatable            = goOOMBannerWire{}
	_ core.ValidatedJSONMarshaler = GoOOMBannerState(0)
	_ core.ValidatedJSONMarshaler = GoOOMBannerEvidence{}

	_ core.OffWireEnum = OperationUnknown
	_ core.OffWireEnum = DiskPressureUnknown
	_ core.OffWireEnum = DiskRotationUnknown
	_ core.OffWireEnum = MemoryPressureUnknown
	_ core.OffWireEnum = WorkloadMemoryLimitUnknown
	_ core.OffWireEnum = WorkloadMemoryLimitSourceUnknown
	_ core.OffWireEnum = MissingPathUnknown
	_ core.OffWireEnum = TerminalAttachmentUnknown
	_ core.OffWireEnum = TerminalColumns(0)

	_ json.Marshaler   = GoOOMBannerState(0)
	_ json.Unmarshaler = (*GoOOMBannerState)(nil)
	_ json.Marshaler   = GoOOMBannerEvidence{}
	_ json.Unmarshaler = (*GoOOMBannerEvidence)(nil)
)
