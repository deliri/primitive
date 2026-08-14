package hostfacts

import (
	"context"
	"errors"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// TestKernelTextIngressBoundaryLayerTriad pins the local parser boundary that
// the bounded stream reader normally protects: the exact maximum is admitted,
// maximum-plus-one is refused with the typed observation identity, and an
// unrelated controller line remains a neutral zero fact.
func TestKernelTextIngressBoundaryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact line ceiling remains a typed v2 membership", func(t *testing.T) {
		t.Parallel()

		line := cgroupMembershipLineAtExtent(procLineMaximumBytes, cgroupV2HierarchyToken, "")
		got, gotErr := parseCgroupMembershipLine(line)
		if gotErr != nil {
			t.Fatalf("parseCgroupMembershipLine(exact ceiling) error = %v, want nil", gotErr)
		}
		if got.source != WorkloadMemoryLimitSourceCgroupV2 || got.path != string(line[len(cgroupV2HierarchyToken)+2:]) {
			t.Fatalf("parseCgroupMembershipLine(exact ceiling) = %+v, want exact v2 membership", got)
		}
	})

	t.Run("negative one byte over line ceiling is a typed zero refusal", func(t *testing.T) {
		t.Parallel()

		line := cgroupMembershipLineAtExtent(procLineMaximumBytes+1, cgroupV2HierarchyToken, "")
		got, gotErr := parseCgroupMembershipLine(line)
		if !errors.Is(gotErr, core.ErrHostFactsObservation) {
			t.Fatalf("parseCgroupMembershipLine(maximum+1) error = %v, want errors.Is(..., %v)", gotErr, core.ErrHostFactsObservation)
		}
		if got != (cgroupMembership{}) {
			t.Fatalf("parseCgroupMembershipLine(maximum+1) = %+v, want zero membership", got)
		}
	})

	t.Run("neutral unrelated controller at ceiling emits no memory fact", func(t *testing.T) {
		t.Parallel()

		line := cgroupMembershipLineAtExtent(procLineMaximumBytes, "2", "cpu")
		got, gotErr := parseCgroupMembershipLine(line)
		if gotErr != nil || got != (cgroupMembership{}) {
			t.Fatalf("parseCgroupMembershipLine(unrelated exact ceiling) = (%+v, %v), want zero membership and nil", got, gotErr)
		}
	})
}

func FuzzHostnameIngressSemanticClosure(f *testing.F) {
	for _, seed := range []string{
		"host",
		"host.example.internal",
		strings.Repeat("h", hostnameMaximumBytes),
		strings.Repeat("h", hostnameMaximumBytes+1),
		"",
		"host\x00name",
		"host\nname",
		"host\xffname",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := admitHostname(value)
		want, wantErr := referenceHostname(value)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrHostFactsObservation) {
				t.Fatalf("admitHostname(rejected %d bytes) error = %v, want errors.Is(..., %v)", len(value), gotErr, core.ErrHostFactsObservation)
			}
			if got != (Hostname{}) {
				t.Fatalf("admitHostname(rejected %d bytes) = %+v, want zero hostname", len(value), got)
			}
			return
		}
		if gotErr != nil || got != want {
			t.Fatalf("admitHostname(%q) = (%+v, %v), want (%+v, nil)", value, got, gotErr, want)
		}
		if err := got.Validate(); err != nil || got.String() != value {
			t.Fatalf("admitHostname(%q) closure = (Validate %v, String %q), want (nil, input)", value, err, got.String())
		}
	})
}

