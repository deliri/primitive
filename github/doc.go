// Package github owns the blind typed socket between Go values and GitHub's
// REST API. It validates and transports repository mechanics, including
// streaming immutable-commit files, trees, and archives; callers retain all
// policy about releases, packages, files, and what an observation means.
//
// File and archive downloads write directly to the supplied destination. An
// error may accompany an acknowledged prefix; callers must check the error
// before treating that output as complete. To preserve a published file,
// callers use filestore.OpenStageDestination, pass its File to the download,
// abandon on transfer failure, and finish then commit only after success.
// GitHub does not stage or publish files itself.
//
// Tree visitors provide synchronous backpressure and must return. Callers own
// cancellation of blocking visitor and destination work. Streaming transfers,
// including archive redirect-body drains, inherit the caller context without
// an additional Primitive timeout or total-byte quota.
//
// Tag pages, head observations, and installation-token requests currently
// decode complete JSON responses in memory. These are whole-value operations;
// their response memory grows with the received document, unlike the file and
// archive transfer windows. A caller that supplies an in-memory destination
// likewise owns the retained download bytes.
package github
