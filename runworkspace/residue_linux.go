//go:build linux

package runworkspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const (
	residueHostEntryMaximum = 1 << 16
	residueLineMaximum      = 64 * 1024
)

// LinuxResidueConfiguration binds the fixed host coordinates whose residue
// can belong to an isolated subject. It does not scan unrelated host state into
// a world model.
type LinuxResidueConfiguration struct {
	ProcRoot             core.AbsolutePath
	RunParent            core.AbsolutePath
	ControlGroupRoot     core.AbsolutePath
	NetworkNamespaceRoot core.AbsolutePath
	CredentialRoot       core.AbsolutePath
	SecretRoot           core.AbsolutePath
	UnitPrefix           core.PathComponent
	ProcessUserID        uint32
}

func (c LinuxResidueConfiguration) Validate() error {
	if err := errors.Join(c.ProcRoot.Validate(), c.RunParent.Validate(), c.ControlGroupRoot.Validate(), c.NetworkNamespaceRoot.Validate(), c.CredentialRoot.Validate(), c.SecretRoot.Validate(), c.UnitPrefix.Validate()); err != nil {
		return err
	}
	if c.ProcessUserID == 0 {
		return errors.Join(core.ErrPrimitiveContract, errors.New("linux residue process user id must be nonzero"))
	}
	return nil
}

// LinuxResidueSource streams the live Linux process and ownership surfaces
// under the configured, reviewed execution coordinates.
type LinuxResidueSource struct{ configuration LinuxResidueConfiguration }

type residueCounts struct {
	processes   uint32
	descriptors uint32
	sockets     uint32
}

func (residueCounts) runWorkspaceInternalFlow() {}

func NewLinuxResidueSource(configuration LinuxResidueConfiguration) (LinuxResidueSource, error) {
	if err := configuration.Validate(); err != nil {
		return LinuxResidueSource{}, err
	}
	return LinuxResidueSource{configuration: configuration}, nil
}

func (s LinuxResidueSource) Validate() error { return s.configuration.Validate() }

func (s LinuxResidueSource) ObserveResidue(ctx context.Context) (Residue, error) {
	if err := s.Validate(); err != nil {
		return Residue{}, err
	}
	processResidue, err := s.observeSubjectProcesses(ctx)
	if err != nil {
		return Residue{}, err
	}
	controlGroups, err := countPrefixedEntries(ctx, s.configuration.ControlGroupRoot, s.configuration.UnitPrefix.String())
	if err != nil {
		return Residue{}, fmt.Errorf("observe control-group residue: %w", err)
	}
	namespaces, err := countPrefixedEntries(ctx, s.configuration.NetworkNamespaceRoot, s.configuration.UnitPrefix.String())
	if err != nil {
		return Residue{}, fmt.Errorf("observe network-namespace residue: %w", err)
	}
	mounts, err := s.observeRunMounts(ctx)
	if err != nil {
		return Residue{}, err
	}
	credentials, err := countAllEntries(ctx, s.configuration.CredentialRoot)
	if err != nil {
		return Residue{}, fmt.Errorf("observe credential custody residue: %w", err)
	}
	secrets, err := countAllEntries(ctx, s.configuration.SecretRoot)
	if err != nil {
		return Residue{}, fmt.Errorf("observe secret custody residue: %w", err)
	}
	return Residue{
		Processes: processResidue.processes, ControlGroups: controlGroups, Namespaces: namespaces, Mounts: mounts,
		Descriptors: processResidue.descriptors, Sockets: processResidue.sockets, CredentialCustody: credentials, SecretCustody: secrets,
	}, nil
}

func (s LinuxResidueSource) observeSubjectProcesses(ctx context.Context) (counts residueCounts, resultErr error) {
	rootPath, err := core.ParseRelativePath(".")
	if err != nil {
		return residueCounts{}, fmt.Errorf("open Linux process table: %w", err)
	}
	root, err := filestore.OpenRoot(ctx, s.configuration.ProcRoot)
	if err != nil {
		return residueCounts{}, fmt.Errorf("open Linux process table: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	location := filestore.Location{Root: root, Path: rootPath}
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: location,
		Order:    filestore.WalkOrderNative,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			contribution, observeErr := s.observeProcessEntry(ctx, location, entry)
			if observeErr != nil {
				return filestore.WalkDirectiveUnknown, observeErr
			}
			if err := counts.add(contribution); err != nil {
				return filestore.WalkDirectiveUnknown, err
			}
			return filestore.WalkSkipDirectory, nil
		},
	})
	return counts, walkErr
}

func (s LinuxResidueSource) observeProcessEntry(ctx context.Context, proc filestore.Location, entry filestore.WalkEntry) (residueCounts, error) {
	if !entry.Entry.IsDir() || !decimalName(entry.Entry.Name()) {
		return residueCounts{}, nil
	}
	owned, err := s.processOwnedBySubject(ctx, proc, entry.Entry.Name())
	if errors.Is(err, fs.ErrNotExist) {
		return residueCounts{}, nil
	}
	if err != nil || !owned {
		return residueCounts{}, err
	}
	counts := residueCounts{processes: 1}
	descriptors, descriptorErr := s.observeProcessDescriptors(ctx, proc, entry.Entry.Name())
	if errors.Is(descriptorErr, fs.ErrNotExist) {
		return counts, nil
	}
	if descriptorErr != nil {
		return residueCounts{}, descriptorErr
	}
	counts.descriptors = descriptors.descriptors
	counts.sockets = descriptors.sockets
	return counts, nil
}

