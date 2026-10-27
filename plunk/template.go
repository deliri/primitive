package plunk

import (
	"context"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// TemplateKind is the provider's declared subscription/footer behavior.
// Applications decide which kind their selected operation requires.
type TemplateKind uint8

const (
	TemplateKindInvalid TemplateKind = iota
	TemplateMarketing
	TemplateTransactional
	TemplateHeadless
	templateKindLimit
)

func (k TemplateKind) Validate() error {
	if k == TemplateKindInvalid || k >= templateKindLimit {
		return core.ErrPlunkContract
	}
	return nil
}

func (k TemplateKind) String() string {
	switch k {
	case TemplateMarketing:
		return "MARKETING"
	case TemplateTransactional:
		return "TRANSACTIONAL"
	case TemplateHeadless:
		return "HEADLESS"
	default:
		return "invalid"
	}
}

func (k TemplateKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, err
	}
	return core.MarshalCanonicalJSONString(k.String())
}

func (k *TemplateKind) UnmarshalJSON(raw []byte) error {
	if k == nil {
		return core.ErrPlunkContract
	}
	text, err := core.DecodeJSONStringToken(raw)
	if err != nil {
		return wireDocumentError(err)
	}
	for next := TemplateMarketing; next < templateKindLimit; next++ {
		if text == next.String() {
			*k = next
			return nil
		}
	}
	return core.ErrPlunkContract
}

// TemplateMetadata is an explicit projection of GET /templates/:id. Content,
// styling, project fields and future provider members are not retained.
type TemplateMetadata struct {
	ID   string       `json:"id"`
	Kind TemplateKind `json:"type"`
}

func (m TemplateMetadata) Validate() error {
	if m.ID == "" {
		return core.ErrPlunkContract
	}
	return m.Kind.Validate()
}

// DecodeTemplateMetadata admits the two policy facts from the full provider
// document. Unknown members are deliberately ignored; grammar, duplicates and
// all caller-owned byte/depth/item/field extents still apply to the whole value.
// No template selection, UUID convention or marketing policy is inferred.
func DecodeTemplateMetadata(ctx context.Context, raw []byte, limits core.StrictJSONLimits) (TemplateMetadata, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return TemplateMetadata{}, wireDocumentError(err)
	}
	if err := limits.ValidateDocument(raw); err != nil {
		return TemplateMetadata{}, wireDocumentError(err)
	}
	metadata, err := jsonio.DecodeValue[TemplateMetadata](raw, jsonio.ProjectionReadPolicy{Members: jsonio.MembersIgnoreExtra})
	if err != nil {
		return TemplateMetadata{}, wireDocumentError(err)
	}
	if err := metadata.Validate(); err != nil {
		return TemplateMetadata{}, wireDocumentError(err)
	}
	if err := contextstate.Validate(ctx); err != nil {
		return TemplateMetadata{}, wireDocumentError(err)
	}
	return metadata, nil
}
