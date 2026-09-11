package runnercontrol

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// This wiring ratchet enumerates every public JSON decoder and its semantic
// fuzz target. Behavioral oracles live in those targets, not in source scanning.
func TestRunnerControlExternalDecodersHaveNamedFuzzTargets(t *testing.T) {
	t.Parallel()
	inventory := map[string]string{
		"AdmissionResponse":             "FuzzAdmissionResponseSemanticClosure",
		"AdmittedRun":                   "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ArtifactChunk":                 "FuzzArtifactChunkSemanticClosure",
		"ArtifactChunkReceipt":          "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ArtifactKind":                  "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"ArtifactManifest":              "FuzzArtifactManifestSemanticClosure",
		"ArtifactManifestReceipt":       "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"CancellationRequest":           "FuzzCancellationRequestJSONSemanticClosure",
		"CancellationResponse":          "FuzzCancellationResponseJSONSemanticClosure",
		"CapabilitySigningDomain":       "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"ClaimKind":                     "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"ClaimRequest":                  "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ClaimResponse":                 "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"CleanupDocument":               "FuzzCleanupDocumentSemanticClosure",
		"CleanupOutcomeKind":            "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"CleanupPayload":                "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"CleanupReceipt":                "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"CompletionSigningDomain":       "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"CoverageMode":                  "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"DirectiveKind":                 "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"EgressMode":                    "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"EvidenceBodyKind":              "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"EvidenceSigningDomain":         "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"ExpansionApproval":             "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ExpansionDisposition":          "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"ExpansionDocument":             "FuzzExpansionDocumentSemanticClosure",
		"ExpansionManifest":             "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ExperimentCapability":          "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ExperimentCapabilityDocument":  "FuzzCapabilityDocumentsSemanticClosure",
		"ExperimentCompletionDocument":  "FuzzExperimentCompletionJSONSemanticClosure",
		"ExperimentCompletionPayload":   "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ExperimentCompletionReceipt":   "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ExperimentDeliveryPage":        "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"GoBuildTag":                    "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"GoInstrumentation":             "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"GoModuleMode":                  "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"GoProfileKind":                 "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"HeartbeatRequest":              "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"HeartbeatResponse":             "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"HeartbeatState":                "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"MachineObservationReceipt":     "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"MachineObservationSubmission":  "FuzzMachineObservationSubmissionSemanticClosure",
		"MemberCapability":              "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"MemberCapabilityDocument":      "FuzzCapabilityDocumentsSemanticClosure",
		"NetworkProtocol":               "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"ObservationDeliveryCommit":     "FuzzObservationDeliveryCommitSemanticClosure",
		"ObservationDeliveryPageUpload": "FuzzObservationDeliveryPageUploadSemanticClosure",
		"ObservationDeliveryReceipt":    "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ObservationDeliveryStage":      "FuzzObservationDeliveryStageSemanticClosure",
		"ObservationEnvelope":           "FuzzObservationEnvelopeSemanticClosure",
		"ObservationEnvelopePayload":    "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"ObservationFormat":             "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"PeerCredentialKind":            "FuzzPeerCredentialKindExternalJSONSemanticClosure",
		"PeerRole":                      "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"RequestedRun":                  "FuzzRequestedRunSemanticClosure",
		"RunControlState":               "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"RunStateRequest":               "FuzzRunStateRequestJSONSemanticClosure",
		"RunStateResponse":              "FuzzRunStateResponseJSONSemanticClosure",
		"RunnerCompletionDocument":      "FuzzRunnerCompletionJSONSemanticClosure",
		"RunnerCompletionPayload":       "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"RunnerCompletionReceipt":       "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"SchedulingCapability":          "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"SchedulingCapabilityDocument":  "FuzzCapabilityDocumentsSemanticClosure",
		"SchedulingClaim":               "FuzzSchedulingClaimSemanticAuthentication",
		"SchedulingUnitKind":            "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"SourceAcquisition":             "FuzzSourceAcquisitionSemanticClosure",
		"SourceAcquisitionRequest":      "FuzzRunnerControlExternalStructureJSONSemanticClosure",
		"SourceArchiveDocument":         "FuzzSourceArchiveDocumentSemanticClosure",
		"SourceSigningDomain":           "FuzzRunnerControlExternalEnumJSONSemanticClosure",
		"SubjectIsolationEngine":        "FuzzRunnerControlExternalEnumJSONSemanticClosure",
	}
	decoders := map[string]bool{}
	targets := map[string]bool{}
	files, err := fs.Glob(runnerControlSource, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		source, err := runnerControlSource.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if strings.HasSuffix(name, "_test.go") {
				if function.Recv == nil && strings.HasPrefix(function.Name.Name, "Fuzz") {
					targets[function.Name.Name] = true
				}
			} else if function.Name.Name == "UnmarshalJSON" && function.Recv != nil {
				decoders[runnerControlReceiverName(function.Recv.List[0].Type)] = true
			}
		}
	}
	for name := range decoders {
		target, ok := inventory[name]
		if !ok || !targets[target] {
			t.Errorf("%s.UnmarshalJSON fuzz target = %q, want named live semantic target", name, target)
		}
	}
	for name := range inventory {
		if !decoders[name] {
			t.Errorf("decoder inventory retains %s, want only live public boundaries", name)
		}
	}
}
