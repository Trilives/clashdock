# Ideas Preview TDD Evidence

## Source

- Plan: [Records/ideas.md](../../Records/ideas.md)
- RED checkpoint: `230233c`
- GREEN checkpoint: `8b3fb85`

## Guarantees

| Guarantee | Test target | Type | Result |
|---|---|---|---|
| Enter edits the focused field and only submits from the submit button | `internal/tui/form_test.go` | Unit | PASS |
| The active config is written before runtime sync and sync errors are returned | `internal/subscription/manager_test.go` | Integration | PASS |
| Runtime config is installed and validated before service restart | `internal/sysd/service_test.go` | Integration | PASS |
| Clash API policy-group selections are parsed and HTTP errors are rejected | `internal/clashapi/clashapi_test.go` | Integration | PASS |
| Node labels distinguish runtime state, configured preference, and fallback state | `internal/flows/nodeselect_test.go` | Unit | PASS |
| Pinned-node writes are persisted before runtime sync, and sync failures explicitly report that the preference was already saved | `internal/flows/nodeselect_test.go` | Integration | PASS |

## Evidence

- RED: `go test ./internal/tui ./internal/subscription ./internal/sysd ./internal/clashapi ./internal/flows` failed on the previous Enter behavior and missing sync/status capabilities.
- GREEN: the same command passed after implementation.
- Full suite: `go test ./...` passed.
- Coverage: `go test ./... -coverprofile=/tmp/clashdock-cover.out && go tool cover -func=/tmp/clashdock-cover.out | tail -1` passed.
- Race detector: `go test -race ./...` passed.
- Build and static checks: `go vet ./...`, `make build`, `gofmt -l internal/flows/nodeselect.go internal/flows/nodeselect_test.go`, `bash -n scripts/portable/install.sh scripts/portable/uninstall.sh scripts/portable/tool/update.sh scripts/portable/tool/nettest.sh scripts/fetch-deb-deps.sh scripts/migrate-runtime-dir.sh`, and `goreleaser check` passed.

## Coverage

- Repository statement coverage: 29.9% (existing project-wide gap; below the 80% target).
- Changed helpers: `CurrentSelections` 85.7%, `currentNodeLabels` 100%, `applyActiveWithSync` 69.2%, `syncStagedAndRestart` 71.4%, and `editField` 87.5%.
- System-level sudo/systemd execution is covered through injected command runners; no real host service was restarted during tests.
