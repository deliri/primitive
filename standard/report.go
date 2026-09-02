package standard

import (
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// WriteProjectMarkdown validates and streams one deterministic project
// standard.md projection. The caller owns the destination and its lifecycle.
func WriteProjectMarkdown(destination io.Writer, project Project) error {
	if destination == nil {
		return contractError(errors.New("standard project report destination is nil"))
	}
	if err := project.Validate(); err != nil {
		return err
	}
	writer := exactReportWriter{destination: destination}
	if err := writeProjectHeader(writer, project); err != nil {
		return err
	}
	if err := writeReasons(writer, project.Knowledge.AuthorReasons); err != nil {
		return err
	}
	if err := writeBoundaries(writer, reportOwnsLabel, project.Knowledge.AuthorOwns); err != nil {
		return err
	}
	if err := writeBoundaries(writer, "Non-goals", project.Knowledge.AuthorNonGoals); err != nil {
		return err
	}
	if err := writeUsage(writer, project.Usage); err != nil {
		return err
	}
	if err := writeFeatures(writer, project.Knowledge.AuthorFeatures); err != nil {
		return err
	}
	return writePackageIndex(writer, project)
}

// WritePackageMarkdown validates and streams one deterministic package
// standard.md projection, including exact source and evidence references.
func WritePackageMarkdown(destination io.Writer, snapshot PackageSnapshot) error {
	if destination == nil {
		return contractError(errors.New("standard package report destination is nil"))
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	writer := exactReportWriter{destination: destination}
	writers := []func(exactReportWriter) error{
		func(w exactReportWriter) error { return writePackageHeader(w, snapshot) },
		func(w exactReportWriter) error { return writeReasons(w, snapshot.Package.Knowledge.AuthorReasons) },
		func(w exactReportWriter) error {
			return writeBoundaries(w, reportOwnsLabel, snapshot.Package.Knowledge.AuthorOwns)
		},
		func(w exactReportWriter) error {
			return writeBoundaries(w, "Does not own", snapshot.Package.Knowledge.AuthorDoesNotOwn)
		},
		func(w exactReportWriter) error { return writeUsage(w, snapshot.Package.Knowledge.AuthorUsage) },
		func(w exactReportWriter) error { return writeFeatures(w, snapshot.Package.Knowledge.AuthorFeatures) },
		func(w exactReportWriter) error { return writeAssurance(w, snapshot.Package.Knowledge.AuthorAssurance) },
		func(w exactReportWriter) error {
			return writeComplexity(w, snapshot.Package.Knowledge.AuthorComplexity)
		},
		func(w exactReportWriter) error { return writeComponents(w, snapshot.Code.Components) },
		func(w exactReportWriter) error { return writeEvidence(w, snapshot) },
		func(w exactReportWriter) error { return writeSourceUsage(w, snapshot.Code.SourceUsage) },
	}
	for _, write := range writers {
		if err := write(writer); err != nil {
			return err
		}
	}
	return nil
}

// WritePackageEvolutionMarkdown streams the facts derived from compatible
// before and after Standard snapshots. It reports absence and negative movement
// explicitly instead of turning either into a success claim.
func WritePackageEvolutionMarkdown(destination io.Writer, evolution PackageEvolution) error {
	if destination == nil {
		return contractError(errors.New("standard package evolution report destination is nil"))
	}
	if err := evolution.Validate(); err != nil {
		return err
	}
	writer := exactReportWriter{destination: destination}
	writers := []func(exactReportWriter, PackageEvolution) error{
		writeEvolutionHeader,
		writeEvolutionInventory,
		writeEvolutionEvidence,
		writeEvolutionSourceUsage,
		writeEvolutionReviewCandidates,
	}
	for _, write := range writers {
		if err := write(writer, evolution); err != nil {
			return err
		}
	}
	return nil
}

func writeEvolutionHeader(writer exactReportWriter, evolution PackageEvolution) error {
	if err := heading(writer, 1, "Package evolution: "+evolution.Path.String()); err != nil {
		return err
	}
	return writeFacts(writer, [][2]string{
		{"Before revision", evolution.BeforeRevision.String()},
		{"After revision", evolution.AfterRevision.String()},
	})
}

func writeEvolutionInventory(writer exactReportWriter, evolution PackageEvolution) error {
	if err := heading(writer, 2, "Code inventory movement"); err != nil {
		return err
	}
	change := evolution.Inventory
	return writeFacts(writer, [][2]string{
		{"Go packages", signed(change.GoPackages)}, {"JavaScript units", signed(change.JavaScriptUnits)},
		{"Files", signed(change.Files)}, {"Test files", signed(change.TestFiles)},
		{"Documents", signed(change.Documents)}, {"Test declarations", signed(change.TestDeclarations)},
		{reportBenchmarksLabel, signed(change.Benchmarks)}, {"Fuzz targets", signed(change.FuzzTargets)},
	})
}

func writeEvolutionEvidence(writer exactReportWriter, evolution PackageEvolution) error {
	if err := heading(writer, 2, "Evidence movement"); err != nil {
		return err
	}
	change := evolution.Evidence
	return writeFacts(writer, [][2]string{
		{reportSurfacesLabel, signed(change.Surfaces)}, {reportRequestsLabel, signed(change.Requests)},
		{"Admissions", signed(change.Admissions)}, {"Refusals", signed(change.Refusals)},
		{reportObservationsLabel, signed(change.Observations)}, {"Current", signed(change.Current)},
		{"Stale", signed(change.Stale)}, {reportPassedLabel, signed(change.Passed)},
		{reportFailedLabel, signed(change.Failed)}, {reportNonAcceptingLabel, signed(change.NonAccepting)},
		{reportBenchmarksLabel, signed(change.Benchmarks)}, {"Artifacts", signed(change.Artifacts)},
		{"Complexity captures", signed(change.ComplexityCaptures)},
	})
}

func writeEvolutionSourceUsage(writer exactReportWriter, evolution PackageEvolution) error {
	if err := heading(writer, 2, "Source visibility movement"); err != nil {
		return err
	}
	change := evolution.SourceUsage
	return writeFacts(writer, [][2]string{
		{"Before analysis", availability(change.BeforeAvailable)}, {"After analysis", availability(change.AfterAvailable)},
		{reportDeclarationsLabel, signed(change.Declarations)}, {reportProductionReferencedLabel, signed(change.ProductionReferenced)},
		{reportRuntimeEntryPointsLabel, signed(change.RuntimeEntryPoints)}, {reportUnresolvedLabel, signed(change.UnresolvedDeclarations)},
		{reportTestOnlyLabel, signed(change.TestReferencedOnly)}, {reportNoReferenceObservedLabel, signed(change.NoReferenceObserved)},
		{"Observed consumer packages", signed(change.ObservedConsumerPackages)},
	})
}

func writeEvolutionReviewCandidates(writer exactReportWriter, evolution PackageEvolution) error {
	if err := heading(writer, 2, "Review candidate movement"); err != nil {
		return err
	}
	if len(evolution.NewReviewCandidates) == 0 && len(evolution.FormerReviewCandidates) == 0 {
		return paragraph(writer, "No review-candidate movement observed.")
	}
	for _, usage := range evolution.NewReviewCandidates {
		if err := bulletPair(writer, "Newly observable", codeReferenceText(usage.Function)); err != nil {
			return err
		}
	}
	for _, usage := range evolution.FormerReviewCandidates {
		if err := bulletPair(writer, "No longer listed", codeReferenceText(usage.Function)); err != nil {
			return err
		}
	}
	return newline(writer)
}

func availability(value bool) string {
	if value {
		return "available"
	}
	return reportUnavailableText
}

func signed(value CountChange) string {
	if value > 0 {
		return "+" + strconv.FormatInt(int64(value), 10)
	}
	return strconv.FormatInt(int64(value), 10)
}

func writeProjectHeader(writer exactReportWriter, project Project) error {
	if err := heading(writer, 1, project.Knowledge.AuthorTitle.String()); err != nil {
		return err
	}
	facts := [][2]string{
		{reportPurposeLabel, project.Knowledge.AuthorPurpose.String()},
		{reportProblemLabel, project.Knowledge.AuthorProblem.String()},
		{reportAudienceLabel, project.Knowledge.AuthorAudience.String()},
		{"Promise", project.Knowledge.AuthorPromise.String()},
		{reportRevisionLabel, project.Revision.String()},
	}
	if project.Release != nil {
		facts = append(facts, [2]string{"Release", project.Release.String()})
	}
	return writeFacts(writer, facts)
}

func writePackageHeader(writer exactReportWriter, snapshot PackageSnapshot) error {
	if err := heading(writer, 1, snapshot.Package.Knowledge.AuthorTitle.String()); err != nil {
		return err
	}
	return writeFacts(writer, [][2]string{
		{"Package", snapshot.Package.Knowledge.Path.String()},
		{reportRevisionLabel, snapshot.Package.Revision.String()},
		{reportPurposeLabel, snapshot.Package.Knowledge.AuthorPurpose.String()},
		{reportProblemLabel, snapshot.Package.Knowledge.AuthorProblem.String()},
		{reportAudienceLabel, snapshot.Package.Knowledge.AuthorAudience.String()},
		{"Value", snapshot.Package.Knowledge.AuthorValue.String()},
		{"Steward", snapshot.Package.Knowledge.AuthorSteward.String()},
		{"Substrate", snapshot.Package.Knowledge.AuthorSubstrate.String()},
		{"Runtime", snapshot.Package.Knowledge.AuthorRuntime.String()},
		{"Removal consequence", snapshot.Package.Knowledge.AuthorRemoval.String()},
	})
}

func writeReasons(writer exactReportWriter, values []Reason) error {
	if err := heading(writer, 2, "Why it exists"); err != nil {
		return err
	}
	for _, value := range values {
		if err := bulletPair(writer, value.Title.String(), value.Detail.String()); err != nil {
			return err
		}
	}
	return newline(writer)
}

func writeBoundaries(writer exactReportWriter, title string, values []Boundary) error {
	if err := heading(writer, 2, title); err != nil {
		return err
	}
	for _, value := range values {
		if err := bulletPair(writer, value.Title.String(), value.Detail.String()); err != nil {
			return err
		}
	}
	return newline(writer)
}

func writeUsage(writer exactReportWriter, values []Usage) error {
	if err := heading(writer, 2, "Usage"); err != nil {
		return err
	}
	for _, usage := range values {
		if err := heading(writer, 3, usage.Title.String()); err != nil {
			return err
		}
		if err := writeFacts(writer, [][2]string{{reportAudienceLabel, usage.Audience.String()}, {"Goal", usage.Goal.String()}}); err != nil {
			return err
		}
		if err := writeUsageSteps(writer, usage.Steps); err != nil {
			return err
		}
		if err := labeled(writer, "Outcome", usage.Outcome.String()); err != nil {
			return err
		}
	}
	return nil
}

func writeUsageSteps(writer exactReportWriter, steps []UsageStep) error {
	for index, step := range steps {
		value := strconv.Itoa(index+1) + ". " + markdown(step.Title.String()) + ": " + markdown(step.Detail.String())
		if step.Reference != nil {
			value += " (" + markdown(codeReferenceText(*step.Reference)) + ")"
		}
		if err := output(writer, value+"\n"); err != nil {
			return err
		}
	}
	return newline(writer)
}

func writeFeatures(writer exactReportWriter, values []Feature) error {
	if err := heading(writer, 2, "Features"); err != nil {
		return err
	}
	for _, feature := range values {
		if err := heading(writer, 3, feature.Title.String()); err != nil {
			return err
		}
		if err := writeFacts(writer, [][2]string{
			{"Contract", feature.Technical.String()},
			{"Benefit", feature.Benefit.String()},
			{"Proof", feature.ProofRequirement.String()},
			{"Delivery", enumText(uint8(feature.Delivery), deliveryLabels())},
		}); err != nil {
			return err
		}
	}
	return nil
}

func writePackageIndex(writer exactReportWriter, project Project) error {
	if err := heading(writer, 2, "Packages"); err != nil {
		return err
	}
	if err := output(writer, "| Package | Purpose | Observations | Source revision |\n| --- | --- | ---: | --- |\n"); err != nil {
		return err
	}
	for _, item := range project.Packages {
		source := reportNotAvailableText
		if item.SourceUsage != nil {
			source = item.SourceUsage.Revision.String()
		}
		row := "| " + markdownTable(item.Path.String()) + " | " + markdownTable(item.Purpose.String()) + " | " +
			strconv.FormatUint(uint64(item.Evidence.ObservedCount), 10) + "/" +
			strconv.FormatUint(uint64(item.Evidence.SurfaceCount), 10) + " | " + markdownTable(source) + " |\n"
		if err := output(writer, row); err != nil {
			return err
		}
	}
	return nil
}

func writeAssurance(writer exactReportWriter, assurance Assurance) error {
	if err := heading(writer, 2, "Assurance chain"); err != nil {
		return err
	}
	controls := [...]AssuranceControl{assurance.Policy, assurance.Validation, assurance.Effects, assurance.Proof}
	for _, control := range controls {
		if err := heading(writer, 3, enumText(uint8(control.Stage), assuranceStageLabels())); err != nil {
			return err
		}
		if err := labeled(writer, "Authority", enumText(uint8(control.Authority), assuranceAuthorityLabels())); err != nil {
			return err
		}
		if err := paragraph(writer, control.Statement.String()); err != nil {
			return err
		}
		for _, reference := range control.References {
			if err := bullet(writer, codeReferenceText(reference)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeComplexity(writer exactReportWriter, claims []ComplexityClaim) error {
	if err := heading(writer, 2, "Complexity contracts"); err != nil {
		return err
	}
	if len(claims) == 0 {
		return paragraph(writer, "No authored complexity claim.")
	}
	for _, claim := range claims {
		if err := heading(writer, 3, claim.ID.String()); err != nil {
			return err
		}
		if err := writeFacts(writer, [][2]string{
			{"Operation", codeReferenceText(claim.Operation)},
			{"Input", claim.Input.Name.String() + " in " + claim.Input.Unit.String() + ": " + claim.Input.Meaning.String()},
			{"Time", claim.Time.Notation()},
			{"Auxiliary space", claim.AuxiliarySpace.Notation()},
			{"Evidence surface", claim.SurfaceID.String()},
		}); err != nil {
			return err
		}
	}
	return nil
}

func writeComponents(writer exactReportWriter, values []Component) error {
	if err := heading(writer, 2, "Components"); err != nil {
		return err
	}
	if len(values) == 0 {
		return paragraph(writer, "No separately authored component record.")
	}
	for _, component := range values {
		if err := bulletPair(writer, component.Path.String(), component.AuthorPurpose.String()); err != nil {
			return err
		}
	}
	return newline(writer)
}

func writeEvidence(writer exactReportWriter, snapshot PackageSnapshot) error {
	summary, err := snapshot.EvidenceSummary()
	if err != nil {
		return err
	}
	if err := heading(writer, 2, "Evidence"); err != nil {
		return err
	}
	return writeFacts(writer, [][2]string{
		{reportSurfacesLabel, decimal(summary.SurfaceCount)},
		{reportRequestsLabel, decimal(summary.RequestedCount)},
		{"Admitted", decimal(summary.AdmittedCount)},
		{"Refused", decimal(summary.RefusedCount)},
		{reportObservationsLabel, decimal(summary.ObservedCount)},
		{"Selected revision", decimal(summary.CurrentCount)},
		{"Historical revision", decimal(summary.StaleCount)},
		{"Selections", decimal(summary.SelectionCount)},
		{reportPassedLabel, decimal(summary.PassedCount)},
		{reportFailedLabel, decimal(summary.FailedCount)},
		{reportNonAcceptingLabel, decimal(summary.NonAcceptingCount)},
	})
}

func writeSourceUsage(writer exactReportWriter, usage *PackageSourceUsage) error {
	if err := heading(writer, 2, "Source usage"); err != nil {
		return err
	}
	if usage == nil {
		return paragraph(writer, "No source-analysis observation.")
	}
	return writeFacts(writer, [][2]string{
		{reportRevisionLabel, usage.Revision.String()},
		{reportDeclarationsLabel, strconv.FormatUint(uint64(usage.DeclarationCount), 10)},
		{reportProductionReferencedLabel, strconv.FormatUint(uint64(usage.ProductionReferenced), 10)},
		{reportRuntimeEntryPointsLabel, strconv.FormatUint(uint64(usage.RuntimeEntryPoints), 10)},
		{reportUnresolvedLabel, strconv.FormatUint(uint64(usage.UnresolvedDeclarations), 10)},
		{reportTestOnlyLabel, strconv.FormatUint(uint64(usage.TestReferencedOnly), 10)},
		{reportNoReferenceObservedLabel, strconv.FormatUint(uint64(usage.NoReferenceObserved), 10)},
	})
}

func writeFacts(writer exactReportWriter, facts [][2]string) error {
	for _, fact := range facts {
		if err := labeled(writer, fact[0], fact[1]); err != nil {
			return err
		}
	}
	return nil
}

func codeReferenceText(reference CodeReference) string {
	if reference.Symbol == nil {
		return reference.Path.String()
	}
	return reference.Path.String() + ":" + reference.Symbol.String()
}

func heading(writer exactReportWriter, level int, value string) error {
	return output(writer, strings.Repeat("#", level)+" "+markdown(value)+"\n\n")
}

func paragraph(writer exactReportWriter, value string) error {
	return output(writer, markdown(value)+"\n\n")
}

func labeled(writer exactReportWriter, label, value string) error {
	return output(writer, "**"+markdown(label)+":** "+markdown(value)+"\n\n")
}

func bullet(writer exactReportWriter, value string) error {
	return output(writer, "- "+markdown(value)+"\n")
}

func bulletPair(writer exactReportWriter, first, second string) error {
	return output(writer, "- **"+markdown(first)+".** "+markdown(second)+"\n")
}

func newline(writer exactReportWriter) error { return output(writer, "\n") }

func markdown(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "\\<", ">", "\\>", "#", "\\#", "|", "\\|")
	return replacer.Replace(value)
}

func markdownTable(value string) string { return strings.ReplaceAll(markdown(value), "|", "\\|") }

func enumText(value uint8, labels []string) string {
	if int(value) >= len(labels) {
		return core.UnknownEnumDiagnostic
	}
	return labels[value]
}

func decimal(value uint16) string { return strconv.FormatUint(uint64(value), 10) }

func output(writer exactReportWriter, value string) error {
	_, err := io.WriteString(writer, value)
	return err
}

type exactReportWriter struct{ destination io.Writer }

func (w exactReportWriter) Write(value []byte) (int, error) {
	written, err := w.destination.Write(value)
	if err != nil {
		return written, err
	}
	if written != len(value) {
		return written, io.ErrShortWrite
	}
	return written, nil
}
