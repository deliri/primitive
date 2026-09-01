// Package runworkspace owns the filesystem boundary for isolated execution
// scheduling units. Product code receives typed workspace coordinates and
// never an ambient path or an operating-system filesystem handle.
package runworkspace

import (
	"context"
	"errors"
	"io/fs"
	"math"
	"os"
	"slices"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	workspaceDirectoryMode fs.FileMode = 0o700
	workspaceEntryMaximum  uint32      = 1 << 16
)

type Configuration struct {
	RunParent core.AbsolutePath
}

func (c Configuration) Validate() error { return c.RunParent.Validate() }

type Manager struct {
	root         *os.Root
	runParent    core.AbsolutePath
	rootIdentity core.SHA256Digest
}

func Open(ctx context.Context, configuration Configuration) (Manager, error) {
	if err := configuration.Validate(); err != nil {
		return Manager{}, err
	}
	root, err := filestore.OpenRoot(ctx, configuration.RunParent)
	if err != nil {
		return Manager{}, err
	}
	manager := Manager{root: root, runParent: configuration.RunParent, rootIdentity: core.SHA256Of([]byte(configuration.RunParent.String()))}
	if err := manager.Validate(); err != nil {
		return Manager{}, errors.Join(err, root.Close())
	}
	return manager, nil
}

func (m Manager) Validate() error {
	if m.root == nil {
		return core.ErrPrimitiveContract
	}
	return errors.Join(m.runParent.Validate(), m.rootIdentity.Validate())
}

func (m Manager) Absolute(path core.RelativePath) (core.AbsolutePath, error) {
	if err := errors.Join(m.Validate(), path.Validate()); err != nil {
		return core.AbsolutePath{}, err
	}
	return m.runParent.JoinRelative(path)
}

func (m Manager) Close() error {
	if err := m.Validate(); err != nil {
		return err
	}
	return m.root.Close()
}

type Unit struct {
	Identity     runnercontrol.SchedulingUnitIdentity
	Root         core.RelativePath
	Checkout     core.RelativePath
	RootIdentity core.SHA256Digest
}

func (u Unit) Validate() error {
	if err := errors.Join(u.Identity.Validate(), u.Root.Validate(), u.Checkout.Validate(), u.RootIdentity.Validate()); err != nil {
		return err
	}
	_, err := u.Checkout.RelativeTo(u.Root)
	return err
}

func (m Manager) CreateUnit(ctx context.Context, identity runnercontrol.SchedulingUnitIdentity) (Unit, error) {
	if err := errors.Join(m.Validate(), identity.Validate()); err != nil {
		return Unit{}, err
	}
	root, err := core.ParseRelativePath(identity.Identity.String())
	if err != nil {
		return Unit{}, err
	}
	if err := filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: m.root, Path: root}, Mode: workspaceDirectoryMode}); err != nil {
		return Unit{}, err
	}
	checkout, err := joinLiteral(root, "checkout")
	if err != nil {
		return Unit{}, errors.Join(err, filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{Location: filestore.Location{Root: m.root, Path: root}}))
	}
	unit := Unit{Identity: identity, Root: root, Checkout: checkout, RootIdentity: m.rootIdentity}
	return unit, unit.Validate()
}

type Member struct {
	Run  projectstandards.RunID
	Root core.RelativePath
}

func (m Member) Validate() error {
	return errors.Join(m.Run.Validate(), m.Root.Validate())
}

func (m Member) Experiment(identity projectstandards.ExperimentID) (Experiment, error) {
	if err := errors.Join(m.Validate(), identity.Validate()); err != nil {
		return Experiment{}, err
	}
	return resolveExperiment(m, identity)
}

func (m Manager) CreateMember(ctx context.Context, unit Unit, run projectstandards.RunID) (Member, error) {
	if err := errors.Join(m.Validate(), unit.Validate(), run.Validate()); err != nil || unit.RootIdentity != m.rootIdentity {
		return Member{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	runComponent, err := runPathComponent(run)
	if err != nil {
		return Member{}, err
	}
	members, err := joinLiteral(unit.Root, "members")
	if err != nil {
		return Member{}, err
	}
	root, err := members.Join(runComponent)
	if err != nil {
		return Member{}, err
	}
	member := Member{Run: run, Root: root}
	experiments, err := joinLiteral(root, "experiments")
	if err != nil {
		return Member{}, err
	}
	if err := filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: m.root, Path: experiments}, Mode: workspaceDirectoryMode}); err != nil {
		return Member{}, err
	}
	return member, member.Validate()
}

type Experiment struct {
	Run       projectstandards.RunID
	Identity  projectstandards.ExperimentID
	Root      core.RelativePath
	Home      core.RelativePath
	Output    core.RelativePath
	Cache     core.RelativePath
	Temporary core.RelativePath
}

