#!/usr/bin/env sh

set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repository_root"

artifact_directory=${1:-".artifacts/gates/local"}
mkdir -p "$artifact_directory"
artifact_directory=$(CDPATH= cd -- "$artifact_directory" && pwd)
go_binary_directory=$(go env GOBIN)
if test -z "$go_binary_directory"; then
	go_binary_directory=$(go env GOPATH)/bin
fi
if test "$(go env GOOS)" = "windows"; then
	go_binary_directory=$(cygpath -u "$go_binary_directory")
fi
PATH="$go_binary_directory:$PATH"
export PATH

result_file="$artifact_directory/gate-results.tsv"
platform=$(go env GOOS)/$(go env GOARCH)
workflow_run=${GITHUB_RUN_ID:-NOT_APPLICABLE}
workflow_attempt=${GITHUB_RUN_ATTEMPT:-NOT_APPLICABLE}

printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
	"gate" "command" "platform" "duration_seconds" "exit_status" "log" "sha256" \
	>"$result_file"
printf '%s\n' "$workflow_run" >"$artifact_directory/workflow-run.log"
printf '%s\n' "$workflow_attempt" >"$artifact_directory/workflow-attempt.log"

file_sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		hash_line=$(sha256sum "$1")
	elif command -v shasum >/dev/null 2>&1; then
		hash_line=$(shasum -a 256 "$1")
	else
		printf '%s\n' "no SHA-256 tool found" >&2
		return 1
	fi
	hash_value=${hash_line%% *}
	case "$hash_value" in
		????????????????????????????????????????????????????????????????)
			case "$hash_value" in
				*[!0-9A-Fa-f]*)
					printf '%s\n' "invalid SHA-256 output" >&2
					return 1
					;;
				*)
					printf '%s' "$hash_value"
					;;
			esac
			;;
		*)
			printf '%s\n' "invalid SHA-256 output" >&2
			return 1
			;;
	esac
}

clock_now_seconds() {
	date +%s
}

append_result() {
	result_name=$1
	result_command=$2
	result_duration=$3
	result_status=$4
	result_log=$5
	result_hash=$(file_sha256 "$result_log")
	printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
		"$result_name" "$result_command" "$platform" "$result_duration" \
		"$result_status" "$(basename "$result_log")" "$result_hash" \
		>>"$result_file"
}

run_gate() {
	gate_name=$1
	shift
	gate_log="$artifact_directory/$gate_name.log"
	gate_command="$*"
	gate_started=$(clock_now_seconds)
	if "$@" >"$gate_log" 2>&1; then
		gate_status=0
	else
		gate_status=$?
	fi
	gate_finished=$(clock_now_seconds)
	gate_duration=$((gate_finished - gate_started))
	append_result \
		"$gate_name" "$gate_command" "$gate_duration" "$gate_status" "$gate_log"
	if test "$gate_status" -eq 0; then
		printf '%s\n' "PASS $gate_name"
		return
	fi
	printf '%s\n' "FAIL $gate_name"
	cat "$gate_log"
	exit "$gate_status"
}

run_empty_output_gate() {
	gate_name=$1
	shift
	gate_log="$artifact_directory/$gate_name.log"
	gate_command="$*"
	gate_started=$(clock_now_seconds)
	if "$@" >"$gate_log" 2>&1; then
		gate_status=0
	else
		gate_status=$?
	fi
	if test "$gate_status" -eq 0 && test -s "$gate_log"; then
		gate_status=1
	fi
	gate_finished=$(clock_now_seconds)
	gate_duration=$((gate_finished - gate_started))
	append_result \
		"$gate_name" "$gate_command" "$gate_duration" "$gate_status" "$gate_log"
	if test "$gate_status" -eq 0; then
		printf '%s\n' "PASS $gate_name"
		return
	fi
	printf '%s\n' "FAIL $gate_name"
	cat "$gate_log"
	exit "$gate_status"
}

run_gate go-version go version
run_gate go-environment go env GOOS GOARCH GOVERSION
run_gate git-status git status --short --branch
run_gate install-tools bash scripts/install-tools.sh
run_gate module-tidy go mod tidy -diff
run_gate workflow actionlint .github/workflows/ci.yml
run_empty_output_gate format gofmt -l .

