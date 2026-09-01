package projectstandards

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	MachineProbeSchemaVersion       uint16 = 1
	MachineFingerprintSchemaVersion uint16 = 1
	MachineToolchainMaximum                = 32
	MachineChangeMaximum                   = 64
)

type MachineIOPS uint64
type MachineBytesPerSecond uint64
type MachineBitsPerSecond uint64

func NewMachineIOPS(value uint64) (MachineIOPS, error) {
	candidate := MachineIOPS(value)
	return candidate, candidate.Validate()
}

func NewMachineBytesPerSecond(value uint64) (MachineBytesPerSecond, error) {
	candidate := MachineBytesPerSecond(value)
	return candidate, candidate.Validate()
}

func NewMachineBitsPerSecond(value uint64) (MachineBitsPerSecond, error) {
	candidate := MachineBitsPerSecond(value)
	return candidate, candidate.Validate()
}

func (r MachineIOPS) Validate() error {
	if r == 0 {
		return contractError(errors.New("project standards machine IOPS is zero"))
	}
	return nil
}

func (r MachineBytesPerSecond) Validate() error {
	if r == 0 {
		return contractError(errors.New("project standards machine byte rate is zero"))
	}
	return nil
}

func (r MachineBitsPerSecond) Validate() error {
	if r == 0 {
		return contractError(errors.New("project standards machine bit rate is zero"))
	}
	return nil
}

// MachineToolchainKind is the closed inventory emitted by a machine probe.
type MachineToolchainKind uint8

const (
	MachineToolchainUnknown MachineToolchainKind = iota
	MachineToolchainGo
	MachineToolchainBun
	MachineToolchainJava
	MachineToolchainGoogleCloudCLI
	MachineToolchainFirestoreEmulator
	MachineToolchainSQLite
	MachineToolchainPostgreSQL
	MachineToolchainGit
	machineToolchainLimit
)

func machineToolchainLabels() []string {
	return []string{"", "go", "bun", "java", "google_cloud_cli", "firestore_emulator", "sqlite", "postgresql", "git"}
}
func (k MachineToolchainKind) Validate() error {
	return validateEnum(uint8(k), machineToolchainLabels(), "project standards machine toolchain kind is invalid")
}
func (k MachineToolchainKind) IsValid() bool  { return k.Validate() == nil }
func (k MachineToolchainKind) String() string { return enumString(uint8(k), machineToolchainLabels()) }
func (k MachineToolchainKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), machineToolchainLabels(), "project standards machine toolchain kind is invalid")
}
func (k *MachineToolchainKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards machine toolchain kind receiver"))
	}
	value, err := unmarshalEnum(data, machineToolchainLabels(), "project standards machine toolchain kind is invalid")
	if err == nil {
		*k = MachineToolchainKind(value)
	}
	return err
}

type MachineInstallMode uint8

const (
	MachineInstallModeUnknown MachineInstallMode = iota
	MachineInstallModeInstalled
	MachineInstallModeBootService
	machineInstallModeLimit
)

func machineInstallModeLabels() []string { return []string{"", "installed", "boot_service"} }
func (m MachineInstallMode) Validate() error {
	return validateEnum(uint8(m), machineInstallModeLabels(), "project standards machine install mode is invalid")
}
func (m MachineInstallMode) IsValid() bool  { return m.Validate() == nil }
func (m MachineInstallMode) String() string { return enumString(uint8(m), machineInstallModeLabels()) }
func (m MachineInstallMode) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(m), machineInstallModeLabels(), "project standards machine install mode is invalid")
}
func (m *MachineInstallMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return jsonError(errors.New("nil project standards machine install mode receiver"))
	}
	value, err := unmarshalEnum(data, machineInstallModeLabels(), "project standards machine install mode is invalid")
	if err == nil {
		*m = MachineInstallMode(value)
	}
	return err
}

type MachineProvisioningModel uint8

