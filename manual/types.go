package manual

import "github.com/deliri/primitive/v2026/core"

const (
	// MaximumPages bounds one product manual.
	MaximumPages = 64
	// MaximumSectionItems bounds every repeated page section.
	MaximumSectionItems = 32
	// MaximumLineBytes bounds one printable line.
	MaximumLineBytes = 1024
	// MaximumTopicBytes bounds one canonical command topic.
	MaximumTopicBytes = 64
	// SchemaV1 identifies the sole machine manual schema.
	SchemaV1 Schema = "primitive.manual.v1"
)

const (
	headingTopics        = "Topics"
	headingUsage         = "Usage"
	headingSuccess       = "Success"
	headingRefusal       = "Refusal"
	headingPrerequisites = "Prerequisites"
	headingChanges       = "What changes"
	headingUnchanged     = "What does not change"
	headingExamples      = "Examples"
	headingTerms         = "Terms"
	headingRelated       = "Related topics"
)

// Topic binds a product-owned command enum to its canonical manual topic.
type Topic interface {
	comparable
	Validate() error
	ManualTopic() TopicName
}

// TopicName is one canonical lower-case command topic.
type TopicName string

// Line is one bounded printable customer-facing line.
type Line string

// Schema identifies a closed machine manual schema.
type Schema string

// Definition explains one term before a product relies on it.
type Definition struct {
	// Term is the customer-visible word or phrase being defined.
	Term Line `json:"term"`
	// Meaning explains Term without relying on undefined product jargon.
	Meaning Line `json:"meaning"`
}

// Outcome states observable success and refusal behavior.
type Outcome struct {
	// Success states what the customer can observe after completion.
	Success []Line `json:"success"`
	// Refusal states what the customer sees when work does not begin or finish.
	Refusal []Line `json:"refusal"`
}

// Page contains complete guidance for one product-owned command.
type Page[T Topic] struct {
	// Topic is the product-owned command enum documented by this page.
	Topic T
	// Summary explains the command's purpose in one short line.
	Summary Line
	// Usage contains copyable command forms.
	Usage []Line
	// Prerequisites state what must already be true.
	Prerequisites []Line
	// Changes state every customer-visible mutation.
	Changes []Line
	// Unchanged states important things the command deliberately leaves alone.
	Unchanged []Line
	// Definitions explain product terms before relying on them.
	Definitions []Definition
	// Examples contain copyable complete commands.
	Examples []Line
	// Outcome describes observable completion and refusal.
	Outcome Outcome
	// Related names other useful product-owned command topics.
	Related []T
}

// Book is one product's source of truth for help and manual output.
type Book[T Topic] struct {
	Title    Line
	Summary  Line
	Offering core.Offering
	Pages    []Page[T]
}

// View selects concise help or a complete manual page.
type View uint8

const (
	ViewUnknown View = iota
	ViewHelp
	ViewManual
	viewLimit
)

// SelectionMode selects the book index or one exact topic.
type SelectionMode uint8

const (
	SelectionModeUnknown SelectionMode = iota
	SelectionModeIndex
	SelectionModeTopic
	selectionModeLimit
)

// Selection identifies which part of a book to render.
type Selection[T Topic] struct {
	// Topic is zero for an index and exact for a topic selection.
	Topic T
	// Mode selects the index or Topic.
	Mode SelectionMode
}

// RenderRequest is one validated human-output request.
type RenderRequest[T Topic] struct {
	// Selection identifies the requested book surface.
	Selection Selection[T]
	// Book supplies the validated product facts.
	Book Book[T]
	// View selects concise help or complete manual detail.
	View View
}

// Report is the stable machine projection of one complete Book.
type Report struct {
	Schema   Schema        `json:"schema"`
	Title    Line          `json:"title"`
	Summary  Line          `json:"summary"`
	Offering core.Offering `json:"offering"`
	Pages    []PageReport  `json:"pages"`
}

// PageReport is one machine-readable manual page.
type PageReport struct {
	// Topic is the canonical product command topic.
	Topic TopicName `json:"topic"`
	// Summary explains the command purpose.
	Summary Line `json:"summary"`
	// Usage contains copyable command forms.
	Usage []Line `json:"usage"`
	// Prerequisites state required prior facts.
	Prerequisites []Line `json:"prerequisites"`
	// Changes state customer-visible mutations.
	Changes []Line `json:"changes"`
	// Unchanged states important non-mutations.
	Unchanged []Line `json:"unchanged"`
	// Definitions explain product terms.
	Definitions []Definition `json:"definitions"`
	// Examples contain copyable commands.
	Examples []Line `json:"examples"`
	// Outcome describes success and refusal.
	Outcome Outcome `json:"outcome"`
	// Related names other documented topics.
	Related []TopicName `json:"related"`
}
