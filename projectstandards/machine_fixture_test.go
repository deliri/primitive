package projectstandards

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func fixtureMachineProbeReport(t testing.TB) MachineProbeReport {
	t.Helper()
	uuid := fixtureProjectStandardsUUID(t)
	machineID, err := NewMachineID(uuid)
	if err != nil {
		t.Fatalf("NewMachineID() setup error = %v, want nil", err)
	}
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{1})
	return MachineProbeReport{
		SchemaVersion: MachineProbeSchemaVersion,
		Configuration: MachineConfiguration{
			Identity: MachineIdentity{
				ID: machineID, Provider: core.Offering{Token: "google-cloud"}, Project: fixtureIdentifier(t, "example-project"),
				Instance: fixtureIdentifier(t, "runner-1"), Zone: fixtureIdentifier(t, "northamerica-northeast2-a"), MachineType: fixtureIdentifier(t, "e2-standard-4"),
			},
			Compute: MachineCompute{
				CPUPlatform: fixtureName(t, "Intel Broadwell"), Processor: fixtureName(t, "Intel Xeon CPU @ 2.20 GHz"),
				Architecture: fixtureName(t, "x86_64"), Virtualization: fixtureName(t, "Google virtualization"),
				VCPU: 4, Sockets: 1, CoresPerSocket: 2, ThreadsPerCore: 2, NUMANodes: 1,
				MemoryConfiguredBytes: fixtureByteCount(t, 16<<30), MemoryGuestBytes: fixtureByteCount(t, 15<<30),
			},
			System: MachineSystem{
				OperatingSystem: fixtureName(t, "Ubuntu"), OperatingSystemVersion: fixtureName(t, "24.04.4 LTS"),
				OperatingSystemImage: fixtureName(t, "ubuntu-2404-noble-amd64-v20260826"), Kernel: fixtureName(t, "6.17.0-1022-gcp"),
			},
			Storage: MachineStorage{
				BootDiskType: fixtureName(t, "Balanced Persistent Disk"), Interface: fixtureName(t, "SCSI"), Filesystem: fixtureName(t, "ext4"),
				PhysicalBlockBytes: fixtureByteCount(t, 4096), CapacityBytes: fixtureByteCount(t, 30<<30),
				BaselineIOPS: 3000, BaselineReadBytes: 140 << 20, InstanceCeilingIOPS: 15_000, InstanceCeilingReadBytes: 240 << 20,
				SwapBytes: fixtureByteLength(t, 0),
			},
			Network: MachineNetwork{
				Interface: fixtureName(t, "VirtIO Net"), NetworkTier: fixtureName(t, "Tier 1 disabled"), Addressing: fixtureName(t, "IPv4 ephemeral"),
				VPC: fixtureIdentifier(t, "example-test-runner"), MTU: 1460, ReceiveQueues: 4, TransmitQueues: 4,
				EgressFloorBits: 1_000_000_000, EgressCeilingBits: 10_000_000_000,
			},
			Lifecycle: MachineLifecycleSecurity{
				ProvisioningModel: MachineProvisioningStandard, StoppedWhenIdle: true, HostMaintenance: MachineMaintenanceMigrate,
				SecureBoot: true, VirtualTPM: true, IntegrityMonitoring: true,
			},
			Toolchains: []MachineToolchain{{
				Tool: MachineToolchainGo, Version: fixtureName(t, "go1.27.0"), Platform: fixtureName(t, "linux/amd64"),
				InstallMode: MachineInstallModeInstalled, ExecutableSHA256: digest,
			}},
		},
		Runtime: MachineRuntime{
			BootID: fixtureIdentifier(t, "4c9b6f78-79d0-442b-bd8f-1dcbbcc9c68f"), Uptime: fixtureDuration(t, 60),
			MemoryAvailableBytes: fixtureByteLength(t, 14<<30), DiskAvailableBytes: fixtureByteLength(t, 24<<30), Address: fixtureName(t, "10.42.0.4"),
		},
	}
}

func fixtureCurrentMachine(t testing.TB) CurrentMachine {
	t.Helper()
	report := fixtureMachineProbeReport(t)
	uuid := fixtureProjectStandardsUUID(t)
	observationID, observationErr := NewMachineObservationID(uuid)
	if observationErr != nil {
		t.Fatalf("NewMachineObservationID() setup error = %v, want nil", observationErr)
	}
	generationID, generationErr := NewMachineGenerationID(uuid)
	if generationErr != nil {
		t.Fatalf("NewMachineGenerationID() setup error = %v, want nil", generationErr)
	}
	fingerprint, fingerprintErr := report.Configuration.Fingerprint()
	if fingerprintErr != nil {
		t.Fatalf("MachineConfiguration.Fingerprint() setup error = %v, want nil", fingerprintErr)
	}
	bash := fixtureAbsolutePath(t, "/bin/bash")
	script := fixtureAbsolutePath(t, "/opt/runner/bin/machine-probe.sh")
	outputLimit := fixtureByteCount(t, 256*1024)
	zeroDuration, durationErr := temporal.DurationFromNanoseconds(0)
	if durationErr != nil {
		t.Fatalf("temporal.DurationFromNanoseconds(0) setup error = %v, want nil", durationErr)
	}
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{1})
	emptyDigest := core.SHA256Of(nil)
	observedAt := temporal.InstantFromNanoseconds(2_000_000)
	observation := MachineObservation{
		SchemaVersion: MachineProbeSchemaVersion, ID: observationID, GenerationID: generationID, ObservedAt: observedAt,
		Collector: EvidenceAuthority{Offering: core.Offering{Token: "runner"}},
		Execution: MachineProbeExecution{
			Bash: bash, Script: script, ScriptDigest: digest, ScriptBytes: fixtureByteLength(t, 1), OutputLimit: outputLimit, CPUTime: zeroDuration,
			StdoutDigest: digest, StdoutBytes: fixtureByteLength(t, 1), StderrDigest: emptyDigest, StderrBytes: fixtureByteLength(t, 0),
		},
		Configuration: report.Configuration, Runtime: report.Runtime, Fingerprint: fingerprint,
	}
	generation := MachineGeneration{
		SchemaVersion: MachineProbeSchemaVersion, ID: generationID, Fingerprint: fingerprint, Configuration: report.Configuration,
		FirstObservedAt: observedAt, LastObservedAt: observedAt, ObservationCount: 1,
	}
	current := CurrentMachine{Generation: generation, Observation: observation}
	if err := current.Validate(); err != nil {
		t.Fatalf("CurrentMachine.Validate() setup error = %v, want nil", err)
	}
	return current
}

func fixtureByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) setup error = %v, want nil", value, err)
	}
	return got
}

func fixtureByteLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()
	got, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) setup error = %v, want nil", value, err)
	}
	return got
}

func fixtureDuration(t testing.TB, seconds uint64) temporal.Duration {
	t.Helper()
	got, err := temporal.DurationFromSeconds(seconds)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(%d) setup error = %v, want nil", seconds, err)
	}
	return got
}

func fixtureAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	got, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) setup error = %v, want nil", value, err)
	}
	return got
}