const (
	MachineProvisioningUnknown MachineProvisioningModel = iota
	MachineProvisioningStandard
	MachineProvisioningSpot
	MachineProvisioningPreemptible
	machineProvisioningLimit
)

func machineProvisioningLabels() []string { return []string{"", "standard", "spot", "preemptible"} }
func (m MachineProvisioningModel) Validate() error {
	return validateEnum(uint8(m), machineProvisioningLabels(), "project standards machine provisioning model is invalid")
}
func (m MachineProvisioningModel) IsValid() bool { return m.Validate() == nil }
func (m MachineProvisioningModel) String() string {
	return enumString(uint8(m), machineProvisioningLabels())
}
func (m MachineProvisioningModel) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(m), machineProvisioningLabels(), "project standards machine provisioning model is invalid")
}
func (m *MachineProvisioningModel) UnmarshalJSON(data []byte) error {
	if m == nil {
		return jsonError(errors.New("nil project standards machine provisioning model receiver"))
	}
	value, err := unmarshalEnum(data, machineProvisioningLabels(), "project standards machine provisioning model is invalid")
	if err == nil {
		*m = MachineProvisioningModel(value)
	}
	return err
}

type MachineMaintenancePolicy uint8

const (
	MachineMaintenanceUnknown MachineMaintenancePolicy = iota
	MachineMaintenanceMigrate
	MachineMaintenanceTerminate
	machineMaintenanceLimit
)

func machineMaintenanceLabels() []string { return []string{"", "migrate", "terminate"} }
func (m MachineMaintenancePolicy) Validate() error {
	return validateEnum(uint8(m), machineMaintenanceLabels(), "project standards machine maintenance policy is invalid")
}
func (m MachineMaintenancePolicy) IsValid() bool { return m.Validate() == nil }
func (m MachineMaintenancePolicy) String() string {
	return enumString(uint8(m), machineMaintenanceLabels())
}
func (m MachineMaintenancePolicy) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(m), machineMaintenanceLabels(), "project standards machine maintenance policy is invalid")
}
func (m *MachineMaintenancePolicy) UnmarshalJSON(data []byte) error {
	if m == nil {
		return jsonError(errors.New("nil project standards machine maintenance policy receiver"))
	}
	value, err := unmarshalEnum(data, machineMaintenanceLabels(), "project standards machine maintenance policy is invalid")
	if err == nil {
		*m = MachineMaintenancePolicy(value)
	}
	return err
}

type MachineFingerprint struct {
	SchemaVersion uint16            `json:"schema_version"`
	SHA256        core.SHA256Digest `json:"sha256"`
}

func (f MachineFingerprint) Validate() error {
	if f.SchemaVersion != MachineFingerprintSchemaVersion {
		return contractError(errors.New("project standards machine fingerprint schema version is unsupported"))
	}
	return contractJoin(f.SHA256.Validate())
}

type MachineIdentity struct {
	ID          MachineID     `json:"id"`
	Provider    core.Offering `json:"provider"`
	Project     Identifier    `json:"project"`
	Instance    Identifier    `json:"instance"`
	Zone        Identifier    `json:"zone"`
	MachineType Identifier    `json:"machine_type"`
}

func (i MachineIdentity) Validate() error {
	return contractJoin(i.ID.Validate(), i.Provider.Validate(), i.Project.Validate(), i.Instance.Validate(), i.Zone.Validate(), i.MachineType.Validate())
}

type MachineCompute struct {
	CPUPlatform           Name           `json:"cpu_platform"`
	Processor             Name           `json:"processor"`
	Architecture          Name           `json:"architecture"`
	Virtualization        Name           `json:"virtualization"`
	VCPU                  uint16         `json:"vcpu"`
	Sockets               uint16         `json:"sockets"`
	CoresPerSocket        uint16         `json:"cores_per_socket"`
	ThreadsPerCore        uint16         `json:"threads_per_core"`
	NUMANodes             uint16         `json:"numa_nodes"`
	MemoryConfiguredBytes core.ByteCount `json:"memory_configured_bytes"`
	MemoryGuestBytes      core.ByteCount `json:"memory_guest_bytes"`
}