func FuzzRotationalFlagSemanticClosure(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(rotationalFlagNonRotationalToken),
		[]byte(rotationalFlagNonRotationalToken + "\n"),
		[]byte(rotationalFlagRotationalToken),
		[]byte(rotationalFlagRotationalToken + "\n"),
		{},
		[]byte("2"),
		[]byte("0\n\n"),
		[]byte(strings.Repeat(rotationalFlagNonRotationalToken, rotationalFlagMaximumBytes+1)),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		got, gotErr := classifyRotationalFlag(data)
		want, wantErr := referenceRotationalFlag(data)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrHostFactsObservation) || got != DiskRotationUnknown {
				t.Fatalf("classifyRotationalFlag(rejected %q) = (%v, %v), want (%v, typed observation refusal)", data, got, gotErr, DiskRotationUnknown)
			}
			return
		}
		if gotErr != nil || got != want {
			t.Fatalf("classifyRotationalFlag(%q) = (%v, %v), want (%v, nil)", data, got, gotErr, want)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("classifyRotationalFlag(%q).Validate() error = %v, want nil", data, err)
		}
	})
}

func FuzzCgroupMembershipLineSemanticClosure(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(cgroupV2HierarchyToken + "::/"),
		[]byte(cgroupV2HierarchyToken + "::/system.slice/app.service"),
		[]byte("5:" + cgroupMemoryController + ":/job"),
		[]byte("2:cpu:/job"),
		{},
		[]byte(cgroupV2HierarchyToken + "::relative"),
		cgroupMembershipLineAtExtent(procLineMaximumBytes, cgroupV2HierarchyToken, ""),
		cgroupMembershipLineAtExtent(procLineMaximumBytes+1, cgroupV2HierarchyToken, ""),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, line []byte) {
		got, gotErr := parseCgroupMembershipLine(line)
		want, wantErr := referenceCgroupMembership(line)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrHostFactsObservation) || got != (cgroupMembership{}) {
				t.Fatalf("parseCgroupMembershipLine(rejected %d bytes) = (source %v, path bytes %d, %v), want zero typed refusal", len(line), got.source, len(got.path), gotErr)
			}
			return
		}
		if gotErr != nil || got != want {
			t.Fatalf("parseCgroupMembershipLine(%d bytes) = (source %v, path bytes %d, %v), want (source %v, path bytes %d, nil)", len(line), got.source, len(got.path), gotErr, want.source, len(want.path))
		}
		if got != (cgroupMembership{}) {
			if err := got.Validate(); err != nil {
				t.Fatalf("parseCgroupMembershipLine(%q).Validate() error = %v, want nil", line, err)
			}
			canonical := canonicalMembershipLine(got)
			roundTrip, roundTripErr := parseCgroupMembershipLine(canonical)
			if roundTripErr != nil || roundTrip != got {
				t.Fatalf("membership canonical round trip = (%+v, %v), want (%+v, nil)", roundTrip, roundTripErr, got)
			}
		}
	})
}

