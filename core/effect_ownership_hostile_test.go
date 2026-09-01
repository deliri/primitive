package core

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type realWorldSubstrate uint8

const (
	realWorldSubstrateUnknown realWorldSubstrate = iota
	realWorldSubstrateOperatingSystem
	realWorldSubstrateProcessExecution
	realWorldSubstrateOperatingSystemSignal
	realWorldSubstrateHTTP
	realWorldSubstrateClock
	realWorldSubstrateEntropy
	realWorldSubstrateUnix
	realWorldSubstrateWindows
	realWorldSubstrateGoogleCloudStorage
	realWorldSubstrateGoogleIAMCredentials
	realWorldSubstrateGoogleSecretManager
	realWorldSubstrateNetwork
	realWorldSubstrateContextDeadline
	realWorldSubstrateLimit
)

const (
	realWorldImportOwnerMaximum        = 10
	realWorldImportUseMaximum          = 64
	realWorldCallUseMaximum            = 128
	realWorldOperatingSystemPath       = "os"
	realWorldProcessExecutionPath      = "os/exec"
	realWorldOperatingSystemSignalPath = "os/signal"
	realWorldHTTPPath                  = "net/http"
	realWorldHTTPCookieJarPath         = "net/http/cookiejar"
	realWorldClockPath                 = "time"
	realWorldEntropyPath               = "crypto/rand"
	realWorldUnixPath                  = "golang.org/x/sys/unix"
	realWorldWindowsPath               = "golang.org/x/sys/windows"
	realWorldGoogleCloudStoragePath    = "cloud.google.com/go/storage"
	realWorldGoogleIAMCredentialsPath  = "google.golang.org/api/iamcredentials/v1"
	realWorldGoogleSecretManagerPath   = "cloud.google.com/go/secretmanager/apiv1"
	realWorldNetworkPath               = "net"
	realWorldContextDeadlinePath       = "context"
)

type realWorldImportContract struct {
	owners    [realWorldImportOwnerMaximum]PackageIdentity
	substrate realWorldSubstrate
	count     uint8
}

type realWorldImportUse struct {
	owner     PackageIdentity
	substrate realWorldSubstrate
}

type realWorldCallUse struct {
	selector  string
	count     uint16
	owner     PackageIdentity
	substrate realWorldSubstrate
}

type realWorldImportInventory struct {
	values [realWorldImportUseMaximum]realWorldImportUse
	count  uint8
}

type realWorldCallInventory struct {
	values [realWorldCallUseMaximum]realWorldCallUse
	count  uint8
}

type realWorldSource struct {
	name   string
	source []byte
	owner  PackageIdentity
}

type realWorldScan struct {
	calls   realWorldCallInventory
	imports realWorldImportInventory
}

type realWorldImportBinding struct {
	name      string
	substrate realWorldSubstrate
}

func TestRealWorldEffectOwnershipMatchesLandedProduction(t *testing.T) {
	t.Parallel()

	got, err := scanLandedRealWorldEffects("..")
	if err != nil {
		t.Fatalf("scanLandedRealWorldEffects() error = %v, want nil", err)
	}
	wantImports, err := declaredRealWorldImports()
	if err != nil {
		t.Fatalf("declaredRealWorldImports() error = %v, want nil", err)
	}
	wantCalls, err := declaredRealWorldCalls()
	if err != nil {
		t.Fatalf("declaredRealWorldCalls() error = %v, want nil", err)
	}
	if !slices.Equal(got.imports.Values(), wantImports.Values()) {
		t.Fatalf("real-world substrate imports = %+v, want %+v", got.imports.Values(), wantImports.Values())
	}
	if !slices.Equal(got.calls.Values(), wantCalls.Values()) {
		t.Fatalf("real-world substrate calls = %+v, want %+v", got.calls.Values(), wantCalls.Values())
	}
	if err := validateAdmittedRealWorldUses(got, wantImports, wantCalls); err != nil {
		t.Fatalf("validateAdmittedRealWorldUses() error = %v, want nil", err)
	}
}