func (c MachineCompute) Validate() error {
	if err := contractJoin(c.CPUPlatform.Validate(), c.Processor.Validate(), c.Architecture.Validate(), c.Virtualization.Validate()); err != nil {
		return err
	}
	if c.VCPU == 0 || c.Sockets == 0 || c.CoresPerSocket == 0 || c.ThreadsPerCore == 0 || c.NUMANodes == 0 {
		return contractError(errors.New("project standards machine compute topology is invalid"))
	}
	configured, configuredErr := c.MemoryConfiguredBytes.Uint64()
	guest, guestErr := c.MemoryGuestBytes.Uint64()
	if err := contractJoin(configuredErr, guestErr); err != nil {
		return err
	}
	if guest > configured {
		return conflictError(errors.New("project standards machine compute memory is contradictory"))
	}
	return nil
}

type MachineSystem struct {
	OperatingSystem        Name `json:"operating_system"`
	OperatingSystemVersion Name `json:"operating_system_version"`
	OperatingSystemImage   Name `json:"operating_system_image"`
	Kernel                 Name `json:"kernel"`
}

func (s MachineSystem) Validate() error {
	return contractJoin(s.OperatingSystem.Validate(), s.OperatingSystemVersion.Validate(), s.OperatingSystemImage.Validate(), s.Kernel.Validate())
}

type MachineStorage struct {
	BootDiskType             Name                  `json:"boot_disk_type"`
	Interface                Name                  `json:"interface"`
	Filesystem               Name                  `json:"filesystem"`
	PhysicalBlockBytes       core.ByteCount        `json:"physical_block_bytes"`
	CapacityBytes            core.ByteCount        `json:"capacity_bytes"`
	BaselineIOPS             MachineIOPS           `json:"baseline_iops"`
	BaselineReadBytes        MachineBytesPerSecond `json:"baseline_read_bytes_per_second"`
	InstanceCeilingIOPS      MachineIOPS           `json:"instance_ceiling_iops"`
	InstanceCeilingReadBytes MachineBytesPerSecond `json:"instance_ceiling_read_bytes_per_second"`
	LocalSSDCount            uint16                `json:"local_ssd_count"`
	SwapBytes                core.ByteLength       `json:"swap_bytes"`
}

func (s MachineStorage) Validate() error {
	if err := contractJoin(s.BootDiskType.Validate(), s.Interface.Validate(), s.Filesystem.Validate()); err != nil {
		return err
	}
	return contractJoin(s.PhysicalBlockBytes.Validate(), s.CapacityBytes.Validate(), s.BaselineIOPS.Validate(), s.BaselineReadBytes.Validate(), s.InstanceCeilingIOPS.Validate(), s.InstanceCeilingReadBytes.Validate(), s.SwapBytes.Validate())
}

type MachineNetwork struct {
	Interface         Name                 `json:"interface"`
	NetworkTier       Name                 `json:"network_tier"`
	Addressing        Name                 `json:"addressing"`
	VPC               Identifier           `json:"vpc"`
	MTU               uint32               `json:"mtu"`
	ReceiveQueues     uint16               `json:"receive_queues"`
	TransmitQueues    uint16               `json:"transmit_queues"`
	EgressFloorBits   MachineBitsPerSecond `json:"egress_floor_bits_per_second"`
	EgressCeilingBits MachineBitsPerSecond `json:"egress_ceiling_bits_per_second"`
}

