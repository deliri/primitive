package machineprobe_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/machineprobe"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestMachineProbeProcessBoundaryLayerTriad(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr     error
		script      func(testing.TB) []byte
		name        string
		wantFailure machineprobe.FailureKind
	}{
		{name: "validated script report becomes an exact observed machine", script: validProbeScript},
		{name: "malformed script report remains typed evidence refusal", script: func(testing.TB) []byte { return []byte("#!/bin/bash\nprintf '{'\n") }, wantErr: core.ErrHostFactsEvidence, wantFailure: machineprobe.FailureOutput},
		{name: "nonzero probe exit creates no machine observation", script: func(testing.TB) []byte { return []byte("#!/bin/bash\nprintf 'probe refused' >&2\nexit 19\n") }, wantErr: core.ErrHostFactsObservation, wantFailure: machineprobe.FailureExit},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			script := current.script(t)
			request := writeProbeFixture(t, directory, script)
			got, gotErr := machineprobe.Collect(t.Context(), request)
			if current.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("machineprobe.Collect() error = %v, want nil", gotErr)
				}
				if err := got.Validate(); err != nil {
					t.Fatalf("machineprobe.Collect().Validate() error = %v, want nil", err)
				}
				if got.Configuration.Compute.VCPU != 4 || got.Execution.ScriptDigest != core.SHA256Of(script) || got.Execution.ScriptBytes.Uint64() != uint64(len(script)) || got.Execution.StdoutBytes.Uint64() == 0 {
					t.Fatalf("machineprobe.Collect() = vCPU %d, script digest %v, script bytes %d, stdout %d; want 4, exact executed digest/bytes, and nonzero output evidence", got.Configuration.Compute.VCPU, got.Execution.ScriptDigest, got.Execution.ScriptBytes.Uint64(), got.Execution.StdoutBytes.Uint64())
				}
				return
			}
			if !errors.Is(gotErr, current.wantErr) || !isZeroObservation(got) {
				t.Fatalf("machineprobe.Collect(rejected) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, current.wantErr)
			}
			var failure machineprobe.Failure
			if !errors.As(gotErr, &failure) || failure.Kind() != current.wantFailure {
				t.Fatalf("machineprobe.Collect(rejected) failure = (%v, %v), want kind %v with exact stderr accounting", failure, gotErr, current.wantFailure)
			}
			gotStderrBytes := failure.StderrBytes().Uint64()
			if current.wantFailure == machineprobe.FailureExit && gotStderrBytes == 0 {
				t.Fatalf("machineprobe.Collect(nonzero exit) stderr bytes = %d, want nonzero", gotStderrBytes)
			}
			if current.wantFailure == machineprobe.FailureOutput && gotStderrBytes != 0 {
				t.Fatalf("machineprobe.Collect(malformed output) stderr bytes = %d, want zero", gotStderrBytes)
			}
		})
	}
}

func TestMachineProbeScriptExtentBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		bytes   uint64
	}{
		{name: "one byte below the script ceiling executes", bytes: machineprobe.ScriptMaximumBytes - 1},
		{name: "the exact script ceiling executes", bytes: machineprobe.ScriptMaximumBytes},
		{name: "one byte above the script ceiling is refused before execution", bytes: machineprobe.ScriptMaximumBytes + 1, wantErr: core.ErrFilestoreSize},
		{name: "an empty script cannot manufacture a machine observation", bytes: 0, wantErr: core.ErrHostFactsEvidence},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			script := exactProbeScript(t, current.bytes)
			got, gotErr := machineprobe.Collect(t.Context(), writeProbeFixture(t, directory, script))
			if current.wantErr == nil {
				if gotErr != nil || got.Execution.ScriptBytes.Uint64() != current.bytes || got.Execution.ScriptDigest != core.SHA256Of(script) {
					t.Fatalf("machineprobe.Collect(%d-byte script) = (script bytes %d, digest %v, error %v), want (%d, %v, nil)", current.bytes, got.Execution.ScriptBytes.Uint64(), got.Execution.ScriptDigest, gotErr, current.bytes, core.SHA256Of(script))
				}
				return
			}
			if !errors.Is(gotErr, current.wantErr) || !isZeroObservation(got) {
				t.Fatalf("machineprobe.Collect(%d-byte script) = (%v, %v), want zero and errors.Is(..., %v)", current.bytes, got, gotErr, current.wantErr)
			}
		})
	}
}

