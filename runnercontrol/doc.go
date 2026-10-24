// Package runnercontrol owns the domain-blind outbound runner/control socket.
// ReadGoEventStream also exposes the cmd/go provider's existing decoder as
// borrowed typed string fragments and complete event frames. It uses fixed
// working memory without retaining package inventories, diagnostic strings, or
// an event history. Callers own the meaning of those observations.
//
// NewJUnitSummaryCompiler observes closed JUnit testcase outcomes without an
// invented plan or a total report-size ceiling. NewJUnitObservationCompiler
// projects that same parser's observations against a genuinely known plan.
// Both compilers require Seal or Abort to close and join their owned parser;
// encoding/xml owns its current token and nesting memory. Neither reported
// suite attributes nor test-file counts establish an independently planned total.
package runnercontrol
