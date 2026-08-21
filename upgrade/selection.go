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

type selectionRevision uint8

const (
	selectionRevisionUnknown selectionRevision = iota
	selectionRevisionCurrent
	selectionRevisionLimit
)

func (r selectionRevision) Validate() error {
	if r != selectionRevisionCurrent {
		return contractError(diagnosticSelection)
	}
	return nil
}

func (r selectionRevision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return []byte("1"), nil
}

func (r *selectionRevision) UnmarshalJSON(data []byte) error {
	if r == nil || !bytes.Equal(data, []byte("1")) {
		return errors.Join(core.ErrJSONContract, contractError(diagnosticSelection))
	}
	*r = selectionRevisionCurrent
	return nil
}

type selectionDocument struct {
	Artifact release.Artifact
	Revision selectionRevision
	Slot     Slot
}

type selectionWire struct {
	Revision *selectionRevision `json:"revision"`
	Slot     *Slot              `json:"slot"`
	Artifact *release.Artifact  `json:"artifact"`
}

func (d selectionDocument) Validate() error {
	if err := d.Revision.Validate(); err != nil {
		return contractError(diagnosticSelection, err)
	}
	if err := d.Slot.Validate(); err != nil {
		return contractError(diagnosticSelection, err)
	}
	if err := d.Artifact.Validate(); err != nil {
		return contractError(diagnosticSelection, err)
	}
	return nil
}

func (d selectionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	revision, slot, artifact := d.Revision, d.Slot, d.Artifact
	return json.Marshal(selectionWire{
		Revision: &revision, Slot: &slot, Artifact: &artifact,
	})
}

func (d *selectionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, contractError(diagnosticSelection))
	}
	decoded, err := decodeSelectionStructure(data)
	if err != nil {
		return err
	}
	if decoded.Revision == nil || decoded.Slot == nil || decoded.Artifact == nil {
		return errors.Join(core.ErrJSONContract, contractError(diagnosticSelection))
	}
	candidate := selectionDocument{
		Revision: *decoded.Revision,
		Slot:     *decoded.Slot,
		Artifact: *decoded.Artifact,
	}
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

func selectionJSONLimits() (core.StrictJSONLimits, error) {
	maximum, err := core.NewByteCount(selectionDocumentMaximumBytes)
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

func decodeSelectionStructure(data []byte) (selectionWire, error) {
	limits, err := selectionJSONLimits()
	if err != nil {
		return selectionWire{}, contractError(err)
	}
	value, err := core.DecodeStrictJSONStructure[selectionWire](data, limits)
	if err != nil {
		return selectionWire{}, contractError(core.ErrJSONContract, diagnosticJSON, err)
	}
	return value, nil
}

func encodeSelection(document selectionDocument) ([]byte, error) {
	limits, err := selectionJSONLimits()
	if err != nil {
		return nil, contractError(err)
	}
	encoded, err := core.EncodeValidatedJSON(document, limits)
	if err != nil {
		return nil, contractError(core.ErrJSONContract, diagnosticJSON, err)
	}
	return encoded, nil
}

func decodeSelection(data []byte) (selectionDocument, error) {
	var decoded selectionDocument
	if err := json.Unmarshal(data, &decoded); err != nil {
		return selectionDocument{}, contractError(core.ErrJSONContract, diagnosticJSON, err)
	}
	encoded, err := encodeSelection(decoded)
	if err != nil || !bytes.Equal(encoded, data) {
		return selectionDocument{}, contractError(core.ErrJSONContract, diagnosticJSON, err)
	}
	return decoded, nil
}

func readSelection(
	ctx context.Context,
	root *os.Root,
) (selectionDocument, error) {
	var destination bytes.Buffer
	maximum, err := core.NewByteCount(selectionDocumentMaximumBytes)
	if err != nil {
		return selectionDocument{}, persistenceError(err)
	}
	path, err := selectionPath()
	if err != nil {
		return selectionDocument{}, persistenceError(err)
	}
	_, err = filestore.Read(ctx, filestore.ReadRequest{
		Destination: &destination,
		Location: filestore.Location{
			Root: root, Path: path,
		},
		MaximumBytes: maximum,
	})
	if err != nil {
		return selectionDocument{}, persistenceError(err)
	}
	document, err := decodeSelection(destination.Bytes())
	if err != nil {
		return selectionDocument{}, persistenceError(err)
	}
	return document, nil
}

func writeSelection(
	ctx context.Context,
	root *os.Root,
	document selectionDocument,
	mode filestore.InstallMode,
) error {
	encoded, err := encodeSelection(document)
	if err != nil {
		return persistenceError(err)
	}
	maximum, err := core.NewByteCount(uint64(len(encoded)))
	if err != nil {
		return persistenceError(err)
	}
	path, err := selectionPath()
	if err != nil {
		return persistenceError(err)
	}
	temporary, err := selectionTemporaryPath()
	if err != nil {
		return persistenceError(err)
	}
	if err := filestore.Remove(recoveryContext(ctx), filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: temporary},
	}); err != nil {
		return persistenceError(err)
	}
	recovery, err := filestore.Write(ctx, filestore.WriteRequest{
		Source: bytes.NewReader(encoded),
		Location: filestore.Location{
			Root: root, Path: path,
		},
		Temporary: temporary,
		Mode:      documentMode, Install: mode, MaximumBytes: maximum,
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