func TestRealWorldEffectOwnershipLayerTriad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		wantImports []realWorldImportUse
		wantCalls   []realWorldCallUse
		source      realWorldSource
	}{
		{
			name: "positive Exchange owns HTTP request construction",
			source: realWorldSource{owner: PackageExchange, name: "client.go", source: []byte(`package exchange
import (
 "context"
 "net/http"
)
func execute(ctx context.Context) { _, _ = http.NewRequestWithContext(ctx, "GET", "https://example.invalid", nil) }
`)},
			wantImports: []realWorldImportUse{{owner: PackageExchange, substrate: realWorldSubstrateHTTP}},
			wantCalls:   []realWorldCallUse{{owner: PackageExchange, substrate: realWorldSubstrateHTTP, selector: "NewRequestWithContext", count: 1}},
		},
		{
			name: "negative Release reaching past Exchange remains visible",
			source: realWorldSource{owner: PackageRelease, name: "release.go", source: []byte(`package release
import (
 "context"
 "net/http"
)
func execute(ctx context.Context) { _, _ = http.NewRequestWithContext(ctx, "GET", "https://example.invalid", nil) }
`)},
			wantImports: []realWorldImportUse{{owner: PackageRelease, substrate: realWorldSubstrateHTTP}},
			wantCalls:   []realWorldCallUse{{owner: PackageRelease, substrate: realWorldSubstrateHTTP, selector: "NewRequestWithContext", count: 1}},
			wantErr:     ErrPrimitiveContract,
		},
		{
			name: "neutral OS capability type and sentinel are not effects",
			source: realWorldSource{owner: PackageUpgrade, name: "upgrade.go", source: []byte(`package upgrade
import "os"
type request struct { Root *os.Root }
func absent(err error) bool { return err == os.ErrNotExist }
`)},
			wantImports: []realWorldImportUse{{owner: PackageUpgrade, substrate: realWorldSubstrateOperatingSystem}},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := scanRealWorldSources([]realWorldSource{testCase.source})
			if err != nil {
				t.Fatalf("scanRealWorldSources() error = %v, want nil", err)
			}
			if !slices.Equal(got.imports.Values(), testCase.wantImports) ||
				!slices.Equal(got.calls.Values(), testCase.wantCalls) {
				t.Fatalf("scanRealWorldSources() = imports %+v calls %+v, want imports %+v calls %+v",
					got.imports.Values(), got.calls.Values(), testCase.wantImports, testCase.wantCalls)
			}
			wantImports, err := declaredRealWorldImports()
			if err != nil {
				t.Fatalf("declaredRealWorldImports() error = %v, want nil", err)
			}
			wantCalls, err := declaredRealWorldCalls()
			if err != nil {
				t.Fatalf("declaredRealWorldCalls() error = %v, want nil", err)
			}
			gotErr := validateAdmittedRealWorldUses(got, wantImports, wantCalls)
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("validateAdmittedRealWorldUses() error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func scanLandedRealWorldEffects(root string) (realWorldScan, error) {
	paths, err := productionSourcePaths(root)
	if err != nil {
		return realWorldScan{}, err
	}
	var result realWorldScan
	for _, path := range paths {
		owner, err := realWorldSourceOwner(path)
		if err != nil {
			return realWorldScan{}, err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return realWorldScan{}, errors.Join(ErrPrimitiveContract, err)
		}
		if err := scanRealWorldSource(realWorldSource{owner: owner, name: path, source: source}, &result); err != nil {
			return realWorldScan{}, err
		}
	}
	result.imports.Sort()
	result.calls.Sort()
	return result, nil
}

func productionSourcePaths(root string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(root, "*", "*.go"))
	if err != nil {
		return nil, errors.Join(ErrPrimitiveContract, err)
	}
	paths = slices.DeleteFunc(paths, func(path string) bool {
		return strings.HasSuffix(path, "_test.go") || filepath.Base(filepath.Dir(path)) == "vendor"
	})
	slices.Sort(paths)
	return paths, nil
}

func realWorldSourceOwner(path string) (PackageIdentity, error) {
	owner, err := ParsePackageIdentity(filepath.Base(filepath.Dir(path)))
	if err != nil {
		return PackageUnknown, err
	}
	return owner, nil
}

func scanRealWorldSources(sources []realWorldSource) (realWorldScan, error) {
	var result realWorldScan
	for _, source := range sources {
		if err := scanRealWorldSource(source, &result); err != nil {
			return realWorldScan{}, err
		}
	}
	result.imports.Sort()
	result.calls.Sort()
	return result, nil
}

func scanRealWorldSource(source realWorldSource, result *realWorldScan) error {
	if err := source.owner.Validate(); err != nil || result == nil {
		return errors.Join(ErrPrimitiveContract, err)
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, source.name, source.source, 0)
	if err != nil {
		return errors.Join(ErrPrimitiveContract, err)
	}
	bindings, err := realWorldImportBindings(file)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.substrate == realWorldSubstrateContextDeadline {
			continue
		}
		if err := result.imports.Add(realWorldImportUse{owner: source.owner, substrate: binding.substrate}); err != nil {
			return err
		}
	}
	var scanErr error
	ast.Inspect(file, func(node ast.Node) bool {
		if scanErr != nil {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if ok {
			selector, selected := call.Fun.(*ast.SelectorExpr)
			if selected {
				scanErr = recordRealWorldSelector(realWorldSelectorInput{
					owner: source.owner, bindings: bindings, selector: selector, inventory: &result.calls,
				})
			}
			return true
		}
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || !realWorldEntropyReader(bindings, selector) {
			return true
		}
		scanErr = recordRealWorldSelector(realWorldSelectorInput{
			owner: source.owner, bindings: bindings, selector: selector, inventory: &result.calls,
		})
		return true
	})
	return scanErr
}

func realWorldEntropyReader(bindings []realWorldImportBinding, selector *ast.SelectorExpr) bool {
	if selector == nil || selector.Sel.Name != "Reader" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	for _, binding := range bindings {
		if binding.name == identifier.Name && binding.substrate == realWorldSubstrateEntropy {
			return true
		}
	}
	return false
}

func realWorldImportBindings(file *ast.File) ([]realWorldImportBinding, error) {
	bindings := make([]realWorldImportBinding, 0, len(file.Imports))
	for _, declaration := range file.Imports {
		path, err := strconv.Unquote(declaration.Path.Value)
		if err != nil {
			return nil, errors.Join(ErrPrimitiveContract, err)
		}
		substrate := parseRealWorldSubstrate(path)
		if substrate == realWorldSubstrateUnknown {
			continue
		}
		name := filepath.Base(path)
		if declaration.Name != nil {
			name = declaration.Name.Name
		}
		if name == "." || name == "_" {
			return nil, fmt.Errorf("real-world substrate import %s has alias %s: %w", path, name, ErrPrimitiveContract)
		}
		bindings = append(bindings, realWorldImportBinding{name: name, substrate: substrate})
	}
	return bindings, nil
}

type realWorldSelectorInput struct {
	selector  *ast.SelectorExpr
	inventory *realWorldCallInventory
	bindings  []realWorldImportBinding
	owner     PackageIdentity
}

func recordRealWorldSelector(input realWorldSelectorInput) error {
	identifier, ok := input.selector.X.(*ast.Ident)
	if !ok {
		return nil
	}
	for _, binding := range input.bindings {
		if binding.name != identifier.Name {
			continue
		}
		if binding.substrate == realWorldSubstrateContextDeadline &&
			input.selector.Sel.Name != "WithTimeout" && input.selector.Sel.Name != "WithDeadline" {
			return nil
		}
		return input.inventory.Add(realWorldCallUse{
			owner: input.owner, substrate: binding.substrate, selector: input.selector.Sel.Name, count: 1,
		})
	}
	return nil
}

func parseRealWorldSubstrate(path string) realWorldSubstrate {
	switch path {
	case realWorldOperatingSystemPath:
		return realWorldSubstrateOperatingSystem
	case realWorldProcessExecutionPath:
		return realWorldSubstrateProcessExecution
	case realWorldOperatingSystemSignalPath:
		return realWorldSubstrateOperatingSystemSignal
	case realWorldHTTPPath, realWorldHTTPCookieJarPath:
		return realWorldSubstrateHTTP
	case realWorldClockPath:
		return realWorldSubstrateClock
	case realWorldEntropyPath:
		return realWorldSubstrateEntropy
	case realWorldUnixPath:
		return realWorldSubstrateUnix
	case realWorldWindowsPath:
		return realWorldSubstrateWindows
	case realWorldGoogleCloudStoragePath:
		return realWorldSubstrateGoogleCloudStorage
	case realWorldGoogleIAMCredentialsPath:
		return realWorldSubstrateGoogleIAMCredentials
	case realWorldGoogleSecretManagerPath:
		return realWorldSubstrateGoogleSecretManager
	case realWorldNetworkPath:
		return realWorldSubstrateNetwork
	case realWorldContextDeadlinePath:
		return realWorldSubstrateContextDeadline
	default:
		return realWorldSubstrateUnknown
	}
}

func declaredRealWorldImports() (realWorldImportInventory, error) {
	contracts := []realWorldImportContract{
		realWorldImportOwners(realWorldSubstrateOperatingSystem,
			PackageCore, PackageDistribution, PackageFileLock, PackageFilestore, PackageHostFacts,
			PackageProcess, PackageRelease, PackageRunWorkspace, PackageShutdown, PackageUpgrade),
		realWorldImportOwners(realWorldSubstrateProcessExecution, PackageProcess),
		realWorldImportOwners(realWorldSubstrateOperatingSystemSignal, PackageShutdown),
		realWorldImportOwners(realWorldSubstrateHTTP, PackageCore, PackageControlWire, PackageExchange, PackageProjectStandards,
			PackageProviderWire, PackageRunnerControl),
		realWorldImportOwners(realWorldSubstrateClock, PackageCloudIdentity, PackageTemporal, PackageTimeProof),
		realWorldImportOwners(realWorldSubstrateEntropy, PackageAttest, PackageKeygen),
		realWorldImportOwners(realWorldSubstrateUnix, PackageFileLock, PackageFilestore, PackageHostFacts, PackageProcess),
		realWorldImportOwners(realWorldSubstrateWindows, PackageFileLock, PackageFilestore, PackageHostFacts, PackageProcess),
		realWorldImportOwners(realWorldSubstrateGoogleCloudStorage, PackageGCSObjects),
		realWorldImportOwners(realWorldSubstrateGoogleIAMCredentials, PackageGCSObjects),
		realWorldImportOwners(realWorldSubstrateGoogleSecretManager, PackageSecretStore),
		realWorldImportOwners(realWorldSubstrateNetwork, PackageGCSObjects),
	}
	var inventory realWorldImportInventory
	for _, contract := range contracts {
		for index := range contract.count {
			if err := inventory.Add(realWorldImportUse{owner: contract.owners[index], substrate: contract.substrate}); err != nil {
				return realWorldImportInventory{}, err
			}
		}
	}
	inventory.Sort()
	return inventory, nil
}

func realWorldImportOwners(substrate realWorldSubstrate, owners ...PackageIdentity) realWorldImportContract {
	contract := realWorldImportContract{substrate: substrate, count: uint8(len(owners))}
	copy(contract.owners[:], owners)
	return contract
}

func declaredRealWorldCalls() (realWorldCallInventory, error) {
	contracts := []realWorldCallUse{
		{owner: PackageCore, substrate: realWorldSubstrateOperatingSystem, selector: "IsPathSeparator", count: 3},
		{owner: PackageFilestore, substrate: realWorldSubstrateOperatingSystem, selector: "Lstat", count: 1},
		{owner: PackageFilestore, substrate: realWorldSubstrateOperatingSystem, selector: "OpenRoot", count: 3},
		{owner: PackageFilestore, substrate: realWorldSubstrateOperatingSystem, selector: "SameFile", count: 10},
		{owner: PackageFilestore, substrate: realWorldSubstrateOperatingSystem, selector: "Stat", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "Hostname", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "Lstat", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "NewFile", count: 2},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "Open", count: 2},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "OpenRoot", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "Readlink", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "SameFile", count: 2},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "TempDir", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "UserCacheDir", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "UserConfigDir", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateOperatingSystem, selector: "UserHomeDir", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateOperatingSystem, selector: "Environ", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateOperatingSystem, selector: "Executable", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateOperatingSystem, selector: "Getpid", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateOperatingSystem, selector: "Getwd", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateOperatingSystem, selector: "LookupEnv", count: 1},
		{owner: PackageRunWorkspace, substrate: realWorldSubstrateOperatingSystem, selector: "Open", count: 5},
		{owner: PackageRunWorkspace, substrate: realWorldSubstrateOperatingSystem, selector: "Readlink", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateProcessExecution, selector: "CommandContext", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateProcessExecution, selector: "LookPath", count: 2},
		{owner: PackageShutdown, substrate: realWorldSubstrateOperatingSystemSignal, selector: "Notify", count: 1},
		{owner: PackageShutdown, substrate: realWorldSubstrateOperatingSystemSignal, selector: "Stop", count: 1},
		{owner: PackageExchange, substrate: realWorldSubstrateHTTP, selector: "NewRequestWithContext", count: 3},
		{owner: PackageExchange, substrate: realWorldSubstrateHTTP, selector: "New", count: 1},
		{owner: PackageExchange, substrate: realWorldSubstrateHTTP, selector: "ParseTime", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "Duration", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "NewTicker", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "NewTimer", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "Now", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "Parse", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "ParseDuration", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateClock, selector: "Unix", count: 2},
		{owner: PackageTimeProof, substrate: realWorldSubstrateClock, selector: "Date", count: 4},
		{owner: PackageCloudIdentity, substrate: realWorldSubstrateClock, selector: "Parse", count: 2},
		{owner: PackageCloudIdentity, substrate: realWorldSubstrateClock, selector: "Unix", count: 2},
		{owner: PackageAttest, substrate: realWorldSubstrateEntropy, selector: "Reader", count: 1},
		{owner: PackageKeygen, substrate: realWorldSubstrateEntropy, selector: "Read", count: 3},
		{owner: PackageFileLock, substrate: realWorldSubstrateUnix, selector: "Flock", count: 2},
		{owner: PackageFilestore, substrate: realWorldSubstrateUnix, selector: "SetNonblock", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Close", count: 6},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Dup", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Fstat", count: 3},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Fstatat", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Fstatfs", count: 2},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "IoctlGetWinsize", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Major", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Minor", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Open", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Openat", count: 2},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "SysctlUint64", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateUnix, selector: "Sysinfo", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateUnix, selector: "Kill", count: 3},
		{owner: PackageFileLock, substrate: realWorldSubstrateWindows, selector: "Handle", count: 2},
		{owner: PackageFileLock, substrate: realWorldSubstrateWindows, selector: "LockFileEx", count: 1},
		{owner: PackageFileLock, substrate: realWorldSubstrateWindows, selector: "UnlockFileEx", count: 1},
		{owner: PackageFilestore, substrate: realWorldSubstrateWindows, selector: "CloseHandle", count: 1},
		{owner: PackageFilestore, substrate: realWorldSubstrateWindows, selector: "CreateFile", count: 1},
		{owner: PackageFilestore, substrate: realWorldSubstrateWindows, selector: "UTF16PtrFromString", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateWindows, selector: "GetConsoleScreenBufferInfo", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateWindows, selector: "GetDiskFreeSpaceEx", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateWindows, selector: "GetFileInformationByHandle", count: 1},
		{owner: PackageHostFacts, substrate: realWorldSubstrateWindows, selector: "GetFinalPathNameByHandle", count: 2},
		{owner: PackageHostFacts, substrate: realWorldSubstrateWindows, selector: "Handle", count: 4},
		{owner: PackageHostFacts, substrate: realWorldSubstrateWindows, selector: "UTF16PtrFromString", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "CloseHandle", count: 2},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "CreateToolhelp32Snapshot", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "GetExitCodeProcess", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "OpenProcess", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "Process32First", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "Process32Next", count: 1},
		{owner: PackageProcess, substrate: realWorldSubstrateWindows, selector: "UTF16ToString", count: 1},
		{owner: PackageGCSObjects, substrate: realWorldSubstrateGoogleCloudStorage, selector: "NewClient", count: 1},
		{owner: PackageGCSObjects, substrate: realWorldSubstrateGoogleCloudStorage, selector: "SignedURL", count: 2},
		{owner: PackageGCSObjects, substrate: realWorldSubstrateGoogleCloudStorage, selector: "WithJSONReads", count: 1},
		{owner: PackageGCSObjects, substrate: realWorldSubstrateGoogleCloudStorage, selector: "WithPolicy", count: 1},
		{owner: PackageGCSObjects, substrate: realWorldSubstrateGoogleIAMCredentials, selector: "NewService", count: 1},
		{owner: PackageSecretStore, substrate: realWorldSubstrateGoogleSecretManager, selector: "NewClient", count: 1},
		{owner: PackageGCSObjects, substrate: realWorldSubstrateNetwork, selector: "ParseIP", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateContextDeadline, selector: "WithDeadline", count: 1},
		{owner: PackageTemporal, substrate: realWorldSubstrateContextDeadline, selector: "WithTimeout", count: 1},
	}
	var inventory realWorldCallInventory
	for _, contract := range contracts {
		for range contract.count {
			unit := contract
			unit.count = 1
			if err := inventory.Add(unit); err != nil {
				return realWorldCallInventory{}, err
			}
		}
	}
	inventory.Sort()
	return inventory, nil
}

