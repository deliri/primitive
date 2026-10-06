# Committed JSON object stream proof

Source revision: bb7a7e89785ed4c37e52f239ceba3c63acf0e811 (v2026.1.122).

Command: go test -json -race -count=1 ./jsonio -timeout=3m.

Go 1.27.1, darwin/arm64; local development machine. Exit 0; 1,450 JSON events; no failed or skipped tests. The entire jsonio package was selected, with result caching disabled. Fuzz seed corpora were exercised; this command did not run timed fuzz campaigns. Raw output is retained in committed-race.jsonl. The checkout was clean at command start. This proof names the source revision; the later evidence commit does not change that coordinate. Independent acceptance belongs to the user.
