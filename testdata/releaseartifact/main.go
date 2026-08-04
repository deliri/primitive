package main

import (
	"fmt"

	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/testdata/releasestamp"
)

func main() {
	identity, err := release.EmbeddedBuildIdentity()
	if err != nil {
		return
	}
	fmt.Print(identity.Offering(), releasestamp.Value)
}
