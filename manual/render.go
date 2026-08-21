package manual

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// WriteText validates and streams deterministic plain text.
func WriteText[T Topic](destination io.Writer, request RenderRequest[T]) error {
	if destination == nil {
		return contractError("manual destination is nil")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Selection.Mode == SelectionModeIndex {
		return writeIndex(destination, request.Book)
	}
	page, err := findPage(request.Book, request.Selection.Topic)
	if err != nil {
		return err
	}
	if request.View == ViewHelp {
		return writeHelp(destination, page)
	}
	return writeManual(destination, page)
}

func writeIndex[T Topic](destination io.Writer, book Book[T]) error {
	if err := heading(destination, book.Title); err != nil {
		return err
	}
	if err := paragraph(destination, book.Summary); err != nil {
		return err
	}
	if err := heading(destination, headingTopics); err != nil {
		return err
	}
	for _, page := range book.Pages {
		if err := bulletPair(destination, Line(page.Topic.ManualTopic()), page.Summary); err != nil {
			return err
		}
	}
	return nil
}

func writeHelp[T Topic](destination io.Writer, page Page[T]) error {
	if err := heading(destination, Line(page.Topic.ManualTopic())); err != nil {
		return err
	}
	if err := paragraph(destination, page.Summary); err != nil {
		return err
	}
	if err := section(destination, headingUsage, page.Usage); err != nil {
		return err
	}
	if err := section(destination, headingSuccess, page.Outcome.Success); err != nil {
		return err
	}
	return section(destination, headingRefusal, page.Outcome.Refusal)
}

func writeManual[T Topic](destination io.Writer, page Page[T]) error {
	if err := writeHelp(destination, page); err != nil {
		return err
	}
	sections := []struct {
		heading Line
		lines   []Line
	}{
		{heading: headingPrerequisites, lines: page.Prerequisites},
		{heading: headingChanges, lines: page.Changes},
		{heading: headingUnchanged, lines: page.Unchanged},
		{heading: headingExamples, lines: page.Examples},
	}
	for _, value := range sections {
		if err := section(destination, value.heading, value.lines); err != nil {
			return err
		}
	}
	if err := writeDefinitions(destination, page.Definitions); err != nil {
		return err
	}
	return writeRelated(destination, page.Related)
}

func writeDefinitions(destination io.Writer, definitions []Definition) error {
	if len(definitions) == 0 {
		return nil
	}
	if err := heading(destination, headingTerms); err != nil {
		return err
	}
	for _, definition := range definitions {
		if err := bulletPair(destination, definition.Term, definition.Meaning); err != nil {
			return err
		}
	}
	return output(destination, "\n")
}

func writeRelated[T Topic](destination io.Writer, related []T) error {
	if len(related) == 0 {
		return nil
	}
	if err := heading(destination, headingRelated); err != nil {
		return err
	}
	for _, topic := range related {
		if err := bullet(destination, Line(topic.ManualTopic())); err != nil {
			return err
		}
	}
	return nil
}

func heading(destination io.Writer, value Line) error {
	if err := output(destination, string(value)); err != nil {
		return err
	}
	return output(destination, "\n\n")
}
func paragraph(destination io.Writer, value Line) error { return heading(destination, value) }
func section(destination io.Writer, title Line, lines []Line) error {
	if len(lines) == 0 {
		return nil
	}
	if err := heading(destination, title); err != nil {
		return err
	}
	for _, line := range lines {
		if err := bullet(destination, line); err != nil {
			return err
		}
	}
	return output(destination, "\n")
}
func bullet(destination io.Writer, value Line) error {
	if err := output(destination, "- "); err != nil {
		return err
	}
	if err := output(destination, string(value)); err != nil {
		return err
	}
	return output(destination, "\n")
}
func bulletPair(destination io.Writer, first, second Line) error {
	if err := output(destination, "- "); err != nil {
		return err
	}
	if err := output(destination, string(first)); err != nil {
		return err
	}
	if err := output(destination, ": "); err != nil {
		return err
	}
	if err := output(destination, string(second)); err != nil {
		return err
	}
	return output(destination, "\n")
}
func output(destination io.Writer, value string) error {
	_, err := io.WriteString(exactWriter{destination: destination}, value)
	return err
}

type exactWriter struct{ destination io.Writer }

func (w exactWriter) Write(value []byte) (int, error) {
	written, err := w.destination.Write(value)
	if err != nil {
		return written, errors.Join(core.ErrManualWrite, err)
	}
	if written != len(value) {
		return written, errors.Join(core.ErrManualWrite, io.ErrShortWrite)
	}
	return written, nil
}
