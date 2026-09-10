package manual_test

import (
	"errors"
	"regexp"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/manual"
)

// The independent grammar oracle pins exact admission, rather than merely
// asking the production validator to agree with its own constructor.
func FuzzTopicNameGrammar(f *testing.F) {
	grammar, err := regexp.Compile(`^[a-z0-9]+(-[a-z0-9]+)*(\.[a-z0-9]+(-[a-z0-9]+)*)*$`)
	if err != nil {
		f.Fatalf("compile grammar = %v, want nil", err)
	}
	for _, seed := range []string{"compile", "work.list", "anvil.file.run", "work-item.read-all"} {
		name, err := manual.NewTopicName(seed)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(name.String())
	}
	for _, seed := range []string{"work..list", "work.-list", "work-.list", "Work.list", "", "x\n"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		want := grammar.MatchString(text)
		got, err := manual.NewTopicName(text)
		if (err == nil) != want {
			t.Fatalf("NewTopicName(%q) = %q/%v, want admitted=%t", text, got, err, want)
		}
		if !want {
			if got != "" || !errors.Is(err, core.ErrManualContract) {
				t.Fatalf("NewTopicName(refused) = %q/%v, want zero and typed refusal", got, err)
			}
			return
		}
		if got.String() != text || got.Validate() != nil {
			t.Fatalf("NewTopicName(accepted) = %q, want exact original %q", got, text)
		}
	})
}