func FuzzMountInfoLineSemanticClosure(f *testing.F) {
	seeds := []struct {
		line   []byte
		source WorkloadMemoryLimitSource
	}{
		{line: []byte("30 23 0:27 / /sys/fs/cgroup rw - " + cgroupV2Filesystem + " cgroup rw"), source: WorkloadMemoryLimitSourceCgroupV2},
		{line: []byte("31 23 0:28 / /sys/fs/cgroup/memory rw - " + cgroupV1Filesystem + " cgroup rw," + cgroupMemoryController), source: WorkloadMemoryLimitSourceCgroupV1},
		{line: []byte("30 23 0:27 / /sys/fs/cgroup rw - ext4 /dev/disk rw"), source: WorkloadMemoryLimitSourceCgroupV2},
		{line: []byte("30 23 0:27 / /sys/fs/cgroup\\040space rw - " + cgroupV2Filesystem + " cgroup rw"), source: WorkloadMemoryLimitSourceCgroupV2},
		{line: []byte("truncated"), source: WorkloadMemoryLimitSourceCgroupV2},
		{line: oversizedMountInfoLine(), source: WorkloadMemoryLimitSourceCgroupV2},
	}
	for _, seed := range seeds {
		f.Add(seed.line, uint8(seed.source))
	}

	f.Fuzz(func(t *testing.T, line []byte, selector uint8) {
		source := WorkloadMemoryLimitSourceCgroupV2
		if selector%2 == 1 {
			source = WorkloadMemoryLimitSourceCgroupV1
		}
		got, gotMatch, gotErr := parseMountInfoLine(line, source)
		want, wantMatch, wantErr := referenceMountInfo(line, source)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrHostFactsObservation) || got != (cgroupMount{}) || gotMatch {
				t.Fatalf("parseMountInfoLine(rejected %d bytes) = (%+v, %t, %v), want zero unmatched typed refusal", len(line), got, gotMatch, gotErr)
			}
			return
		}
		if gotErr != nil || gotMatch != wantMatch || got != want {
			t.Fatalf("parseMountInfoLine(%d bytes) = (%+v, %t, %v), want (%+v, %t, nil)", len(line), got, gotMatch, gotErr, want, wantMatch)
		}
		if gotMatch {
			if err := got.Validate(); err != nil {
				t.Fatalf("parseMountInfoLine(%q).Validate() error = %v, want nil", line, err)
			}
			canonical := canonicalMountInfoLine(got)
			roundTrip, roundTripMatch, roundTripErr := parseMountInfoLine(canonical, source)
			if roundTripErr != nil || !roundTripMatch || roundTrip != got {
				t.Fatalf("mountinfo canonical round trip = (%+v, %t, %v), want (%+v, true, nil)", roundTrip, roundTripMatch, roundTripErr, got)
			}
		}
	})
}

func FuzzCgroupLimitFileSemanticClosure(f *testing.F) {
	for _, seed := range []struct {
		data   []byte
		source WorkloadMemoryLimitSource
	}{
		{data: []byte("0"), source: WorkloadMemoryLimitSourceCgroupV2},
		{data: []byte(cgroupV2MaxToken + "\n"), source: WorkloadMemoryLimitSourceCgroupV2},
		{data: []byte(strconv.FormatUint(cgroupV1UnlimitedMin-1, 10)), source: WorkloadMemoryLimitSourceCgroupV1},
		{data: []byte(strconv.FormatUint(cgroupV1UnlimitedMin, 10)), source: WorkloadMemoryLimitSourceCgroupV1},
		{data: []byte(strconv.FormatUint(math.MaxUint64, 10)), source: WorkloadMemoryLimitSourceCgroupV2},
		{data: []byte("00"), source: WorkloadMemoryLimitSourceCgroupV2},
		{data: []byte(strings.Repeat("1", limitValueMaximumBytes+1)), source: WorkloadMemoryLimitSourceCgroupV2},
	} {
		f.Add(seed.data, uint8(seed.source))
	}

	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		root := t.TempDir()
		valuePath := filepath.Join(root, cgroupV2LimitName)
		if err := os.WriteFile(valuePath, data, 0o600); err != nil {
			t.Fatalf("os.WriteFile(cgroup limit) error = %v, want nil", err)
		}
		interfacePath, err := core.ParseAbsolutePath(valuePath)
		if err != nil {
			t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", valuePath, err)
		}
		source := WorkloadMemoryLimitSourceCgroupV2
		if selector%2 == 1 {
			source = WorkloadMemoryLimitSourceCgroupV1
		}
		gotValue, gotUnlimited, gotErr := readCgroupLimit(context.Background(), interfacePath, source)
		wantValue, wantUnlimited, wantErr := referenceCgroupLimit(data, source)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrHostFactsObservation) || gotValue != 0 || gotUnlimited {
				t.Fatalf("readCgroupLimit(rejected %d bytes) = (%d, %t, %v), want zero finite typed refusal", len(data), gotValue, gotUnlimited, gotErr)
			}
			return
		}
		if gotErr != nil || gotValue != wantValue || gotUnlimited != wantUnlimited {
			t.Fatalf("readCgroupLimit(%q, %v) = (%d, %t, %v), want (%d, %t, nil)", data, source, gotValue, gotUnlimited, gotErr, wantValue, wantUnlimited)
		}
	})
}

