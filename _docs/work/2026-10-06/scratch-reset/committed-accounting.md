# Exact committed scratch reset proof

Source: a2ec501b980dbe9d16bed12b03498376feef90e2 (v2026.1.123).
Command: go test -json -race -count=1 ./filestore ./compass -run 'TestScratchReset|TestCurrent' -timeout=3m.
Go 1.27.1 darwin/arm64; exit 0, no failed or skipped selected tests. The checkout was clean at command start. The command is filtered and exercises scratch reset and matching Compass metadata tests only; it is not complete Filestore package coverage. No timed fuzz, retry or cached result is represented as complete. Raw output is retained in committed-race.jsonl.
