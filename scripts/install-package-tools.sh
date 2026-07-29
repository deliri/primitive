#!/usr/bin/env sh

set -eu

witness_revision=v0.0.0-20260723032132-133471a26114

go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
go install honnef.co/go/tools/cmd/staticcheck@v0.7.0
go install golang.org/x/vuln/cmd/govulncheck@v1.3.0
go install github.com/kisielk/errcheck@v1.20.0
go install go.uber.org/nilaway/cmd/nilaway@v0.0.0-20260612163715-2d8907f431ca
go install github.com/jgautheron/goconst/cmd/goconst@v1.10.1
go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@v0.45.0
go install github.com/securego/gosec/v2/cmd/gosec@v2.26.1
go install github.com/offGridSoft/witness/cmd/witness-lint@"$witness_revision"
go install github.com/offGridSoft/witness/cmd/deadcode@"$witness_revision"