func cgroupMembershipLineAtExtent(extent int, hierarchy, controllers string) []byte {
	prefix := hierarchy + ":" + controllers + ":/"
	return []byte(prefix + strings.Repeat("a", extent-len(prefix)))
}

func referenceHostname(value string) (Hostname, error) {
	if value == "" || len(value) > hostnameMaximumBytes || !utf8.ValidString(value) {
		return Hostname{}, core.ErrHostFactsObservation
	}
	for _, valueRune := range value {
		if valueRune < 0x20 || valueRune == 0x7f {
			return Hostname{}, core.ErrHostFactsObservation
		}
	}
	return Hostname{value: value}, nil
}

func referenceRotationalFlag(data []byte) (DiskRotation, error) {
	if len(data) == 0 || len(data) > rotationalFlagMaximumBytes {
		return DiskRotationUnknown, core.ErrHostFactsObservation
	}
	token := data
	if len(token) == 2 && token[1] == '\n' {
		token = token[:1]
	}
	if len(token) != 1 {
		return DiskRotationUnknown, core.ErrHostFactsObservation
	}
	switch string(token) {
	case rotationalFlagNonRotationalToken:
		return DiskRotationNonRotational, nil
	case rotationalFlagRotationalToken:
		return DiskRotationRotational, nil
	default:
		return DiskRotationUnknown, core.ErrHostFactsObservation
	}
}

func referenceCgroupMembership(line []byte) (cgroupMembership, error) {
	if len(line) == 0 || len(line) > procLineMaximumBytes {
		return cgroupMembership{}, core.ErrHostFactsObservation
	}
	parts := strings.SplitN(string(line), ":", 3)
	if len(parts) != 3 || parts[2] == "" {
		return cgroupMembership{}, core.ErrHostFactsObservation
	}
	source := WorkloadMemoryLimitSourceUnknown
	if parts[0] == cgroupV2HierarchyToken && parts[1] == "" {
		source = WorkloadMemoryLimitSourceCgroupV2
	} else if referenceCommaTokenContains(parts[1], cgroupMemoryController) {
		source = WorkloadMemoryLimitSourceCgroupV1
	}
	if source == WorkloadMemoryLimitSourceUnknown {
		return cgroupMembership{}, nil
	}
	if !referenceCgroupPath(parts[2]) {
		return cgroupMembership{}, core.ErrHostFactsObservation
	}
	return cgroupMembership{path: parts[2], source: source}, nil
}

func canonicalMembershipLine(membership cgroupMembership) []byte {
	if membership.source == WorkloadMemoryLimitSourceCgroupV2 {
		return []byte(cgroupV2HierarchyToken + "::" + membership.path)
	}
	return []byte("1:" + cgroupMemoryController + ":" + membership.path)
}

func referenceMountInfo(line []byte, source WorkloadMemoryLimitSource) (cgroupMount, bool, error) {
	if len(line) == 0 || len(line) > procLineMaximumBytes {
		return cgroupMount{}, false, core.ErrHostFactsObservation
	}
	fields := strings.Fields(string(line))
	separator := -1
	for index, field := range fields {
		if field == mountInfoSeparator {
			separator = index
			break
		}
	}
	if separator < 6 || len(fields) < separator+4 {
		return cgroupMount{}, false, core.ErrHostFactsObservation
	}
	filesystem, superOptions := fields[separator+1], fields[separator+3]
	matched := source == WorkloadMemoryLimitSourceCgroupV2 && filesystem == cgroupV2Filesystem ||
		source == WorkloadMemoryLimitSourceCgroupV1 && filesystem == cgroupV1Filesystem &&
			referenceCommaTokenContains(superOptions, cgroupMemoryController)
	if !matched {
		return cgroupMount{}, false, nil
	}
	root, err := referenceDecodeMountInfoPath(fields[3])
	if err != nil || !referenceCgroupPath(root) {
		return cgroupMount{}, false, core.ErrHostFactsObservation
	}
	mountPointText, err := referenceDecodeMountInfoPath(fields[4])
	if err != nil {
		return cgroupMount{}, false, core.ErrHostFactsObservation
	}
	mountPoint, err := core.ParseAbsolutePath(mountPointText)
	if err != nil {
		return cgroupMount{}, false, core.ErrHostFactsObservation
	}
	return cgroupMount{root: root, mountPoint: mountPoint, source: source}, true, nil
}

