package exchange

import (
	"context"
	"io"
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// MultipartName owns one exact field name. Native multipart.Writer owns its
// quoting; control characters and empty names cannot enter a MIME header.
type MultipartName struct{ value string }

// ParseMultipartName admits a UTF-8 field name without header delimiters.
func ParseMultipartName(value string) (MultipartName, error) {
	candidate := MultipartName{value: value}
	if err := candidate.Validate(); err != nil {
		return MultipartName{}, err
	}
	return candidate, nil
}

// Validate refuses absent or header-injecting names without an extent quota.
func (n MultipartName) Validate() error { return validateMultipartHeaderText(n.value) }

// String projects the admitted name as native MIME data.
func (n MultipartName) String() string { return n.value }

// MultipartFileName owns a filename rather than an operating-system path.
type MultipartFileName struct{ value string }

// ParseMultipartFileName admits a UTF-8 basename without path separators.
func ParseMultipartFileName(value string) (MultipartFileName, error) {
	candidate := MultipartFileName{value: value}
	if err := candidate.Validate(); err != nil {
		return MultipartFileName{}, err
	}
	return candidate, nil
}

// Validate refuses paths and header-injecting filenames before source custody.
func (n MultipartFileName) Validate() error {
	if err := validateMultipartHeaderText(n.value); err != nil {
		return err
	}
	if n.value == "." || n.value == ".." || strings.ContainsAny(n.value, "/\\") {
		return core.ErrExchangeContract
	}
	return nil
}

// String projects the admitted filename for Go's native quoting.
func (n MultipartFileName) String() string { return n.value }

func validateMultipartHeaderText(value string) error {
	if value == "" || !utf8.ValidString(value) {
		return core.ErrExchangeContract
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return core.ErrExchangeContract
		}
	}
	return nil
}

// MultipartText owns field data. Empty text is valid when explicitly admitted;
// arbitrary controls remain data and cannot affect the part's native headers.
type MultipartText struct {
	value string
	set   bool
}

// ParseMultipartText admits exact UTF-8 data without trimming or a byte quota.
func ParseMultipartText(value string) (MultipartText, error) {
	candidate := MultipartText{value: value, set: true}
	if err := candidate.Validate(); err != nil {
		return MultipartText{}, err
	}
	return candidate, nil
}

// Validate distinguishes admitted empty text from an absent value.
func (t MultipartText) Validate() error {
	if !t.set || !utf8.ValidString(t.value) {
		return core.ErrExchangeContract
	}
	return nil
}

// String projects exact admitted field data.
func (t MultipartText) String() string { return t.value }

// MultipartPartKind is the closed field/file framing choice.
type MultipartPartKind uint8

const (
	// MultipartPartUnknown is invalid.
	MultipartPartUnknown MultipartPartKind = iota
	// MultipartPartField carries one admitted text value.
	MultipartPartField
	// MultipartPartFile carries one scope-owned interruptible byte source.
	MultipartPartFile
)

// IsValid reports membership in the native framing domain.
func (k MultipartPartKind) IsValid() bool { return k == MultipartPartField || k == MultipartPartFile }

// Validate refuses unknown or future framing choices.
func (k MultipartPartKind) Validate() error {
	if !k.IsValid() {
		return core.ErrExchangeContract
	}
	return nil
}

// String renders the framing choice for diagnostics, never native control.
func (k MultipartPartKind) String() string {
	switch k {
	case MultipartPartField:
		return "field"
	case MultipartPartFile:
		return "file"
	default:
		return ""
	}
}

// OffWireEnum marks a compiler-owned producer choice rather than a wire number.
func (MultipartPartKind) OffWireEnum() {}

// MultipartPart transfers Source custody only after validation and admission of
// a file part. Exchange closes that source exactly once on finish or cancel.
// Sources for invalid or not-yet-admitted parts remain owned by the iterator.
// Source.Close must unblock its Read and return; uncooperative custom closers
// cannot be made interruptible by a caller-independent timeout. Fields have no
// Source or FileName; files have no Text. Repeated names preserve input order.
type MultipartPart struct {
	Source   io.ReadCloser
	Name     MultipartName
	FileName MultipartFileName
	Text     MultipartText
	Kind     MultipartPartKind
}

// Validate refuses contradictory field/file agreements before custody transfer.
func (p MultipartPart) Validate() error {
	if err := p.Name.Validate(); err != nil {
		return err
	}
	switch p.Kind {
	case MultipartPartField:
		if p.Source != nil || p.FileName != (MultipartFileName{}) {
			return core.ErrExchangeContract
		}
		return p.Text.Validate()
	case MultipartPartFile:
		if core.ReaderIsNil(p.Source) || p.Text != (MultipartText{}) {
			return core.ErrExchangeContract
		}
		return p.FileName.Validate()
	default:
		return core.ErrExchangeContract
	}
}

// MultipartSource is borrowed only during Use. Reader exposes no close method;
// the enclosing scope owns its native pipe and producing goroutine.
type MultipartSource struct {
	Reader      io.Reader
	ContentType core.HTTPMediaType
}

// Validate admits the reader and the native multipart/form-data content type.
func (s MultipartSource) Validate() error {
	if core.ReaderIsNil(s.Reader) {
		return core.ErrExchangeContract
	}
	want, err := core.ParseHTTPMediaType(core.AIMultipartMediaType)
	if err != nil {
		return err
	}
	matches, err := s.ContentType.SameBase(want)
	if err != nil {
		return err
	}
	if !matches {
		return core.ErrExchangeContract
	}
	return nil
}

// MultipartSourceRequest supplies a forward-only part iterator and synchronous
// consumer. Parts must stop when yield refuses further work and must not block
// uninterruptibly between yields. Use owns any other effects and must honor its
// context. No request world, replay, aggregate body or transfer ceiling is built.
type MultipartSourceRequest struct {
	Parts iter.Seq[MultipartPart]
	Use   func(context.Context, MultipartSource) error
}

// Validate refuses absent producer/consumer callbacks without invoking either.
func (r MultipartSourceRequest) Validate() error {
	if r.Parts == nil || r.Use == nil {
		return core.ErrExchangeContract
	}
	return nil
}

var (
	_ core.Validatable = MultipartName{}
	_ core.Validatable = MultipartFileName{}
	_ core.Validatable = MultipartText{}
	_ core.Validatable = MultipartPart{}
	_ core.Validatable = MultipartSource{}
	_ core.Validatable = MultipartSourceRequest{}
	_ core.OffWireEnum = MultipartPartUnknown
)
