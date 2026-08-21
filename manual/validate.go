package manual

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(text string) error { return errors.Join(core.ErrManualContract, errors.New(text)) }

// NewTopicName validates and returns one canonical topic name.
func NewTopicName(value string) (TopicName, error) {
	name := TopicName(value)
	if err := name.Validate(); err != nil {
		return "", err
	}
	return name, nil
}

// ParseLine validates and returns one printable manual line.
func ParseLine(value string) (Line, error) {
	line := Line(value)
	if err := line.Validate(); err != nil {
		return "", err
	}
	return line, nil
}

// NewSchema validates and returns the supported machine schema.
func NewSchema(value string) (Schema, error) {
	schema := Schema(value)
	if err := schema.Validate(); err != nil {
		return "", err
	}
	return schema, nil
}

// String returns canonical topic text.
func (n TopicName) String() string { return string(n) }

// String returns customer-facing line text.
func (l Line) String() string { return string(l) }

// Validate rejects unsupported machine schemas.
func (s Schema) Validate() error {
	if s != SchemaV1 {
		return contractError("manual report schema is unsupported")
	}
	return nil
}

// String returns canonical schema text.
func (s Schema) String() string { return string(s) }

// Validate rejects an empty, oversized, or non-canonical topic.
func (n TopicName) Validate() error {
	value := string(n)
	if len(value) == 0 || len(value) > MaximumTopicBytes || !utf8.ValidString(value) {
		return contractError("manual topic has invalid extent")
	}
	for index, current := range value {
		if !validTopicRune(index, current) {
			return contractError("manual topic is not canonical")
		}
	}
	if value[len(value)-1] == '-' || strings.Contains(value, "--") {
		return contractError("manual topic is not canonical")
	}
	return nil
}

func validTopicRune(index int, current rune) bool {
	if current >= 'a' && current <= 'z' {
		return true
	}
	if current >= '0' && current <= '9' {
		return true
	}
	return index > 0 && current == '-'
}

// Validate rejects empty, oversized, untrimmed, or control-bearing text.
func (l Line) Validate() error {
	value := string(l)
	if len(value) == 0 || len(value) > MaximumLineBytes || !utf8.ValidString(value) {
		return contractError("manual line has invalid extent")
	}
	if strings.TrimSpace(value) != value {
		return contractError("manual line is not trimmed")
	}
	for _, current := range value {
		if unicode.IsControl(current) {
			return contractError("manual line contains control text")
		}
	}
	return nil
}

// Validate rejects an incomplete definition.
func (d Definition) Validate() error {
	if err := d.Term.Validate(); err != nil {
		return err
	}
	return d.Meaning.Validate()
}

// Validate requires observable success and refusal facts.
func (o Outcome) Validate() error {
	if err := requiredLines(o.Success); err != nil {
		return err
	}
	return requiredLines(o.Refusal)
}

// Validate rejects incomplete or contradictory page facts.
func (p Page[T]) Validate() error {
	if err := validTopic(p.Topic); err != nil {
		return err
	}
	if err := p.Summary.Validate(); err != nil {
		return err
	}
	if err := validateRequiredSections(p.Usage, p.Changes, p.Unchanged, p.Examples); err != nil {
		return err
	}
	if err := optionalLines(p.Prerequisites); err != nil {
		return err
	}
	if err := validateDefinitions(p.Definitions); err != nil {
		return err
	}
	if err := p.Outcome.Validate(); err != nil {
		return err
	}
	return validateRelated(p.Topic, p.Related)
}

func validateRequiredSections(sections ...[]Line) error {
	for _, lines := range sections {
		if err := requiredLines(lines); err != nil {
			return err
		}
	}
	return nil
}

func validateDefinitions(definitions []Definition) error {
	if len(definitions) > MaximumSectionItems {
		return contractError("manual definitions exceed their item limit")
	}
	seen := make(map[Line]struct{}, len(definitions))
	for _, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return err
		}
		if _, exists := seen[definition.Term]; exists {
			return contractError("manual definitions repeat a term")
		}
		seen[definition.Term] = struct{}{}
	}
	return nil
}