func (e Experiment) Validate() error {
	return errors.Join(e.Run.Validate(), e.Identity.Validate(), e.Root.Validate(), e.Home.Validate(), e.Output.Validate(), e.Cache.Validate(), e.Temporary.Validate())
}

func (m Manager) CreateExperiment(ctx context.Context, member Member, identity projectstandards.ExperimentID) (Experiment, error) {
	if err := errors.Join(m.Validate(), member.Validate(), identity.Validate()); err != nil {
		return Experiment{}, err
	}
	experiment, err := resolveExperiment(member, identity)
	if err != nil {
		return Experiment{}, err
	}
	directories := [...]struct {
		label       string
		destination *core.RelativePath
	}{
		{label: "home", destination: &experiment.Home},
		{label: "output", destination: &experiment.Output},
		{label: "cache", destination: &experiment.Cache},
		{label: "tmp", destination: &experiment.Temporary},
	}
	for _, directory := range directories {
		path, joinErr := joinLiteral(experiment.Root, directory.label)
		if joinErr != nil {
			return Experiment{}, joinErr
		}
		*directory.destination = path
		if ensureErr := filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: m.root, Path: path}, Mode: workspaceDirectoryMode}); ensureErr != nil {
			return Experiment{}, ensureErr
		}
	}
	return experiment, experiment.Validate()
}

func (m Manager) ResolveExperiment(member Member, identity projectstandards.ExperimentID) (Experiment, error) {
	if err := errors.Join(m.Validate(), member.Validate(), identity.Validate()); err != nil {
		return Experiment{}, err
	}
	return member.Experiment(identity)
}

func (m Manager) ValidateWritableWorkspace(workspace Experiment, binding runnercontrol.WritableWorkspace, environment process.Environment) error {
	if err := errors.Join(m.Validate(), workspace.Validate(), binding.Validate()); err != nil {
		return err
	}
	root, rootErr := m.Absolute(workspace.Root)
	home, homeErr := m.Absolute(workspace.Home)
	output, outputErr := m.Absolute(workspace.Output)
	cache, cacheErr := m.Absolute(workspace.Cache)
	temporary, temporaryErr := m.Absolute(workspace.Temporary)
	if err := errors.Join(rootErr, homeErr, outputErr, cacheErr, temporaryErr); err != nil {
		return err
	}
	if binding != (runnercontrol.WritableWorkspace{Root: root, Home: home, Output: output, Cache: cache, Temporary: temporary}) {
		return core.ErrPrimitiveContract
	}
	values, err := environment.Strings()
	if err != nil {
		return err
	}
	want := [...]string{
		core.EnvironmentHomeName + "=" + home.String(),
		core.EnvironmentTemporaryName + "=" + temporary.String(),
		core.EnvironmentCacheName + "=" + cache.String(),
	}
	for _, required := range want {
		if !slices.Contains(values, required) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func resolveExperiment(member Member, identity projectstandards.ExperimentID) (Experiment, error) {
	experiments, err := joinLiteral(member.Root, "experiments")
	if err != nil {
		return Experiment{}, err
	}
	component, err := experimentPathComponent(identity)
	if err != nil {
		return Experiment{}, err
	}
	root, err := experiments.Join(component)
	if err != nil {
		return Experiment{}, err
	}
	experiment := Experiment{Run: member.Run, Identity: identity, Root: root}
	for _, directory := range []struct {
		label       string
		destination *core.RelativePath
	}{{"home", &experiment.Home}, {"output", &experiment.Output}, {"cache", &experiment.Cache}, {"tmp", &experiment.Temporary}} {
		path, joinErr := joinLiteral(root, directory.label)
		if joinErr != nil {
			return Experiment{}, joinErr
		}
		*directory.destination = path
	}
	return experiment, experiment.Validate()
}

func runPathComponent(run projectstandards.RunID) (core.PathComponent, error) {
	encoded, err := run.MarshalJSON()
	if err != nil {
		return core.PathComponent{}, err
	}
	value, err := core.DecodeJSONStringToken(encoded)
	if err != nil {
		return core.PathComponent{}, err
	}
	return core.ParsePathComponent(value)
}

func experimentPathComponent(experiment projectstandards.ExperimentID) (core.PathComponent, error) {
	encoded, err := experiment.MarshalJSON()
	if err != nil {
		return core.PathComponent{}, err
	}
	value, err := core.DecodeJSONStringToken(encoded)
	if err != nil {
		return core.PathComponent{}, err
	}
	return core.ParsePathComponent(value)
}

func joinLiteral(parent core.RelativePath, value string) (core.RelativePath, error) {
	component, err := core.ParsePathComponent(value)
	if err != nil {
		return core.RelativePath{}, err
	}
	return parent.Join(component)
}

type Residue struct {
	Processes         uint32
	ControlGroups     uint32
	Namespaces        uint32
	Mounts            uint32
	Descriptors       uint32
	Sockets           uint32
	CredentialCustody uint32
	SecretCustody     uint32
}

func (Residue) Validate() error { return nil }

// ResidueSource is the Primitive-owned observation capability for non-file
// machine state that must be clean before and after an execution unit.
type ResidueSource interface {
	ObserveResidue(context.Context) (Residue, error)
}

func (m Manager) Observe(ctx context.Context, observedAt temporal.Instant, residue Residue) (runnercontrol.MachineStateObservation, error) {
	if err := errors.Join(m.Validate(), observedAt.Validate(), residue.Validate()); err != nil {
		return runnercontrol.MachineStateObservation{}, err
	}
	rootPath, err := core.ParseRelativePath(".")
	maximum, maximumErr := filestore.NewDirectoryEntryMaximum(workspaceEntryMaximum)
	if err != nil || maximumErr != nil {
		return runnercontrol.MachineStateObservation{}, errors.Join(err, maximumErr)
	}
	var entries uint32
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: m.root, Path: rootPath}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum,
		Visit: func(filestore.WalkEntry) (filestore.WalkDirective, error) {
			if entries == math.MaxUint32 {
				return filestore.WalkDirectiveUnknown, core.ErrPrimitiveContract
			}
			entries++
			return filestore.WalkContinue, nil
		},
	})
	if walkErr != nil {
		return runnercontrol.MachineStateObservation{}, walkErr
	}
	observation := runnercontrol.MachineStateObservation{
		RootIdentity: m.rootIdentity, Entries: entries, Processes: residue.Processes,
		ControlGroups: residue.ControlGroups, Namespaces: residue.Namespaces, Mounts: residue.Mounts,
		Descriptors: residue.Descriptors, Sockets: residue.Sockets, CredentialCustody: residue.CredentialCustody,
		SecretCustody: residue.SecretCustody, ObservedAt: observedAt,
	}
	return observation, observation.Validate()
}