func (n MachineNetwork) Validate() error {
	if err := contractJoin(n.Interface.Validate(), n.NetworkTier.Validate(), n.Addressing.Validate(), n.VPC.Validate()); err != nil {
		return err
	}
	if err := contractJoin(n.EgressFloorBits.Validate(), n.EgressCeilingBits.Validate()); err != nil {
		return err
	}
	if n.MTU == 0 || n.ReceiveQueues == 0 || n.TransmitQueues == 0 || n.EgressCeilingBits < n.EgressFloorBits {
		return conflictError(errors.New("project standards machine network bounds are contradictory"))
	}
	return nil
}

type MachineLifecycleSecurity struct {
	ProvisioningModel      MachineProvisioningModel `json:"provisioning_model"`
	StoppedWhenIdle        bool                     `json:"stopped_when_idle"`
	AutomaticRestart       bool                     `json:"automatic_restart"`
	HostMaintenance        MachineMaintenancePolicy `json:"host_maintenance"`
	SecureBoot             bool                     `json:"secure_boot"`
	VirtualTPM             bool                     `json:"virtual_tpm"`
	IntegrityMonitoring    bool                     `json:"integrity_monitoring"`
	ServiceAccountAttached bool                     `json:"service_account_attached"`
	IPForwarding           bool                     `json:"ip_forwarding"`
}

func (s MachineLifecycleSecurity) Validate() error {
	return contractJoin(s.ProvisioningModel.Validate(), s.HostMaintenance.Validate())
}

type MachineToolchain struct {
	Tool             MachineToolchainKind `json:"tool"`
	Version          Name                 `json:"version"`
	Platform         Name                 `json:"platform"`
	InstallMode      MachineInstallMode   `json:"install_mode"`
	ExecutableSHA256 core.SHA256Digest    `json:"executable_sha256"`
}

func (t MachineToolchain) Validate() error {
	return contractJoin(t.Tool.Validate(), t.Version.Validate(), t.Platform.Validate(), t.InstallMode.Validate(), t.ExecutableSHA256.Validate())
}

// MachineConfiguration contains only generation-defining observed facts.
// Volatile availability and uptime live in MachineRuntime.
type MachineConfiguration struct {
	Identity   MachineIdentity          `json:"identity"`
	Compute    MachineCompute           `json:"compute"`
	System     MachineSystem            `json:"system"`
	Storage    MachineStorage           `json:"storage"`
	Network    MachineNetwork           `json:"network"`
	Lifecycle  MachineLifecycleSecurity `json:"lifecycle_security"`
	Toolchains []MachineToolchain       `json:"toolchains"`
}

func (c MachineConfiguration) Validate() error {
	if err := contractJoin(c.Identity.Validate(), c.Compute.Validate(), c.System.Validate(), c.Storage.Validate(), c.Network.Validate(), c.Lifecycle.Validate()); err != nil {
		return err
	}
	if len(c.Toolchains) == 0 || len(c.Toolchains) > MachineToolchainMaximum {
		return contractError(errors.New("project standards machine toolchain inventory is invalid"))
	}
	for index := range c.Toolchains {
		if err := c.Toolchains[index].Validate(); err != nil {
			return err
		}
		if index > 0 && c.Toolchains[index-1].Tool >= c.Toolchains[index].Tool {
			return conflictError(errors.New("project standards machine toolchains are not canonical"))
		}
	}
	return nil
}

