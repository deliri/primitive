package runnercontrol

type goEventAction uint8

const (
	goEventUnknown goEventAction = iota
	goEventStart
	goEventRun
	goEventPause
	goEventContinue
	goEventPass
	goEventBenchmark
	goEventFail
	goEventOutput
	goEventSkip
	goEventBuildOutput
	goEventBuildFail
)

func decodeGoEventAction(value string) (goEventAction, error) {
	switch value {
	case "start":
		return goEventStart, nil
	case "run":
		return goEventRun, nil
	case "pause":
		return goEventPause, nil
	case "cont":
		return goEventContinue, nil
	case "pass":
		return goEventPass, nil
	case "bench":
		return goEventBenchmark, nil
	case "fail":
		return goEventFail, nil
	case "output":
		return goEventOutput, nil
	case "skip":
		return goEventSkip, nil
	case "build-output":
		return goEventBuildOutput, nil
	case "build-fail":
		return goEventBuildFail, nil
	default:
		return goEventUnknown, goJSONFailure()
	}
}

type goOutputKind uint8

const (
	goOutputOrdinary goOutputKind = iota
	goOutputFrame
	goOutputError
	goOutputErrorContinue
)

func decodeGoOutputKind(value string) (goOutputKind, error) {
	switch value {
	case "":
		return goOutputOrdinary, nil
	case "frame":
		return goOutputFrame, nil
	case "error":
		return goOutputError, nil
	case "error-continue":
		return goOutputErrorContinue, nil
	default:
		return goOutputOrdinary, goJSONFailure()
	}
}
