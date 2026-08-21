package upgrade

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
)

type trialRevision uint8

const (
	trialRevisionUnknown trialRevision = iota
	trialRevisionCurrent
	trialRevisionLimit
)

func (r trialRevision) Validate() error {
	if r != trialRevisionCurrent {
		return contractError(diagnosticTrialDocument)
	}
	return nil
}

func (r trialRevision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return []byte("1"), nil
}

func (r *trialRevision) UnmarshalJSON(data []byte) error {
	if r == nil || !bytes.Equal(data, []byte("1")) {
		return errors.Join(
			core.ErrJSONContract,
			contractError(diagnosticTrialDocument),
		)
	}
	*r = trialRevisionCurrent
	return nil
}

// trialDocument is the durable ownership fact for one candidate slot. It
// distinguishes an interrupted attempt for the same candidate from another
// candidate that a caller may still be trialing.
type trialDocument struct {
	Candidate release.Artifact
	Prior     selectionDocument
	Revision  trialRevision
}

type trialWire struct {
	Revision  *trialRevision     `json:"revision"`
	Prior     *selectionDocument `json:"prior"`
	Candidate *release.Artifact  `json:"candidate"`
}

func newTrialDocument(target TrialTarget) (trialDocument, error) {
	if err := target.Validate(); err != nil {
		return trialDocument{}, err
	}
	document := trialDocument{
		Revision:  trialRevisionCurrent,
		Prior:     target.prior,
		Candidate: target.candidate,
	}
	if err := document.Validate(); err != nil {
		return trialDocument{}, err
	}
	return document, nil
}

func (d trialDocument) Validate() error {
	if err := d.Revision.Validate(); err != nil {
		return contractError(diagnosticTrialDocument, err)
	}
	if err := d.Prior.Validate(); err != nil {
		return contractError(diagnosticTrialDocument, err)
	}
	if err := d.Candidate.Validate(); err != nil {
		return contractError(diagnosticTrialDocument, err)
	}
	if err := validateUpgradePair(d.Prior.Artifact, d.Candidate); err != nil {
		return contractError(diagnosticTrialDocument, err)
	}
	return nil
}

func (d trialDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	revision, prior, candidate := d.Revision, d.Prior, d.Candidate
	return json.Marshal(trialWire{
		Revision: &revision, Prior: &prior, Candidate: &candidate,
	})
}

func (d *trialDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(
			core.ErrJSONContract,
			contractError(diagnosticTrialDocument),
		)
	}
	decoded, err := decodeTrialStructure(data)
	if err != nil {
		return err
	}
	if decoded.Revision == nil ||
		decoded.Prior == nil ||
		decoded.Candidate == nil {
		return errors.Join(
			core.ErrJSONContract,
			contractError(diagnosticTrialDocument),
		)
	}
	candidate := trialDocument{
		Revision:  *decoded.Revision,
		Prior:     *decoded.Prior,
		Candidate: *decoded.Candidate,
	}
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func trialJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(trialDocumentMaximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, err
	}
	return core.StrictJSONLimits{
		DocumentMaximumBytes: maximum,
		NestingDepthMaximum:  core.JSONNestingDepthMaximum,
		ObjectFieldMaximum:   core.JSONObjectFieldCountMaximum,
		ArrayItemMaximum:     selectionArrayItemMaximum,
	}, nil
}

func decodeTrialStructure(data []byte) (trialWire, error) {
	limits, err := trialJSONLimits()
	if err != nil {
		return trialWire{}, contractError(err)
	}
	value, err := core.DecodeStrictJSONStructure[trialWire](data, limits)
	if err != nil {
		return trialWire{}, contractError(
			core.ErrJSONContract, diagnosticJSON, err,
		)
	}
	return value, nil
}

func encodeTrial(document trialDocument) ([]byte, error) {
	limits, err := trialJSONLimits()
	if err != nil {
		return nil, contractError(err)
	}
	encoded, err := core.EncodeValidatedJSON(document, limits)
	if err != nil {
		return nil, contractError(
			core.ErrJSONContract, diagnosticJSON, err,
		)
	}
	return encoded, nil
}

func decodeTrial(data []byte) (trialDocument, error) {
	var decoded trialDocument
	if err := json.Unmarshal(data, &decoded); err != nil {
		return trialDocument{}, contractError(
			core.ErrJSONContract, diagnosticJSON, err,
		)
	}
	encoded, err := encodeTrial(decoded)
	if err != nil || !bytes.Equal(encoded, data) {
		return trialDocument{}, contractError(
			core.ErrJSONContract, diagnosticJSON, err,
		)
	}
	return decoded, nil
}

func readTrial(
	ctx context.Context,
	root *os.Root,
	slot Slot,
) (trialDocument, error) {
	var destination bytes.Buffer
	maximum, err := core.NewByteCount(trialDocumentMaximumBytes)
	if err != nil {
		return trialDocument{}, persistenceError(err)
	}
	path, err := trialPath(slot)
	if err != nil {
		return trialDocument{}, persistenceError(err)
	}
	_, err = filestore.Read(ctx, filestore.ReadRequest{
		Destination:  &destination,
		Location:     filestore.Location{Root: root, Path: path},
		MaximumBytes: maximum,
	})
	if err != nil {
		return trialDocument{}, persistenceError(err)
	}
	document, err := decodeTrial(destination.Bytes())
	if err != nil {
		return trialDocument{}, persistenceError(err)
	}
	return document, nil
}

func requireTrialReceipt(
	ctx context.Context,
	root *os.Root,
	target TrialTarget,
) error {
	expected, err := newTrialDocument(target)
	if err != nil {
		return err
	}
	current, err := readTrial(ctx, root, target.slot)
	if err != nil {
		return conflictError(diagnosticActiveTrial, err)
	}
	if current != expected {
		return conflictError(diagnosticActiveTrial)
	}
	return nil
}

func writeTrial(
	ctx context.Context,
	root *os.Root,
	slot Slot,
	document trialDocument,
) error {
	encoded, err := encodeTrial(document)
	if err != nil {
		return persistenceError(err)
	}
	maximum, err := core.NewByteCount(uint64(len(encoded)))
	if err != nil {
		return persistenceError(err)
	}
	path, err := trialPath(slot)
	if err != nil {
		return persistenceError(err)
	}
	temporary, err := trialTemporaryPath(slot)
	if err != nil {
		return persistenceError(err)
	}
	recovery, err := filestore.Write(ctx, filestore.WriteRequest{
		Source: bytes.NewReader(encoded),
		Location: filestore.Location{
			Root: root, Path: path,
		},
		Temporary: temporary,
		Mode:      documentMode, Install: filestore.InstallCreate,
		MaximumBytes: maximum,
	})
	if err == nil {
		return nil
	}
	if recovery.Validate() != nil {
		return persistenceError(err)
	}
	recoveryErr := filestore.Recover(recoveryContext(ctx), recovery)
	if recoveryErr != nil {
		return persistenceError(errors.Join(err, recoveryErr))
	}
	return nil
}
