package capabilities

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

// Operation identifies an actual callable agreement, not just an effect owner.
// Unavailable is an intentional zero: no equivalent operation is advertised.
type Operation uint8

const (
	OperationUnavailable Operation = iota
	OperationReadFile
	OperationWriteFile
	OperationRunProcess
	OperationObserveTime
	operationLimit
)

func (o Operation) Validate() error {
	if o >= operationLimit {
		return contractError(catalogOperationIsOutsideTheAdmittedDomain)
	}
	return nil
}
func (o Operation) IsValid() bool { return o.Validate() == nil }

func (o Operation) String() string {
	if o.Validate() != nil {
		return core.UnknownEnumDiagnostic
	}
	return [...]string{"unavailable", "filestore.Read", "filestore.Write", "process.Run", "temporal.Observe"}[o]
}
func (o Operation) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(o.String())
}
func (o *Operation) UnmarshalJSON(data []byte) error {
	if o == nil {
		return contractError("operation receiver is nil")
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
	}
	for candidate := range operationLimit {
		if value == candidate.String() {
			*o = candidate
			return nil
		}
	}
	return contractError(catalogOperationIsOutsideTheAdmittedDomain)
}

// OperationContract supplies the exact exported function and request/result
// type coordinates. Callers still construct its request and select policy.
// These are bounded alternatives, never drop-in semantic aliases.
type OperationContract struct {
	ResultPackage core.PackageIdentity
	Function      StandardSymbol
	Request       SymbolName
	Result        SymbolName
	HasRequest    bool
}

func (c OperationContract) Validate() error {
	if err := errors.Join(c.Function.Validate(), c.Result.Validate(), c.ResultPackage.Validate()); err != nil {
		return err
	}
	if c.Function.Receiver != nil {
		return contractError("operation requires a package function")
	}
	if c.HasRequest {
		return c.Request.Validate()
	}
	if c.Request != (SymbolName{}) {
		return contractError("operation carries an unexpected request type")
	}
	return nil
}
func (o Operation) Contract() (OperationContract, bool, error) {
	if err := o.Validate(); err != nil {
		return OperationContract{}, false, err
	}
	switch o {
	case OperationUnavailable:
		return OperationContract{}, false, nil
	case OperationReadFile:
		return operationContract(operationDefinition{owner: core.PackageFilestore, selector: symbolRead, request: "ReadRequest", result: "ByteLength", resultPackage: core.PackageCore})
	case OperationWriteFile:
		return operationContract(operationDefinition{owner: core.PackageFilestore, selector: symbolWrite, request: "WriteRequest", result: "CommitRequest", resultPackage: core.PackageFilestore})
	case OperationRunProcess:
		return operationContract(operationDefinition{owner: core.PackageProcess, selector: "Run", request: symbolRequest, result: "Result", resultPackage: core.PackageProcess})
	case OperationObserveTime:
		return operationContract(operationDefinition{owner: core.PackageTemporal, selector: "Observe", result: "Observation", resultPackage: core.PackageTemporal})
	default:
		return OperationContract{}, false, contractError(catalogOperationIsOutsideTheAdmittedDomain)
	}
}

type operationDefinition struct {
	owner, resultPackage      core.PackageIdentity
	selector, request, result string
}

func (operationDefinition) capabilitiesInternalFlow() {}

func operationContract(definition operationDefinition) (OperationContract, bool, error) {
	path, err := definition.owner.ImportPath()
	if err != nil {
		return OperationContract{}, false, err
	}
	imported, err := gomodule.ParseImportPath(path)
	if err != nil {
		return OperationContract{}, false, err
	}
	name, err := ParseSymbolName(definition.selector)
	if err != nil {
		return OperationContract{}, false, err
	}
	resultName, err := ParseSymbolName(definition.result)
	if err != nil {
		return OperationContract{}, false, err
	}
	contract := OperationContract{Function: StandardSymbol{ImportPath: imported, Selector: name}, Result: resultName, ResultPackage: definition.resultPackage, HasRequest: definition.request != ""}
	if contract.HasRequest {
		contract.Request, err = ParseSymbolName(definition.request)
		if err != nil {
			return OperationContract{}, false, err
		}
	}
	return contract, true, contract.Validate()
}

// Replacement reports only reviewed callable alternatives. In particular,
// os.Exit has process ownership but no offered Primitive exit capability.
func (f StandardSymbolFact) Replacement() (Operation, error) {
	if err := f.Validate(); err != nil {
		return OperationUnavailable, err
	}
	path, selector := f.Symbol.ImportPath.String(), f.Symbol.Selector.String()
	if f.Disposition != StandardSymbolEffect {
		return OperationUnavailable, nil
	}
	if f.Symbol.Receiver != nil {
		if path == catalogOsExec && f.Symbol.Receiver.String() == "Cmd" && selector == "Run" {
			return OperationRunProcess, nil
		}
		return OperationUnavailable, nil
	}
	return functionReplacement(path, selector), nil
}
func functionReplacement(path, selector string) Operation {
	switch path {
	case "os":
		switch selector {
		case symbolReadFile:
			return OperationReadFile
		case symbolWriteFile:
			return OperationWriteFile
		default:
			return OperationUnavailable
		}
	case timeContractText:
		if selector == "Now" {
			return OperationObserveTime
		}
	}
	return OperationUnavailable
}

func (OperationContract) capabilitiesSealedProjection() {}

func (o Operation) effect() (Effect, error) {
	if err := o.Validate(); err != nil {
		return EffectUnknown, err
	}
	return [...]Effect{OperationUnavailable: EffectUnknown, OperationReadFile: EffectFilesystem, OperationWriteFile: EffectFilesystem, OperationRunProcess: EffectProcess, OperationObserveTime: EffectTime}[o], nil
}