func (c MachineConfiguration) Fingerprint() (MachineFingerprint, error) {
	if err := c.Validate(); err != nil {
		return MachineFingerprint{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c)
	if err != nil {
		return MachineFingerprint{}, jsonError(err)
	}
	return MachineFingerprint{SchemaVersion: MachineFingerprintSchemaVersion, SHA256: core.SHA256Of(encoded)}, nil
}

type MachineRuntime struct {
	BootID               Identifier        `json:"boot_id"`
	Uptime               temporal.Duration `json:"uptime"`
	MemoryAvailableBytes core.ByteLength   `json:"memory_available_bytes"`
	DiskAvailableBytes   core.ByteLength   `json:"disk_available_bytes"`
	Address              Name              `json:"address"`
}

func (r MachineRuntime) ValidateFor(configuration MachineConfiguration) error {
	if err := contractJoin(r.BootID.Validate(), r.Uptime.Validate(), r.MemoryAvailableBytes.Validate(), r.DiskAvailableBytes.Validate(), r.Address.Validate()); err != nil {
		return err
	}
	guest, guestErr := configuration.Compute.MemoryGuestBytes.Uint64()
	capacity, capacityErr := configuration.Storage.CapacityBytes.Uint64()
	if err := contractJoin(guestErr, capacityErr); err != nil {
		return err
	}
	if r.Uptime.IsZero() || r.MemoryAvailableBytes.Uint64() > guest || r.DiskAvailableBytes.Uint64() > capacity {
		return conflictError(errors.New("project standards machine runtime is contradictory"))
	}
	return nil
}

// MachineProbeReport is the strict bounded document emitted by the probe
// script. IDs, time, collector authority, and fingerprints are added by the
// Go collector and cannot be asserted by the script.
type MachineProbeReport struct {
	SchemaVersion uint16               `json:"schema_version"`
	Configuration MachineConfiguration `json:"configuration"`
	Runtime       MachineRuntime       `json:"runtime"`
}

type MachineProbeExecution struct {
	Bash         core.AbsolutePath `json:"bash"`
	Script       core.AbsolutePath `json:"script"`
	ScriptDigest core.SHA256Digest `json:"script_digest"`
	ScriptBytes  core.ByteLength   `json:"script_bytes"`
	OutputLimit  core.ByteCount    `json:"output_limit"`
	ExitCode     int32             `json:"exit_code"`
	CPUTime      temporal.Duration `json:"cpu_time"`
	StdoutDigest core.SHA256Digest `json:"stdout_digest"`
	StdoutBytes  core.ByteLength   `json:"stdout_bytes"`
	StderrDigest core.SHA256Digest `json:"stderr_digest"`
	StderrBytes  core.ByteLength   `json:"stderr_bytes"`
}

func (e MachineProbeExecution) Validate() error {
	if err := contractJoin(e.Bash.Validate(), e.Script.Validate(), e.ScriptDigest.Validate(), e.ScriptBytes.Validate(), e.OutputLimit.Validate(), e.CPUTime.Validate(), e.StdoutDigest.Validate(), e.StdoutBytes.Validate(), e.StderrDigest.Validate(), e.StderrBytes.Validate()); err != nil {
		return err
	}
	if e.ScriptBytes.Uint64() == 0 {
		return conflictError(errors.New("project standards successful machine probe execution has an empty script"))
	}
	if e.ExitCode != 0 {
		return conflictError(errors.New("project standards successful machine probe execution has nonzero exit"))
	}
	return nil
}

func (r MachineProbeReport) Validate() error {
	if r.SchemaVersion != MachineProbeSchemaVersion {
		return contractError(errors.New("project standards machine probe schema version is unsupported"))
	}
	if err := r.Configuration.Validate(); err != nil {
		return err
	}
	return r.Runtime.ValidateFor(r.Configuration)
}

type machineProbeReportWire MachineProbeReport

func (r MachineProbeReport) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(machineProbeReportWire(r))
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (r *MachineProbeReport) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil project standards machine probe report receiver"))
	}
	wire, err := core.DecodeStrictJSONStructure[machineProbeReportWire](data, aboutJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := MachineProbeReport(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type MachineObservation struct {
	SchemaVersion uint16                `json:"schema_version"`
	ID            MachineObservationID  `json:"id"`
	GenerationID  MachineGenerationID   `json:"generation_id"`
	ObservedAt    temporal.Instant      `json:"observed_at"`
	Collector     EvidenceAuthority     `json:"collector"`
	Execution     MachineProbeExecution `json:"execution"`
	Configuration MachineConfiguration  `json:"configuration"`
	Runtime       MachineRuntime        `json:"runtime"`
	Fingerprint   MachineFingerprint    `json:"fingerprint"`
}

// MachineExecutionSettings is the exact observed machine fact that bounds an
// execution plan. The observation and generation identities make the capacity
// decision auditable instead of treating CPU availability as ambient state.
type MachineExecutionSettings struct {
	Observation     MachineObservationID `json:"observation_id"`
	Generation      MachineGenerationID  `json:"generation_id"`
	LogicalCPUCount uint16               `json:"logical_cpu_count"`
}

func (o MachineObservation) ExecutionSettings() (MachineExecutionSettings, error) {
	if err := o.Validate(); err != nil {
		return MachineExecutionSettings{}, err
	}
	settings := MachineExecutionSettings{Observation: o.ID, Generation: o.GenerationID, LogicalCPUCount: o.Configuration.Compute.VCPU}
	return settings, settings.Validate()
}

func (s MachineExecutionSettings) Validate() error {
	if s.LogicalCPUCount == 0 {
		return contractError(errors.New("project standards machine execution settings lack logical CPUs"))
	}
	return contractJoin(s.Observation.Validate(), s.Generation.Validate())
}

func (o MachineObservation) Validate() error {
	if o.SchemaVersion != MachineProbeSchemaVersion {
		return contractError(errors.New("project standards machine observation schema version is unsupported"))
	}
	if err := contractJoin(o.ID.Validate(), o.GenerationID.Validate(), o.ObservedAt.Validate(), o.Collector.Validate(), o.Execution.Validate(), o.Configuration.Validate(), o.Fingerprint.Validate()); err != nil {
		return err
	}
	if err := o.Runtime.ValidateFor(o.Configuration); err != nil {
		return err
	}
	want, err := o.Configuration.Fingerprint()
	if err != nil || want != o.Fingerprint {
		return conflictError(errors.New("project standards machine observation fingerprint differs from configuration"), err)
	}
	return nil
}

type MachineGeneration struct {
	SchemaVersion    uint16               `json:"schema_version"`
	ID               MachineGenerationID  `json:"id"`
	Fingerprint      MachineFingerprint   `json:"fingerprint"`
	Configuration    MachineConfiguration `json:"configuration"`
	FirstObservedAt  temporal.Instant     `json:"first_observed_at"`
	LastObservedAt   temporal.Instant     `json:"last_observed_at"`
	ObservationCount uint64               `json:"observation_count"`
}

func (g MachineGeneration) Validate() error {
	if g.SchemaVersion != MachineProbeSchemaVersion {
		return contractError(errors.New("project standards machine generation schema version is unsupported"))
	}
	if err := contractJoin(g.ID.Validate(), g.Fingerprint.Validate(), g.Configuration.Validate(), g.FirstObservedAt.Validate(), g.LastObservedAt.Validate()); err != nil {
		return err
	}
	want, err := g.Configuration.Fingerprint()
	if err != nil || want != g.Fingerprint || g.ObservationCount == 0 {
		return conflictError(errors.New("project standards machine generation is contradictory"), err)
	}
	return validateMachineGenerationOrder(g.FirstObservedAt, g.LastObservedAt)
}

func validateMachineGenerationOrder(first, last temporal.Instant) error {
	firstNanoseconds, firstErr := first.Nanoseconds()
	lastNanoseconds, lastErr := last.Nanoseconds()
	if err := errors.Join(firstErr, lastErr); err != nil {
		return contractError(err)
	}
	if lastNanoseconds < firstNanoseconds {
		return conflictError(errors.New("project standards machine generation observation order is contradictory"))
	}
	return nil
}

type MachineChangeField uint8

const (
	MachineChangeUnknown MachineChangeField = iota
	MachineChangeIdentity
	MachineChangeCompute
	MachineChangeMemory
	MachineChangeSystem
	MachineChangeStorage
	MachineChangeNetwork
	MachineChangeLifecycleSecurity
	MachineChangeToolchain
	machineChangeFieldLimit
)

func machineChangeFieldLabels() []string {
	return []string{"", "identity", "compute", "memory", "system", "storage", "network", "lifecycle_security", "toolchain"}
}
func (f MachineChangeField) Validate() error {
	return validateEnum(uint8(f), machineChangeFieldLabels(), "project standards machine change field is invalid")
}
func (f MachineChangeField) IsValid() bool  { return f.Validate() == nil }
func (f MachineChangeField) String() string { return enumString(uint8(f), machineChangeFieldLabels()) }
func (f MachineChangeField) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(f), machineChangeFieldLabels(), "project standards machine change field is invalid")
}
func (f *MachineChangeField) UnmarshalJSON(data []byte) error {
	if f == nil {
		return jsonError(errors.New("nil project standards machine change field receiver"))
	}
	value, err := unmarshalEnum(data, machineChangeFieldLabels(), "project standards machine change field is invalid")
	if err == nil {
		*f = MachineChangeField(value)
	}
	return err
}

