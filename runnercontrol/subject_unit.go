package runnercontrol

import "github.com/deliri/primitive/v2026/runprotocol"

const subjectUnitPrefix = "primitive-run-"

func subjectUnitName(experiment runprotocol.ExperimentID) string {
	return subjectUnitPrefix + experiment.String()
}
