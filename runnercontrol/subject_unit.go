package runnercontrol

import "github.com/deliri/primitive/v2026/standard"

const subjectUnitPrefix = "primitive-run-"

func subjectUnitName(experiment standard.ExperimentID) string {
	return subjectUnitPrefix + experiment.String()
}
