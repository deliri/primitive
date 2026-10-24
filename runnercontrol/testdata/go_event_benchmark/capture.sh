#!/bin/sh
# Regenerates the native go test -json streams beside this script from module/.
# Recorded with the toolchain in toolchain.txt; rerun with it before replacing.
# Package a/ also holds a sub-benchmark whose 1100-byte name makes test2json
# split its result row across output events (outBuffer is 1024 bytes).
set -eu
here=$(cd "$(dirname "$0")" && pwd)
cd "$here/module"
go version > "$here/toolchain.txt"
go test -json -run '^$' -bench 'BenchmarkB$|BenchmarkSub' -benchtime 1x -benchmem -cpu 1,2 . > "$here/benchmem.jsonl"
go test -json -run '^$' -bench 'BenchmarkB$' -benchtime 1x . > "$here/nobenchmem.jsonl"
go test -json -run '^TestPass$' . > "$here/tests.jsonl"
go test -json -run '^$' -bench 'Long' -benchtime 1x -benchmem ./a > "$here/longname.jsonl"
go test -json -p 2 -run '^$' -bench . -benchtime 1x -benchmem ./a ./b > "$here/packages.jsonl"
go test -json -run '^$' -bench 'BenchmarkCustom$' -benchtime 1x . > "$here/custom.jsonl"
go test -json -run '^$' -bench 'BenchmarkCustom$' -benchtime 1x -benchmem . > "$here/custommem.jsonl"
