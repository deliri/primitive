package core

// Systemd path-property arguments identify the native transient-unit wire
// vocabulary. Runner Control compiles the caller's typed path capabilities.
const (
	SystemdReadOnlyPathsArgument     = "--property=ReadOnlyPaths="
	SystemdReadWritePathsArgument    = "--property=ReadWritePaths="
	SystemdInaccessiblePathsArgument = "--property=InaccessiblePaths="
)