func canonicalMountInfoLine(mount cgroupMount) []byte {
	filesystem, superOptions := cgroupV2Filesystem, "rw"
	if mount.source == WorkloadMemoryLimitSourceCgroupV1 {
		filesystem = cgroupV1Filesystem
		superOptions = "rw," + cgroupMemoryController
	}
	return []byte("1 1 0:1 " + referenceEncodeMountInfoPath(mount.root) + " " +
		referenceEncodeMountInfoPath(mount.mountPoint.String()) + " rw " + mountInfoSeparator +
		" " + filesystem + " cgroup " + superOptions)
}

func oversizedMountInfoLine() []byte {
	return []byte("30 23 0:27 / /sys/fs/cgroup rw " +
		strings.Repeat("x ", procLineMaximumBytes) + mountInfoSeparator +
		" " + cgroupV2Filesystem + " cgroup rw")
}

func referenceDecodeMountInfoPath(value string) (string, error) {
	if len(value) > procLineMaximumBytes {
		return "", core.ErrHostFactsObservation
	}
	var decoded strings.Builder
	decoded.Grow(len(value))
	for index := 0; index < len(value); {
		if value[index] != '\\' {
			decoded.WriteByte(value[index])
			index++
			continue
		}
		if index+3 >= len(value) {
			return "", core.ErrHostFactsObservation
		}
		switch value[index+1 : index+4] {
		case "040":
			decoded.WriteByte(' ')
		case "011":
			decoded.WriteByte('\t')
		case "012":
			decoded.WriteByte('\n')
		case "134":
			decoded.WriteByte('\\')
		default:
			return "", core.ErrHostFactsObservation
		}
		index += 4
	}
	if !utf8.ValidString(decoded.String()) {
		return "", core.ErrHostFactsObservation
	}
	return decoded.String(), nil
}

func referenceEncodeMountInfoPath(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\134")
	value = strings.ReplaceAll(value, " ", "\\040")
	value = strings.ReplaceAll(value, "\t", "\\011")
	return strings.ReplaceAll(value, "\n", "\\012")
}

func referenceCgroupLimit(data []byte, source WorkloadMemoryLimitSource) (uint64, bool, error) {
	if len(data) == 0 || len(data) > limitValueMaximumBytes {
		return 0, false, core.ErrHostFactsObservation
	}
	token := strings.TrimSuffix(string(data), "\n")
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return 0, false, core.ErrHostFactsObservation
	}
	if source == WorkloadMemoryLimitSourceCgroupV2 && token == cgroupV2MaxToken {
		return 0, true, nil
	}
	value, err := strconv.ParseUint(token, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != token {
		return 0, false, core.ErrHostFactsObservation
	}
	if source == WorkloadMemoryLimitSourceCgroupV1 && value >= cgroupV1UnlimitedMin {
		return 0, true, nil
	}
	return value, false, nil
}

func referenceCgroupPath(value string) bool {
	return value != "" && utf8.ValidString(value) && strings.HasPrefix(value, "/") &&
		path.Clean(value) == value && !strings.ContainsRune(value, '\x00') &&
		!strings.HasSuffix(value, " (deleted)")
}

func referenceCommaTokenContains(value, token string) bool {
	for _, candidate := range strings.Split(value, ",") {
		if candidate == token {
			return true
		}
	}
	return false
}
