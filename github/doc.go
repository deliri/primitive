// Package github owns the blind typed socket between Go values and GitHub's
// REST API. It validates and transports repository mechanics, including
// bounded immutable-commit archive streams; callers retain all policy about
// releases, packages, files, and what an observation means.
package github
