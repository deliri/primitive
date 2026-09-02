// Package standard defines the compiler-visible bar shared by Go repositories.
//
// It owns bounded project, package, source-file, declaration, import,
// dependency, effect, and evidence facts. Evidence records describe the common
// bar; they do not execute or coordinate a runner. This package is not a
// service, workflow, runner, product, or web application. Consumers own prose,
// policy, presentation, and completion.
package standard
