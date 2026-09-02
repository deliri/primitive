package projectstandards

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestKnowledgeRecordsExposeExactAuthoredFieldPrefixes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		record       reflect.Type
		wantObserved []string
		wantAuthor   []string
	}{
		{
			name:   "file knowledge separates source coordinates from authored meaning",
			record: reflect.TypeFor[Component](),
			wantObserved: []string{
				"Created", "Path", "Language", "Package", "Changed", "Kind",
			},
			wantAuthor: []string{
				"AuthorPurpose", "AuthorTitle", "AuthorRemoval", "AuthorReasons",
				"AuthorOwns", "AuthorDoesNotOwn", "AuthorFeatures",
			},
		},
		{
			name:   "project knowledge separates source coordinates from authored meaning",
			record: reflect.TypeFor[ProductKnowledge](),
			wantObserved: []string{
				"Created", "SourcePath", "Changed",
			},
			wantAuthor: []string{
				"AuthorTitle", "AuthorProblem", "AuthorPurpose", "AuthorAudience",
				"AuthorPromise", "AuthorReasons", "AuthorOwns", "AuthorNonGoals",
				"AuthorFeatures",
			},
		},
		{
			name:   "package knowledge separates source coordinates from authored meaning",
			record: reflect.TypeFor[PackageKnowledge](),
			wantObserved: []string{
				"Created", "Path", "Changed",
			},
			wantAuthor: []string{
				"AuthorTitle", "AuthorPurpose", "AuthorAudience", "AuthorRole", "AuthorValue",
				"AuthorSteward", "AuthorSubstrate", "AuthorRuntime", "AuthorRemoval",
				"AuthorProblem", "AuthorReasons", "AuthorOwns", "AuthorDoesNotOwn",
				"AuthorUsage", "AuthorFeatures", "AuthorComplexity", "AuthorAssurance",
				"AuthorCapabilityOwnership",
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			wantCount := len(testCase.wantObserved) + len(testCase.wantAuthor)
			if testCase.record.NumField() != wantCount {
				t.Fatalf("%s field count = %d, want %d classified fields", testCase.record.Name(), testCase.record.NumField(), wantCount)
			}
			for field := range testCase.record.Fields() {
				observed := slices.Contains(testCase.wantObserved, field.Name)
				authored := slices.Contains(testCase.wantAuthor, field.Name)
				if observed == authored {
					t.Fatalf("%s.%s classification = (observed %t, authored %t), want exactly one", testCase.record.Name(), field.Name, observed, authored)
				}
				if authored && !strings.HasPrefix(field.Name, "Author") {
					t.Fatalf("%s authored field = %q, want Author prefix", testCase.record.Name(), field.Name)
				}
			}
		})
	}
}
