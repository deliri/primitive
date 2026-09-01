package release

import "github.com/deliri/primitive/v2026/core"

const doctrinePackageCapability core.PackageCapability = core.PackageCapabilityProcessExecution

func validatePackageCapability() error { return doctrinePackageCapability.Validate() }

var _ core.PackageCapability = doctrinePackageCapability