func (m Manager) CleanupUnit(ctx context.Context, unit Unit) error {
	if err := errors.Join(m.Validate(), unit.Validate()); err != nil || unit.RootIdentity != m.rootIdentity {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	maximum, err := filestore.NewDirectoryEntryMaximum(workspaceEntryMaximum)
	if err != nil {
		return err
	}
	if err := filestore.SetPermissions(ctx, filestore.PermissionRequest{
		Location: filestore.Location{Root: m.root, Path: unit.Root}, Mode: workspaceDirectoryMode,
	}); err != nil {
		return err
	}
	walkErr := filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: m.root, Path: unit.Root}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if !entry.Entry.IsDir() {
				return filestore.WalkContinue, nil
			}
			permissionErr := filestore.SetPermissions(ctx, filestore.PermissionRequest{Location: filestore.Location{Root: m.root, Path: entry.Path}, Mode: workspaceDirectoryMode})
			return filestore.WalkContinue, permissionErr
		},
	})
	if walkErr != nil {
		return walkErr
	}
	return filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{Location: filestore.Location{Root: m.root, Path: unit.Root}})
}

// Scrub removes every entry beneath the fixed run parent. It streams the
// parent's bounded entry set and never follows a symbolic link outside the
// rooted filestore capability.
func (m Manager) Scrub(ctx context.Context) error {
	if err := m.Validate(); err != nil {
		return err
	}
	rootPath, err := core.ParseRelativePath(".")
	maximum, maximumErr := filestore.NewDirectoryEntryMaximum(workspaceEntryMaximum)
	if err != nil || maximumErr != nil {
		return errors.Join(err, maximumErr)
	}
	return filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: m.root, Path: rootPath},
		Order:    filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			removeErr := filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{
				Location: filestore.Location{Root: m.root, Path: entry.Path},
			})
			return filestore.WalkSkipDirectory, removeErr
		},
	})
}

func (m Manager) ProveClean(ctx context.Context, observedAt temporal.Instant, residue Residue) (runnercontrol.CleanMachineState, error) {
	observation, err := m.Observe(ctx, observedAt, residue)
	if err != nil {
		return runnercontrol.CleanMachineState{}, err
	}
	clean := runnercontrol.CleanMachineState{Observation: observation}
	return clean, clean.Validate()
}

var (
	_ core.Validatable = Configuration{}
	_ core.Validatable = Manager{}
	_ core.Validatable = Unit{}
	_ core.Validatable = Member{}
	_ core.Validatable = Residue{}
)
