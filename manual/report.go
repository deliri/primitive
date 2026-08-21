package manual

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// Project validates and copies a Book into its stable machine report.
func Project[T Topic](book Book[T]) (Report, error) {
	if err := book.Validate(); err != nil {
		return Report{}, err
	}
	report := Report{Schema: SchemaV1, Offering: book.Offering, Title: book.Title, Summary: book.Summary, Pages: make([]PageReport, 0, len(book.Pages))}
	for _, page := range book.Pages {
		report.Pages = append(report.Pages, projectPage(page))
	}
	return report, nil
}

func projectPage[T Topic](page Page[T]) PageReport {
	related := make([]TopicName, 0, len(page.Related))
	for _, topic := range page.Related {
		related = append(related, topic.ManualTopic())
	}
	return PageReport{Topic: page.Topic.ManualTopic(), Summary: page.Summary, Usage: clone(page.Usage), Prerequisites: clone(page.Prerequisites), Changes: clone(page.Changes), Unchanged: clone(page.Unchanged), Definitions: append([]Definition(nil), page.Definitions...), Examples: clone(page.Examples), Outcome: Outcome{Success: clone(page.Outcome.Success), Refusal: clone(page.Outcome.Refusal)}, Related: related}
}

func clone(lines []Line) []Line { return append([]Line(nil), lines...) }

// Validate rejects machine reports outside the stable schema and bounds.
func (r Report) Validate() error {
	if err := r.Schema.Validate(); err != nil {
		return err
	}
	if err := r.Offering.Validate(); err != nil {
		return errors.Join(core.ErrManualContract, err)
	}
	if err := r.Title.Validate(); err != nil {
		return err
	}
	if err := r.Summary.Validate(); err != nil {
		return err
	}
	if len(r.Pages) == 0 || len(r.Pages) > MaximumPages {
		return contractError("manual report has invalid page count")
	}
	seen, err := validateReportPages(r.Pages)
	if err != nil {
		return err
	}
	return validateReportRelations(r.Pages, seen)
}

func validateReportPages(pages []PageReport) (map[TopicName]struct{}, error) {
	seen := make(map[TopicName]struct{}, len(pages))
	for _, page := range pages {
		if err := page.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seen[page.Topic]; exists {
			return nil, contractError("manual report repeats a topic")
		}
		seen[page.Topic] = struct{}{}
	}
	return seen, nil
}

func validateReportRelations(pages []PageReport, seen map[TopicName]struct{}) error {
	for _, page := range pages {
		for _, related := range page.Related {
			if _, exists := seen[related]; !exists {
				return contractError("manual report related topic is absent")
			}
			if related == page.Topic {
				return contractError("manual report page relates to itself")
			}
		}
	}
	return nil
}

// Validate rejects incomplete machine page facts.
func (p PageReport) Validate() error {
	if err := p.Topic.Validate(); err != nil {
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
	return validateTopicNames(p.Related)
}

func validateTopicNames(names []TopicName) error {
	if len(names) > MaximumSectionItems {
		return contractError("manual related topics exceed their item limit")
	}
	seen := make(map[TopicName]struct{}, len(names))
	for _, name := range names {
		if err := name.Validate(); err != nil {
			return err
		}
		if _, exists := seen[name]; exists {
			return contractError("manual report repeats a related topic")
		}
		seen[name] = struct{}{}
	}
	return nil
}

// WriteJSON validates and streams one canonical JSON report.
func WriteJSON(destination io.Writer, report Report) error {
	if destination == nil {
		return contractError("manual destination is nil")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	if err := json.NewEncoder(exactWriter{destination: destination}).Encode(report); err != nil {
		return err
	}
	return nil
}