func exactProbeScript(t testing.TB, size uint64) []byte {
	t.Helper()
	if size == 0 {
		return []byte{}
	}
	suffix := []byte("\nexec /bin/cat -- report.json\n")
	if size < uint64(len(suffix)) {
		return bytes.Repeat([]byte("#"), int(size))
	}
	prefix := bytes.Repeat([]byte("#"), int(size)-len(suffix))
	return append(prefix, suffix...)
}

func isZeroObservation(got runprotocol.MachineObservation) bool {
	return got.SchemaVersion == 0 &&
		got.Configuration.Toolchains == nil &&
		got.Execution.StdoutBytes.Uint64() == 0
}

func writeProbeFixture(t *testing.T, directory string, script []byte) machineprobe.Request {
	t.Helper()
	rootPath := mustAbsolutePath(t, directory)
	root, err := filestore.OpenRoot(t.Context(), rootPath)
	if err != nil {
		t.Fatalf("filestore.OpenRoot() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("probe root Close() error = %v, want nil", closeErr)
		}
	})
	writeFile(t, root, "machine-probe.sh", ".machine-probe.sh-stage", script, 0o700)
	if bytes.Contains(script, []byte("report.json")) {
		report := machineReport(t)
		encoded, encodeErr := report.MarshalJSON()
		if encodeErr != nil {
			t.Fatalf("MachineProbeReport.MarshalJSON() setup error = %v, want nil", encodeErr)
		}
		writeFile(t, root, "report.json", ".report.json-stage", encoded, 0o600)
	}
	bashName, err := core.ParsePathComponent("bash")
	if err != nil {
		t.Fatalf("core.ParsePathComponent(bash) setup error = %v, want nil", err)
	}
	bash, err := process.Resolve(t.Context(), bashName)
	if err != nil {
		t.Fatalf("process.Resolve(bash) setup error = %v, want nil", err)
	}
	uuid := mustUUID(t)
	observationID, observationErr := runprotocol.NewMachineObservationID(uuid)
	generationID, generationErr := runprotocol.NewMachineGenerationID(uuid)
	if err := errors.Join(observationErr, generationErr); err != nil {
		t.Fatalf("machine probe identity setup error = %v, want nil", err)
	}
	waitDelay, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(2) setup error = %v, want nil", err)
	}
	return machineprobe.Request{
		ObservationID: observationID, GenerationID: generationID, ObservedAt: temporal.InstantFromNanoseconds(2_000_000),
		Collector: runprotocol.EvidenceAuthority{Offering: core.Offering{Token: "runner"}}, Bash: bash,
		Script:           mustAbsolutePath(t, filepath.Join(directory, "machine-probe.sh")),
		WorkingDirectory: rootPath, Environment: process.Environment{Mode: process.EnvironmentModeExact, Variables: []process.EnvironmentVariable{}}, WaitDelay: waitDelay,
	}
}

func writeFile(t *testing.T, root *os.Root, target, temporary string, content []byte, mode fs.FileMode) {
	t.Helper()
	maximum := max(uint64(len(content)), 1)
	_, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: bytes.NewReader(content), Location: filestore.Location{Root: root, Path: mustRelativePath(t, target)},
		Temporary: mustRelativePath(t, temporary), Mode: mode, Install: filestore.InstallCreate, MaximumBytes: mustByteCount(t, maximum),
	})
	if err != nil {
		t.Fatalf("filestore.Write(%s) error = %v, want nil", target, err)
	}
}

func validProbeScript(testing.TB) []byte {
	return []byte("#!/bin/bash\nexec /bin/cat -- report.json\n")
}