package_list="$artifact_directory/packages.log"
package_stderr="$artifact_directory/packages.stderr.log"
package_started=$(clock_now_seconds)
if go list ./... >"$package_list" 2>"$package_stderr"; then
	package_status=0
else
	package_status=$?
fi
package_finished=$(clock_now_seconds)
package_duration=$((package_finished - package_started))
append_result \
	"package-list-stdout" "go list ./..." "$package_duration" "$package_status" \
	"$package_list"
append_result \
	"package-list-stderr" "go list ./..." "$package_duration" "$package_status" \
	"$package_stderr"
if test "$package_status" -ne 0; then
	printf '%s\n' "FAIL package-list"
	cat "$package_stderr"
	exit "$package_status"
fi
if ! test -s "$package_list"; then
	not_applicable_log="$artifact_directory/package-gates.log"
	printf '%s\n' \
		"NOT_APPLICABLE package gates: the package-free design phase contains no Go packages" \
		>"$not_applicable_log"
	append_result \
		"package-gates" "NOT_APPLICABLE" 0 0 "$not_applicable_log"
	printf '%s\n' "PASS package-gates (not applicable: no packages)"
	exit 0
fi

run_gate install-package-tools bash scripts/install-package-tools.sh
run_gate test go test -count=1 ./...
run_gate test-race go test -race -count=1 ./...
run_gate vet go vet ./...
run_gate staticcheck staticcheck ./...
run_gate errcheck errcheck ./...
run_gate nilaway nilaway ./...
run_gate witness-lint witness-lint ./...
run_gate complexity gocyclo -over 10 --ignore '_test.go' .
run_gate constants goconst -set-exit-status ./...
run_gate field-alignment fieldalignment ./...
run_gate security gosec -quiet ./...
run_gate vulnerabilities govulncheck ./...
run_gate dead-code deadcode -test ./...
run_gate benchmark-json-contracts \
	go test -run '^$' -bench '^Benchmark(DecodeStrictJSON|EncodeValidatedJSON|JSONMarshal|RejectDuplicateJSONFields)' \
	-benchmem -count=1 ./core
run_gate benchmark-attest-sign \
	go test -run '^$' -bench '^BenchmarkSignCanonicalBody(64KiB|Maximum)$' \
	-benchmem -count=1 ./attest
run_gate benchmark-currency \
	go test -run '^$' -bench '^Benchmark(ParseDecimal|FormatDecimal)$' \
	-benchmem -count=1 ./currency
run_gate benchmark-garble \
	go test -run '^$' -bench '^Benchmark(DeriveSeed|ParseSeed)$' \
	-benchmem -count=1 ./garble
run_gate benchmark-keygen \
	go test -run '^$' -bench '^BenchmarkGenerate(SigningKey|MaximumSecret)$' \
	-benchmem -count=1 ./keygen
run_gate fuzz-decode-strict-json-absolute-path \
	go test -run '^$' -fuzz '^FuzzDecodeStrictJSONAbsolutePathPublicBoundary$' \
	-fuzztime=100000x -parallel=1 ./core
run_gate fuzz-marshal-json-string \
	go test -run '^$' -fuzz '^FuzzMarshalJSONStringRoundTrip$' \
	-fuzztime=100000x -parallel=1 ./core
run_gate fuzz-attest-envelope-json \
	go test -run '^$' -fuzz '^FuzzEnvelopeJSONSemanticClosure$' \
	-fuzztime=100000x -parallel=1 ./attest
run_gate fuzz-attest-signed-fields \
	go test -run '^$' -fuzz '^FuzzVerifyRejectsEveryIndependentlyMutatedSignedField$' \
	-fuzztime=100000x -parallel=1 ./attest
run_gate fuzz-currency-decimal \
	go test -run '^$' -fuzz '^FuzzDecimalParserAgainstStandardGrammarAndBigRationalOracle$' \
	-fuzztime=100000x -parallel=1 ./currency
run_gate fuzz-currency-amount-json \
	go test -run '^$' -fuzz '^FuzzAmountJSONAgainstStandardTokenStreamOracle$' \
	-fuzztime=100000x -parallel=1 ./currency
run_gate fuzz-garble-seed-json \
	go test -run '^$' -fuzz '^FuzzSeedJSONAgainstDirectStandardLibraryOracle$' \
	-fuzztime=100000x -parallel=1 ./garble