type MachineChange struct {
	Field  MachineChangeField `json:"field"`
	Before Name               `json:"before"`
	After  Name               `json:"after"`
}

func (c MachineChange) Validate() error {
	if err := contractJoin(c.Field.Validate(), c.Before.Validate(), c.After.Validate()); err != nil {
		return err
	}
	if c.Before == c.After {
		return conflictError(errors.New("project standards machine change does not change a fact"))
	}
	return nil
}

type MachineGenerationTransition struct {
	PreviousGenerationID MachineGenerationID `json:"previous_generation_id"`
	CurrentGenerationID  MachineGenerationID `json:"current_generation_id"`
	DetectedAt           temporal.Instant    `json:"detected_at"`
	Changes              []MachineChange     `json:"changes"`
}

func (t MachineGenerationTransition) Validate() error {
	if err := contractJoin(t.PreviousGenerationID.Validate(), t.CurrentGenerationID.Validate(), t.DetectedAt.Validate()); err != nil {
		return err
	}
	if t.PreviousGenerationID == t.CurrentGenerationID || len(t.Changes) == 0 || len(t.Changes) > MachineChangeMaximum {
		return conflictError(errors.New("project standards machine generation transition is invalid"))
	}
	for index := range t.Changes {
		if err := t.Changes[index].Validate(); err != nil {
			return err
		}
		if index > 0 && t.Changes[index-1].Field >= t.Changes[index].Field {
			return conflictError(errors.New("project standards machine changes are not canonical"))
		}
	}
	return nil
}

