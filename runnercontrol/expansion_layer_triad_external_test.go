package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestExpansionProducerSchemaVerifierLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive selection expansion signs exact build context and admitted child", func(t *testing.T) {
		t.Parallel()
		manifest := expansionManifestFixture(t, true)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueExpansion(manifest, key)
		verifyErr := runnercontrol.VerifyExpansion(document, trusted)
		record, recordErr := runnercontrol.NewExpansionRecord(document)
		if issueErr != nil || verifyErr != nil || recordErr != nil || record.Bytes.Uint64() == 0 || record.Digest != core.SHA256Of(record.Canonical) {
			t.Fatalf("expansion proof = (issue %v, verify %v, record %v, bytes %d), want nil errors and exact nonzero record", issueErr, verifyErr, recordErr, record.Bytes.Uint64())
		}
		if len(manifest.Children) != 1 || manifest.Children[0].Probe.Parent == nil || manifest.Children[0].Probe.Parent.ExpansionDigest != manifest.Identity {
			t.Fatalf("ExpansionManifest child binding = %+v, want one child bound to manifest identity %v", manifest.Children, manifest.Identity)
		}
	})

	t.Run("negative one-child experiment mutation keeps structure valid but fails signature verification", func(t *testing.T) {
		t.Parallel()
		manifest := expansionManifestFixture(t, true)
		key, trusted := completionSignerFixture(t)
		document, issueErr := runnercontrol.IssueExpansion(manifest, key)
		if issueErr != nil {
			t.Fatalf("IssueExpansion() setup error = %v, want nil", issueErr)
		}
		mutatedUUID, uuidErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000002")
		mutatedExperiment, experimentErr := runprotocol.NewExperimentID(mutatedUUID)
		if err := errors.Join(uuidErr, experimentErr); err != nil {
			t.Fatalf("expansion mutation fixture error = %v, want nil", err)
		}
		document.Manifest.Children[0].Experiment = &mutatedExperiment
		if gotErr := document.Validate(); gotErr != nil {
			t.Fatalf("ExpansionDocument.Validate(mutated child experiment) error = %v, want nil so signature verification owns rejection", gotErr)
		}
		gotErr := runnercontrol.VerifyExpansion(document, trusted)
		if !errors.Is(gotErr, core.ErrAttestVerification) {
			t.Fatalf("VerifyExpansion(mutated child experiment) error = %v, want errors.Is(..., %v)", gotErr, core.ErrAttestVerification)
		}
	})

	t.Run("negative child-set mutation cannot retain the same manifest digest", func(t *testing.T) {
		t.Parallel()
		withChild := expansionManifestFixture(t, true)
		withoutChild := expansionManifestFixture(t, false)
		if withChild.Identity != withoutChild.Identity {
			t.Fatalf("expansion parent identities = (%v, %v), want equal non-circular parent identity", withChild.Identity, withoutChild.Identity)
		}
		withDigest, withErr := withChild.Digest()
		withoutDigest, withoutErr := withoutChild.Digest()
		if withErr != nil || withoutErr != nil || withDigest == withoutDigest {
			t.Fatalf("ExpansionManifest.Digest(child-set mutation) = (%v, %v, errors %v/%v), want distinct full-manifest digests", withDigest, withoutDigest, withErr, withoutErr)
		}
		signer, _ := completionSignerFixture(t)
		approval := expansionApprovalSeed(t, withChild, signer)
		got := *approval.Experiments[0].Payload.ExpansionManifestDigest
		if got != withDigest || got == withChild.Identity {
			t.Fatalf("experiment manifest binding = %v, want full digest %v distinct from parent identity %v", got, withDigest, withChild.Identity)
		}
	})

	t.Run("negative parent identity cannot impersonate the full manifest digest", func(t *testing.T) {
		t.Parallel()
		manifest := expansionManifestFixture(t, true)
		signer, _ := completionSignerFixture(t)
		approval := expansionApprovalSeed(t, manifest, signer)
		capability := approval.Experiments[0].Payload
		capability.ExpansionManifestDigest = &manifest.Identity
		document, issueErr := runnercontrol.IssueExperimentCapability(capability, signer)
		approval.ManifestDigest = manifest.Identity
		approval.Experiments = []runnercontrol.ExperimentCapabilityDocument{document}
		if err := errors.Join(issueErr, approval.Validate()); err != nil {
			t.Fatalf("identity-only ExpansionApproval setup error = %v, want structurally valid signed document", err)
		}
		got, gotErr := runnercontrol.CompileSelectionObservation(manifest, approval, 1)
		if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != (runprotocol.SelectionObservation{}) {
			t.Fatalf("CompileSelectionObservation(identity-only approval) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
	})

	t.Run("positive selection observation carries parent identity and full manifest digest separately", func(t *testing.T) {
		t.Parallel()
		manifest := expansionManifestFixture(t, true)
		signer, _ := completionSignerFixture(t)
		approval := expansionApprovalSeed(t, manifest, signer)
		manifestDigest, digestErr := manifest.Digest()
		got, gotErr := runnercontrol.CompileSelectionObservation(manifest, approval, 1)
		if digestErr != nil || gotErr != nil || got.ExpansionIdentity != manifest.Identity || got.ManifestDigest != manifestDigest || got.ExpansionIdentity == got.ManifestDigest {
			t.Fatalf("CompileSelectionObservation() = (%+v, %v; digest error %v), want identity %v and distinct full digest %v", got, gotErr, digestErr, manifest.Identity, manifestDigest)
		}
	})

	t.Run("neutral selection with no discovered child retains zero accounting without inventing an experiment", func(t *testing.T) {
		t.Parallel()
		manifest := expansionManifestFixture(t, false)
		if gotErr := manifest.Validate(); gotErr != nil || len(manifest.Children) != 0 || manifest.Admitted != 0 || manifest.Refused != 0 || manifest.NotApplicable != 0 {
			t.Fatalf("ExpansionManifest.Validate(no children) = (%v, children %d, counts %d/%d/%d), want nil and all zero", gotErr, len(manifest.Children), manifest.Admitted, manifest.Refused, manifest.NotApplicable)
		}
	})
}

func FuzzExpansionDocumentSemanticClosure(f *testing.F) {
	manifest := expansionManifestFixture(f, true)
	key, _ := completionSignerFixture(f)
	seedValue, issueErr := runnercontrol.IssueExpansion(manifest, key)
	if issueErr != nil {
		f.Fatalf("IssueExpansion(seed) error = %v, want nil", issueErr)
	}
	seed := mustExpansionDocumentJSON(f, seedValue)
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`null`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seedValue
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !bytes.Equal(mustExpansionDocumentJSON(t, got), seed) {
				t.Fatalf("ExpansionDocument.UnmarshalJSON(rejected) = (receiver %q, error %v), want preserved %q and errors.Is(..., %v)", mustExpansionDocumentJSON(t, got), gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		encoded := mustExpansionDocumentJSON(t, got)
		var roundTrip runnercontrol.ExpansionDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !bytes.Equal(mustExpansionDocumentJSON(t, roundTrip), encoded) {
			t.Fatalf("ExpansionDocument canonical closure = (second %q, error %v), want %q and nil", mustExpansionDocumentJSON(t, roundTrip), err, encoded)
		}
	})
}

func expansionManifestFixture(t testing.TB, includeChild bool) runnercontrol.ExpansionManifest {
	t.Helper()
	completion := experimentCompletionPayloadFixture(t, true)
	request, requestErr := runprotocol.NewRequestIdentity(completionUUIDFixture(t))
	module, moduleErr := runprotocol.NewIdentifier("runner")
	packagePath, packageErr := runprotocol.ParseSourcePath("runnercontrol")
	filePath, fileErr := runprotocol.ParseSourcePath("runnercontrol/claim.go")
	symbol, symbolErr := runprotocol.NewName("TestClaim")
	toolchain, toolchainErr := runprotocol.NewIdentifier("go1-27-0")
	releaseTag, releaseErr := runnercontrol.NewGoBuildTag("go1.27")
	discovery, discoveryErr := runprotocol.NewIdentifier("go-ast-discovery")
	moduleRoot, rootErr := runprotocol.ParseSourcePath("module")
	if err := errors.Join(requestErr, moduleErr, packageErr, fileErr, symbolErr, toolchainErr, releaseErr, discoveryErr, rootErr); err != nil {
		t.Fatalf("expansion identity fixture error = %v, want nil", err)
	}
	parentTarget := runprotocol.ProbeTarget{Kind: runprotocol.ProbeTargetGoPackage, GoPackage: &runprotocol.GoPackageTarget{Module: module, Package: packagePath, ChildKinds: []runprotocol.ProbeKind{runprotocol.ProbeKindGoTest}}}
	parent := runprotocol.ProbeIdentity{Origin: completion.Probe.Origin, Subject: completion.Probe.Subject, Source: completion.Probe.Source, Role: runprotocol.ProbeRoleSelection, Kind: runprotocol.ProbeKindGoPackageSelection, Target: parentTarget, Profile: completion.Probe.Profile, Environment: completion.Probe.Environment}
	context := runnercontrol.GoBuildContext{Toolchain: toolchain, ReleaseTags: []runnercontrol.GoBuildTag{releaseTag}, GOOS: core.OperatingSystemLinux, GOARCH: core.CPUArchitectureAMD64, CGOEnabled: false, BuildTags: []runnercontrol.GoBuildTag{}, Instrumentation: runnercontrol.GoInstrumentationOrdinary, GOExperiment: []runnercontrol.GoBuildTag{}, ModuleMode: runnercontrol.GoModuleModeModule, ModuleRoot: moduleRoot, OtherInputs: core.SHA256Of([]byte("go-build-inputs"))}
	contextDigest, contextErr := context.Digest()
	contexts := runnercontrol.GoBuildContextSet{Entries: []runnercontrol.GoBuildContextEntry{{Kind: runprotocol.ProbeKindGoTest, Profile: completion.Probe.Profile, Context: context, Digest: contextDigest}}}
	manifest := runnercontrol.ExpansionManifest{SchemaVersion: runnercontrol.SchemaVersion, Request: request, Run: completion.Run, Fence: completion.Fence, Members: completion.Members, Parent: parent, Source: completion.Probe.Source, Discovery: discovery, DiscoveryVersion: 1, RequestedKinds: []runprotocol.ProbeKind{runprotocol.ProbeKindGoTest}, Contexts: contexts, Children: []runnercontrol.ExpansionChild{}}
	identity, identityErr := runnercontrol.CalculateExpansionIdentity(manifest)
	manifest.Identity = identity
	if includeChild {
		selectionParent := runprotocol.SelectionParent{Request: request, Kind: parent.Kind, Target: parent.Target, ExpansionDigest: identity}
		childTarget := runprotocol.ProbeTarget{Kind: runprotocol.ProbeTargetGoDeclaration, GoDeclaration: &runprotocol.GoDeclarationTarget{Module: module, Package: packagePath, File: filePath, Symbol: symbol}}
		childProbe := runprotocol.ProbeIdentity{Origin: parent.Origin, Subject: parent.Subject, Source: parent.Source, Role: runprotocol.ProbeRoleExperiment, Kind: runprotocol.ProbeKindGoTest, Target: childTarget, Profile: parent.Profile, Environment: parent.Environment, Parent: &selectionParent}
		experiment := completion.Observation.Experiment
		manifest.Children = []runnercontrol.ExpansionChild{{Sequence: 1, Probe: childProbe, BuildContextDigest: contextDigest, Disposition: runnercontrol.ExpansionAdmitted, Experiment: &experiment}}
		manifest.Admitted = 1
	}
	if err := errors.Join(contextErr, identityErr, manifest.Validate()); err != nil {
		t.Fatalf("ExpansionManifest.Validate() fixture error = %v, want nil", err)
	}
	return manifest
}

func mustExpansionDocumentJSON(t testing.TB, value runnercontrol.ExpansionDocument) []byte {
	t.Helper()
	got, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ExpansionDocument.MarshalJSON() fixture error = %v, want nil", err)
	}
	return got
}
