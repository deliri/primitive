// Package runnercontrol owns the domain-blind outbound runner/control socket.
// ReadGoEventStream also exposes the cmd/go provider's existing decoder as
// borrowed typed string fragments and complete event frames. It uses fixed
// working memory without retaining package inventories, diagnostic strings, or
// an event history. Callers own the meaning of those observations.
package runnercontrol