func validateRelated[T Topic](owner T, related []T) error {
	if len(related) > MaximumSectionItems {
		return contractError("manual related topics exceed their item limit")
	}
	seen := make(map[T]struct{}, len(related))
	for _, topic := range related {
		if err := validTopic(topic); err != nil {
			return err
		}
		if topic == owner {
			return contractError("manual page relates to itself")
		}
		if _, exists := seen[topic]; exists {
			return contractError("manual page repeats a related topic")
		}
		seen[topic] = struct{}{}
	}
	return nil
}

// Validate rejects incomplete, duplicate, or unbound book facts.
func (b Book[T]) Validate() error {
	if err := b.Offering.Validate(); err != nil {
		return errors.Join(core.ErrManualContract, err)
	}
	if err := b.Title.Validate(); err != nil {
		return err
	}
	if err := b.Summary.Validate(); err != nil {
		return err
	}
	if len(b.Pages) == 0 || len(b.Pages) > MaximumPages {
		return contractError("manual book has invalid page count")
	}
	seen, err := validateBookPages(b.Pages)
	if err != nil {
		return err
	}
	return validateBookRelations(b.Pages, seen)
}

func validateBookPages[T Topic](pages []Page[T]) (map[TopicName]struct{}, error) {
	seen := make(map[TopicName]struct{}, len(pages))
	for _, page := range pages {
		if err := page.Validate(); err != nil {
			return nil, err
		}
		name := page.Topic.ManualTopic()
		if _, exists := seen[name]; exists {
			return nil, contractError("manual book repeats a topic")
		}
		seen[name] = struct{}{}
	}
	return seen, nil
}

func validateBookRelations[T Topic](pages []Page[T], seen map[TopicName]struct{}) error {
	for _, page := range pages {
		for _, related := range page.Related {
			if _, exists := seen[related.ManualTopic()]; !exists {
				return contractError("manual related topic is absent from the book")
			}
		}
	}
	return nil
}

// Validate rejects views outside the closed domain.
func (v View) Validate() error {
	if v <= ViewUnknown || v >= viewLimit {
		return contractError("manual view is outside the closed domain")
	}
	return nil
}

// Validate rejects invalid or contradictory selection facts.
func (s Selection[T]) Validate() error {
	if s.Mode <= SelectionModeUnknown || s.Mode >= selectionModeLimit {
		return contractError("manual selection is outside the closed domain")
	}
	var zero T
	if s.Mode == SelectionModeIndex && s.Topic != zero {
		return contractError("manual index selection carries a topic")
	}
	if s.Mode == SelectionModeTopic {
		return validTopic(s.Topic)
	}
	return nil
}

// Validate rejects invalid requests and absent selected topics.
func (r RenderRequest[T]) Validate() error {
	if err := r.Book.Validate(); err != nil {
		return err
	}
	if err := r.View.Validate(); err != nil {
		return err
	}
	if err := r.Selection.Validate(); err != nil {
		return err
	}
	if r.Selection.Mode == SelectionModeTopic {
		_, err := findPage(r.Book, r.Selection.Topic)
		return err
	}
	return nil
}

func validTopic[T Topic](topic T) error {
	if err := topic.Validate(); err != nil {
		return errors.Join(core.ErrManualContract, err)
	}
	return topic.ManualTopic().Validate()
}
func requiredLines(lines []Line) error {
	if len(lines) == 0 {
		return contractError("manual section is required")
	}
	return optionalLines(lines)
}
func optionalLines(lines []Line) error {
	if len(lines) > MaximumSectionItems {
		return contractError("manual section exceeds its item limit")
	}
	seen := make(map[Line]struct{}, len(lines))
	for _, line := range lines {
		if err := line.Validate(); err != nil {
			return err
		}
		if _, exists := seen[line]; exists {
			return contractError("manual section repeats a line")
		}
		seen[line] = struct{}{}
	}
	return nil
}
func findPage[T Topic](book Book[T], topic T) (Page[T], error) {
	for _, page := range book.Pages {
		if page.Topic == topic {
			return page, nil
		}
	}
	return Page[T]{}, contractError("manual topic is absent from the book")
}