func (c *residueCounts) add(other residueCounts) error {
	return errors.Join(
		addResidue(&c.processes, other.processes),
		addResidue(&c.descriptors, other.descriptors),
		addResidue(&c.sockets, other.sockets),
	)
}

func (s LinuxResidueSource) processOwnedBySubject(ctx context.Context, proc filestore.Location, processID string) (owned bool, resultErr error) {
	statusPath, err := proc.Path.Resolve(processID, "status")
	if err != nil {
		return false, err
	}
	status, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: filestore.Location{Root: proc.Root, Path: statusPath}})
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, status.Close()) }()
	scanner := bufio.NewScanner(status)
	scanner.Buffer(make([]byte, 256), residueLineMaximum)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, linuxUIDFieldPrefix) {
			continue
		}
		uid, parseErr := ParseLinuxStatusUIDRow(line)
		return uid == s.configuration.ProcessUserID, parseErr
	}
	return false, errors.Join(core.ErrPrimitiveContract, scanner.Err(), errors.New("linux process status has no uid row"))
}

func (s LinuxResidueSource) observeProcessDescriptors(ctx context.Context, proc filestore.Location, processID string) (residueCounts, error) {
	descriptorPath, err := proc.Path.Resolve(processID, "fd")
	if err != nil {
		return residueCounts{}, err
	}
	var counts residueCounts
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: proc.Root, Path: descriptorPath},
		Order:    filestore.WalkOrderNative,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			contribution, observeErr := s.observeDescriptor(ctx, proc, processID, entry.Entry.Name())
			if observeErr != nil {
				return filestore.WalkDirectiveUnknown, observeErr
			}
			if err := counts.add(contribution); err != nil {
				return filestore.WalkDirectiveUnknown, err
			}
			return filestore.WalkSkipDirectory, nil
		},
	})
	return counts, walkErr
}

func (s LinuxResidueSource) observeDescriptor(ctx context.Context, proc filestore.Location, processID, descriptorName string) (residueCounts, error) {
	descriptorPath, err := proc.Path.Resolve(processID, "fd", descriptorName)
	if err != nil {
		return residueCounts{}, err
	}
	target, err := filestore.ReadSymbolicLink(ctx, filestore.Location{Root: proc.Root, Path: descriptorPath})
	if errors.Is(err, fs.ErrNotExist) {
		return residueCounts{}, nil
	}
	if err != nil {
		return residueCounts{}, err
	}
	var counts residueCounts
	if pathWithin(target.String(), s.configuration.RunParent.String()) {
		counts.descriptors = 1
	}
	if strings.HasPrefix(target.String(), "socket:[") {
		counts.sockets = 1
	}
	return counts, nil
}

func (s LinuxResidueSource) observeRunMounts(ctx context.Context) (count uint32, resultErr error) {
	mountInfo, err := s.configuration.ProcRoot.Resolve("self", "mountinfo")
	if err != nil {
		return 0, err
	}
	location, err := filestore.OpenParent(ctx, mountInfo)
	if err != nil {
		return 0, fmt.Errorf(openLinuxMountTableDiagnostic, err)
	}
	defer func() { resultErr = errors.Join(resultErr, location.Root.Close()) }()
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err != nil {
		return 0, fmt.Errorf(openLinuxMountTableDiagnostic, err)
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), residueLineMaximum)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		mountPoint, parseErr := ParseLinuxMountInfoPoint(scanner.Text())
		if parseErr != nil {
			return 0, parseErr
		}
		if pathWithin(mountPoint, s.configuration.RunParent.String()) {
			if err := incrementResidue(&count); err != nil {
				return 0, err
			}
		}
	}
	return count, scanner.Err()
}

func countPrefixedEntries(ctx context.Context, root core.AbsolutePath, prefix string) (uint32, error) {
	return countDirectoryEntries(ctx, root, func(name string) bool { return strings.HasPrefix(name, prefix) })
}

func countAllEntries(ctx context.Context, root core.AbsolutePath) (uint32, error) {
	return countDirectoryEntries(ctx, root, func(string) bool { return true })
}

func countDirectoryEntries(ctx context.Context, root core.AbsolutePath, include func(string) bool) (count uint32, resultErr error) {
	location, err := filestore.OpenParent(ctx, root)
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, location.Root.Close()) }()
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: location,
		Order:    filestore.WalkOrderNative,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if include(entry.Entry.Name()) {
				if err := incrementResidue(&count); err != nil {
					return filestore.WalkDirectiveUnknown, err
				}
			}
			return filestore.WalkSkipDirectory, nil
		},
	})
	return count, walkErr
}

func decimalName(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func pathWithin(value, root string) bool {
	return value == root || strings.HasPrefix(value, root+string(filepath.Separator))
}

func incrementResidue(value *uint32) error {
	if *value >= residueHostEntryMaximum || *value == math.MaxUint32 {
		return errors.Join(core.ErrPrimitiveContract, errors.New("host residue exceeds the bounded observation ceiling"))
	}
	*value++
	return nil
}

func addResidue(value *uint32, addition uint32) error {
	if addition > residueHostEntryMaximum-*value {
		return errors.Join(core.ErrPrimitiveContract, errors.New("host residue aggregate exceeds the bounded observation ceiling"))
	}
	*value += addition
	return nil
}

var (
	_ core.Validatable = LinuxResidueConfiguration{}
	_ core.Validatable = LinuxResidueSource{}
	_ ResidueSource    = LinuxResidueSource{}
	_ internalFlow     = residueCounts{}
)