func machineReport(t testing.TB) runprotocol.MachineProbeReport {
	t.Helper()
	machineID, err := runprotocol.NewMachineID(mustUUID(t))
	if err != nil {
		t.Fatalf("runprotocol.NewMachineID() setup error = %v, want nil", err)
	}
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{1})
	return runprotocol.MachineProbeReport{
		SchemaVersion: runprotocol.MachineProbeSchemaVersion,
		Configuration: runprotocol.MachineConfiguration{
			Identity:   runprotocol.MachineIdentity{ID: machineID, Provider: core.Offering{Token: "google-cloud"}, Project: mustIdentifier(t, "example-project"), Instance: mustIdentifier(t, "runner-1"), Zone: mustIdentifier(t, "northamerica-northeast2-a"), MachineType: mustIdentifier(t, "e2-standard-4")},
			Compute:    runprotocol.MachineCompute{CPUPlatform: mustName(t, "Intel Broadwell"), Processor: mustName(t, "Intel Xeon"), Architecture: mustName(t, "x86_64"), Virtualization: mustName(t, "Google virtualization"), VCPU: 4, Sockets: 1, CoresPerSocket: 2, ThreadsPerCore: 2, NUMANodes: 1, MemoryConfiguredBytes: mustByteCount(t, 16<<30), MemoryGuestBytes: mustByteCount(t, 15<<30)},
			System:     runprotocol.MachineSystem{OperatingSystem: mustName(t, "Ubuntu"), OperatingSystemVersion: mustName(t, "24.04.4 LTS"), OperatingSystemImage: mustName(t, "ubuntu-2404"), Kernel: mustName(t, "6.17.0-gcp")},
			Storage:    runprotocol.MachineStorage{BootDiskType: mustName(t, "Balanced Persistent Disk"), Interface: mustName(t, "SCSI"), Filesystem: mustName(t, "ext4"), PhysicalBlockBytes: mustByteCount(t, 4096), CapacityBytes: mustByteCount(t, 30<<30), BaselineIOPS: 3000, BaselineReadBytes: 140 << 20, InstanceCeilingIOPS: 15_000, InstanceCeilingReadBytes: 240 << 20, SwapBytes: mustByteLength(t, 0)},
			Network:    runprotocol.MachineNetwork{Interface: mustName(t, "VirtIO Net"), NetworkTier: mustName(t, "Tier 1 disabled"), Addressing: mustName(t, "IPv4 ephemeral"), VPC: mustIdentifier(t, "example-test-runner"), MTU: 1460, ReceiveQueues: 4, TransmitQueues: 4, EgressFloorBits: 1_000_000_000, EgressCeilingBits: 10_000_000_000},
			Lifecycle:  runprotocol.MachineLifecycleSecurity{ProvisioningModel: runprotocol.MachineProvisioningStandard, StoppedWhenIdle: true, HostMaintenance: runprotocol.MachineMaintenanceMigrate, SecureBoot: true, VirtualTPM: true, IntegrityMonitoring: true},
			Toolchains: []runprotocol.MachineToolchain{{Tool: runprotocol.MachineToolchainGo, Version: mustName(t, "go1.27.0"), Platform: mustName(t, "linux/amd64"), InstallMode: runprotocol.MachineInstallModeInstalled, ExecutableSHA256: digest}},
		},
		Runtime: runprotocol.MachineRuntime{BootID: mustIdentifier(t, "4c9b6f78-79d0-442b-bd8f-1dcbbcc9c68f"), Uptime: mustDuration(t, 60), MemoryAvailableBytes: mustByteLength(t, 14<<30), DiskAvailableBytes: mustByteLength(t, 24<<30), Address: mustName(t, "10.42.0.4")},
	}
}

func mustUUID(t testing.TB) primitiveid.UUIDv7 {
	t.Helper()
	got, err := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	if err != nil {
		t.Fatalf("id.ParseUUIDv7() setup error = %v, want nil", err)
	}
	return got
}

func mustIdentifier(t testing.TB, value string) runprotocol.Identifier {
	t.Helper()
	got, err := runprotocol.NewIdentifier(value)
	if err != nil {
		t.Fatalf("runprotocol.NewIdentifier(%q) setup error = %v, want nil", value, err)
	}
	return got
}
func mustName(t testing.TB, value string) runprotocol.Name {
	t.Helper()
	got, err := runprotocol.NewName(value)
	if err != nil {
		t.Fatalf("runprotocol.NewName(%q) setup error = %v, want nil", value, err)
	}
	return got
}
func mustByteCount(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) setup error = %v, want nil", value, err)
	}
	return got
}
func mustByteLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()
	got, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) setup error = %v, want nil", value, err)
	}
	return got
}
func mustDuration(t testing.TB, seconds uint64) temporal.Duration {
	t.Helper()
	got, err := temporal.DurationFromSeconds(seconds)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(%d) setup error = %v, want nil", seconds, err)
	}
	return got
}
func mustAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	got, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) setup error = %v, want nil", value, err)
	}
	return got
}
func mustRelativePath(t testing.TB, value string) core.RelativePath {
	t.Helper()
	got, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("core.ParseRelativePath(%q) setup error = %v, want nil", value, err)
	}
	return got
}
