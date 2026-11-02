package gcsobjects

import "github.com/deliri/primitive/v2026/core"

// GCSListRange is the provider's optional lexical object-name interval.
// Start is inclusive, End is exclusive, and a zero bound is omitted. The zero
// range preserves an ordinary prefix listing. Equal nonzero bounds are empty.
// https://docs.cloud.google.com/storage/docs/json_api/v1/objects/list
type GCSListRange struct {
	Start GCSObjectName
	End   GCSObjectName
}

func (r GCSListRange) IsZero() bool { return r == (GCSListRange{}) }

func (r GCSListRange) Validate() error {
	if r.Start != (GCSObjectName{}) {
		if err := r.Start.Validate(); err != nil {
			return err
		}
	}
	if r.End != (GCSObjectName{}) {
		if err := r.End.Validate(); err != nil {
			return err
		}
	}
	if r.Start != (GCSObjectName{}) && r.End != (GCSObjectName{}) && r.Start.String() > r.End.String() {
		return core.ErrObjectStoreContract
	}
	return nil
}

func (r GCSListRange) contains(name GCSObjectName) bool {
	return (r.Start == (GCSObjectName{}) || name.String() >= r.Start.String()) &&
		(r.End == (GCSObjectName{}) || name.String() < r.End.String())
}
