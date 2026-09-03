package sourceobservation

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// BuildContext is one exact Go source-selection context. ID is the stable
// claim-facing coordinate; GOOS, GOARCH, tags, and CGO are observed facts.
type BuildContext struct {
	ID     ContextID `json:"id"`
	GOOS   Symbol    `json:"goos"`
	GOARCH Symbol    `json:"goarch"`
	Tags   []Symbol  `json:"tags"`
	CGO    bool      `json:"cgo"`
}

func (c BuildContext) Validate() error {
	if err := contractJoin(c.ID.Validate(), c.GOOS.Validate(), c.GOARCH.Validate()); err != nil {
		return err
	}
	for index := range c.Tags {
		if err := c.Tags[index].Validate(); err != nil {
			return err
		}
		if index > 0 && c.Tags[index-1].String() >= c.Tags[index].String() {
			return conflictError(errors.New("source observation build tags are duplicated or not canonical"))
		}
	}
	return nil
}

func validateBuildContexts(values []BuildContext) error {
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		if index > 0 && values[index-1].ID.String() >= values[index].ID.String() {
			return conflictError(errors.New("source observation build contexts are duplicated or not canonical"))
		}
	}
	return nil
}

// Digest binds one context independently of its project rollup.
func (c BuildContext) Digest() (core.SHA256Digest, error) {
	if err := c.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c)
	if err != nil {
		return core.SHA256Digest{}, errors.Join(core.ErrJSONContract, core.ErrSourceObservationContract, err)
	}
	return core.SHA256Of(encoded), nil
}
