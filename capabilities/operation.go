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
	OperationExitCurrent
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
	return [...]string{"unavailable", "filestore.Read", "filestore.Write", "process.Run", "temporal.Observe", "process.ExitCurrent"}[o]
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

// OperationResultKind identifies the complete Go return shape. A terminal
// operation may return only an error on refusal; it has no value to fabricate.
// The zero value is invalid. This discriminator is a compiler-only contract.
type OperationResultKind uint8

const (
	OperationResultUnknown OperationResultKind = iota
	OperationResultValueAndError
	OperationResultErrorOnly
)

func (k OperationResultKind) Validate() error {
	switch k {
	case OperationResultValueAndError, OperationResultErrorOnly:
		return nil
	default:
		return contractError("operation result kind is outside the admitted domain")
	}
}

func (OperationResultKind) OffWireEnum() {}

var _ core.OffWireEnum = OperationResultUnknown

// OperationContract supplies the exact exported function and typed input and
// result coordinates. Callers still construct its input and select policy.
// These are bounded alternatives, never drop-in semantic aliases.
type OperationContract struct {
	Function      StandardSymbol
	Request       SymbolName
	Result        SymbolName
	ResultPackage core.PackageIdentity
	ResultKind    OperationResultKind
	HasRequest    bool
}

func (c OperationContract) Validate() error {
	if err := errors.Join(c.Function.Validate(), c.validateResult()); err != nil {
		return errors.Join(core.ErrCapabilitiesContract, err)
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

func (c OperationContract) validateResult() error {
	if err := c.ResultKind.Validate(); err != nil {
		return err
	}
	switch c.ResultKind {
	case OperationResultValueAndError:
		return errors.Join(c.Result.Validate(), c.ResultPackage.Validate())
	case OperationResultErrorOnly:
		if c.Result != (SymbolName{}) || c.ResultPackage != core.PackageUnknown {
			return contractError("error-only operation carries a fabricated value result")
		}
		return nil
	default:
		return contractError("operation result kind is outside the admitted domain")
	}
}
func (o Operation) Contract() (OperationContract, bool, error) {
	if err := o.Validate(); err != nil {
		return OperationContract{}, false, err
	}
	switch o {
	case OperationUnavailable:
		return OperationContract{}, false, nil
	case OperationReadFile:
		return operationContract(operationDefinition{resultKind: OperationResultValueAndError, owner: core.PackageFilestore, selector: symbolRead, request: "ReadRequest", result: "ByteLength", resultPackage: core.PackageCore})
	case OperationWriteFile:
		return operationContract(operationDefinition{resultKind: OperationResultValueAndError, owner: core.PackageFilestore, selector: symbolWrite, request: "WriteRequest", result: "CommitRequest", resultPackage: core.PackageFilestore})
	case OperationRunProcess:
		return operationContract(operationDefinition{resultKind: OperationResultValueAndError, owner: core.PackageProcess, selector: "Run", request: symbolRequest, result: "Result", resultPackage: core.PackageProcess})
	case OperationObserveTime:
		return operationContract(operationDefinition{resultKind: OperationResultValueAndError, owner: core.PackageTemporal, selector: "Observe", result: "Observation", resultPackage: core.PackageTemporal})
	case OperationExitCurrent:
		return operationContract(operationDefinition{resultKind: OperationResultErrorOnly, owner: core.PackageProcess, selector: "ExitCurrent", request: "ExitStatus"})
	default:
		return OperationContract{}, false, contractError(catalogOperationIsOutsideTheAdmittedDomain)
	}
}

type operationDefinition struct {
	resultKind                OperationResultKind
	selector, request, result string
	owner, resultPackage      core.PackageIdentity
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
	contract := OperationContract{Function: StandardSymbol{ImportPath: imported, Selector: name}, ResultPackage: definition.resultPackage, ResultKind: definition.resultKind, HasRequest: definition.request != ""}
	if definition.resultKind == OperationResultValueAndError {
		contract.Result, err = ParseSymbolName(definition.result)
		if err != nil {
			return OperationContract{}, false, err
		}
	}
	if contract.HasRequest {
		contract.Request, err = ParseSymbolName(definition.request)
		if err != nil {
			return OperationContract{}, false, err
		}
	}
	return contract, true, contract.Validate()
}

// Replacement reports only reviewed callable alternatives.
func (f StandardSymbolFact) Replacement() (Operation, error) {
	if err := f.Validate(); err != nil {
		return OperationUnavailable, err
	}
	if f.Disposition != StandardSymbolEffect {
		return OperationUnavailable, nil
	}
	operation := symbolReplacement(f.Symbol)
	owner, err := operation.effect()
	if err != nil {
		return OperationUnavailable, err
	}
	if operation != OperationUnavailable && owner != f.Effect {
		return OperationUnavailable, contractError("replacement contradicts the supplied effect")
	}
	if f.Operation != OperationUnavailable && f.Operation != operation {
		return OperationUnavailable, contractError("replacement contradicts the retained operation")
	}
	return operation, nil
}

func symbolReplacement(symbol StandardSymbol) Operation {
	path, selector := symbol.ImportPath.String(), symbol.Selector.String()
	if symbol.Receiver == nil {
		return functionReplacement(path, selector)
	}
	if path == catalogOsExec && symbol.Receiver.String() == "Cmd" && selector == "Run" {
		return OperationRunProcess
	}
	return OperationUnavailable
}

func functionReplacement(path, selector string) Operation {
	switch path {
	case "os":
		switch selector {
		case symbolReadFile:
			return OperationReadFile
		case symbolWriteFile:
			return OperationWriteFile
		case "Exit":
			return OperationExitCurrent
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
	return [...]Effect{OperationUnavailable: EffectUnknown, OperationReadFile: EffectFilesystem, OperationWriteFile: EffectFilesystem, OperationRunProcess: EffectProcess, OperationObserveTime: EffectTime, OperationExitCurrent: EffectProcess}[o], nil
}