func validateAdmittedRealWorldUses(
	got realWorldScan,
	wantImports realWorldImportInventory,
	wantCalls realWorldCallInventory,
) error {
	admittedImports := wantImports.Values()
	admittedCalls := wantCalls.Values()
	for _, observed := range got.imports.Values() {
		if !slices.Contains(admittedImports, observed) {
			return ErrPrimitiveContract
		}
	}
	for _, observed := range got.calls.Values() {
		admitted := false
		for _, contract := range admittedCalls {
			if contract.owner == observed.owner && contract.substrate == observed.substrate &&
				contract.selector == observed.selector && observed.count <= contract.count {
				admitted = true
				break
			}
		}
		if !admitted {
			return ErrPrimitiveContract
		}
	}
	return nil
}

func (i *realWorldImportInventory) Add(value realWorldImportUse) error {
	if value.owner.Validate() != nil || value.substrate <= realWorldSubstrateUnknown || value.substrate >= realWorldSubstrateLimit {
		return ErrPrimitiveContract
	}
	for index := range i.count {
		if i.values[index] == value {
			return nil
		}
	}
	if i.count == realWorldImportUseMaximum {
		return ErrPrimitiveContract
	}
	i.values[i.count] = value
	i.count++
	return nil
}

func (i *realWorldCallInventory) Add(value realWorldCallUse) error {
	if value.owner.Validate() != nil || value.substrate <= realWorldSubstrateUnknown ||
		value.substrate >= realWorldSubstrateLimit || value.selector == "" || value.count != 1 {
		return ErrPrimitiveContract
	}
	for index := range i.count {
		if i.values[index].owner == value.owner && i.values[index].substrate == value.substrate &&
			i.values[index].selector == value.selector {
			i.values[index].count++
			return nil
		}
	}
	if i.count == realWorldCallUseMaximum {
		return ErrPrimitiveContract
	}
	i.values[i.count] = value
	i.count++
	return nil
}

func (i *realWorldImportInventory) Sort() {
	slices.SortFunc(i.values[:i.count], compareRealWorldImportUse)
}

func (i *realWorldCallInventory) Sort() {
	slices.SortFunc(i.values[:i.count], compareRealWorldCallUse)
}

func (i realWorldImportInventory) Values() []realWorldImportUse {
	return slices.Clone(i.values[:i.count])
}

func (i realWorldCallInventory) Values() []realWorldCallUse {
	return slices.Clone(i.values[:i.count])
}

func compareRealWorldImportUse(left, right realWorldImportUse) int {
	if left.substrate != right.substrate {
		return int(left.substrate) - int(right.substrate)
	}
	return int(left.owner) - int(right.owner)
}

func compareRealWorldCallUse(left, right realWorldCallUse) int {
	if compared := compareRealWorldImportUse(
		realWorldImportUse{owner: left.owner, substrate: left.substrate},
		realWorldImportUse{owner: right.owner, substrate: right.substrate},
	); compared != 0 {
		return compared
	}
	return strings.Compare(left.selector, right.selector)
}
