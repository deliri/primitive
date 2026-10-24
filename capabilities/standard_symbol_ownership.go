package capabilities

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// OwnsStandardSymbol reports whether caller owns this native reference. The
// catalog retains one canonical owner for each effect. Descriptor custody is
// also shared by the compiled filesystem, process and locking owners: these
// packages must keep their descriptor valid while performing their own effect.
// This does not authorize the callback's operations or descriptor escape.
// Filestore additionally owns File.Fd's native blocking-mode transition; its
// blocking descriptor duplication door proves the held lifetime separately.
func (c Catalog) OwnsStandardSymbol(symbol StandardSymbol, caller gomodule.ImportPath) (bool, error) {
	if err := errors.Join(c.Validate(), symbol.Validate(), caller.Validate()); err != nil {
		return false, errors.Join(core.ErrCapabilitiesContract, err)
	}
	fact, err := ResolveStandardSymbol(symbol)
	if err != nil || fact.Disposition != StandardSymbolEffect {
		return false, err
	}
	for contract := range c.architecture.Packages() {
		path, err := contract.Identity.ImportPath()
		if err != nil {
			return false, err
		}
		if path != caller.String() {
			continue
		}
		owner, err := effectOwner(fact.Effect)
		if err != nil {
			return false, err
		}
		if contract.Identity == owner {
			return true, nil
		}
		if contract.Identity == core.PackageFilestore && symbol.ImportPath.String() == "os" && symbol.Receiver != nil && symbol.Receiver.String() == symbolFile && symbol.Selector.String() == "Fd" {
			return true, nil
		}
		if !sharedDescriptorCustody(symbol) {
			return false, nil
		}
		for _, effect := range [...]Effect{EffectFilesystem, EffectProcess, EffectLocking} {
			owner, err := effectOwner(effect)
			if err != nil {
				return false, err
			}
			if contract.Identity == owner {
				return true, nil
			}
		}
		return false, nil
	}
	return false, nil
}

func sharedDescriptorCustody(symbol StandardSymbol) bool {
	if symbol.Receiver == nil {
		return false
	}
	return symbol.ImportPath.String() == "os" && symbol.Receiver.String() == symbolFile && symbol.Selector.String() == symbolSyscallConn ||
		symbol.ImportPath.String() == "syscall" && symbol.Receiver.String() == "RawConn" && symbol.Selector.String() == "Control"
}