type CurrentMachine struct {
	Generation  MachineGeneration            `json:"generation"`
	Observation MachineObservation           `json:"observation"`
	Transition  *MachineGenerationTransition `json:"transition,omitempty"`
}

func (m CurrentMachine) Validate() error {
	if err := contractJoin(m.Generation.Validate(), m.Observation.Validate()); err != nil {
		return err
	}
	if m.Generation.ID != m.Observation.GenerationID || m.Generation.Fingerprint != m.Observation.Fingerprint || m.Generation.Configuration.Identity.ID != m.Observation.Configuration.Identity.ID {
		return conflictError(errors.New("project standards current machine generation differs from observation"))
	}
	if m.Transition == nil {
		return nil
	}
	if err := m.Transition.Validate(); err != nil {
		return err
	}
	if m.Transition.CurrentGenerationID != m.Generation.ID {
		return conflictError(errors.New("project standards current machine transition differs from generation"))
	}
	return nil
}

func (m CurrentMachine) Digest() (core.SHA256Digest, error) {
	if err := m.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(m)
	if err != nil {
		return core.SHA256Digest{}, jsonError(err)
	}
	return core.SHA256Of(encoded), nil
}

var (
	_ core.Validatable = MachineExecutionSettings{}
	_ json.Marshaler   = MachineProbeReport{}
	_ json.Unmarshaler = (*MachineProbeReport)(nil)
)
