package runnercontrol

import "github.com/deliri/primitive/v2026/projectstandards"

const subjectUnitPrefix = "primitive-run-"

func subjectUnitName(experiment projectstandards.ExperimentID) string {
	return subjectUnitPrefix + experiment.String()
}
