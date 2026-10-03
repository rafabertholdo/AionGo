# Applying the Go skills to AionGo

Prepared 2026-10-01 from the current working tree. This is an implementation
plan. Implemented changes and their actual validation are tracked in
[GO_REVIEW_LEDGER.md](GO_REVIEW_LEDGER.md); the findings below are the original
sample, not an exhaustive audit or a claim that all checks have passed.

Planning update, 2026-10-02: the porting roadmap now incorporates a critical
Go reliability pass before broader non-quest feature work, followed by a full
codebase review alongside porting. Quest implementation and quest correctness
continue in parallel. See [JAVA_GO_PORT_PLAN.md](JAVA_GO_PORT_PLAN.md).

## Scope and findings

The Go source is in `AionGo/go`; `the-one/Apps/AionServer` is a compatibility
symlink. There are 46 installed Go skills under `.agents/skills`, 446 Go files
(192 test files), and seven commands. The module declares Go 1.25 and has one
direct dependency, the MySQL driver. The working tree already contains substantial
quest and panel work; implementation must preserve it.

The review sampled server lifecycle, connection handling, packet encoding,
visibility, persistence, panel authentication and mutations, test fixtures, and
build tooling. Findings below identify implementation targets; they do not
constitute an exhaustive security audit or measured performance diagnosis.

| Evidence | Implication |
| --- | --- |
| `go/scripts/run-go.sh` pins Go 1.25.1, one CPU, and `asyncpreemptoff`; `go/Dockerfile` builds with floating `golang:1.25` | Establish reproducible validation and separately qualify build-toolchain changes. Preserve the documented host workaround until it has been tested. |
| No `.github/workflows` or project `.golangci.yml` is present | Turn existing test/vet instructions into repeatable automation. |
| No `Fuzz*` or `Benchmark*` functions exist in the module | Add targeted coverage for hostile packets and a baseline for future optimization. |
| `game.NewServer` starts world tasks and login/chat goroutines; `game/links.go` reconnects forever | Listener closure alone cannot stop all server-owned work. |
| `game/conn.go` holds `worldMu` during dispatch and discards socket write errors; visibility code sends while `visMu` is held | Test slow-peer and failed-write behavior, then measure lock contention before redesigning output. |
| Most store and older panel calls use `Query`, `Exec`, or `Begin`; newer `cmd/panel/character_edit.go` uses request contexts, transactions, and offline-player locking | Extend the existing local pattern incrementally. |
| `game/inventory.go:saveItem` logs persistence failures but returns no error; callers can report updated balances | Define failure behavior before changing transactional paths. |
| `options.Load` silently falls back on query errors, skips scan failures, and omits `rows.Err()` | Distinguish the intentionally absent options table from database failure. |
| Panel uses `http.ListenAndServe`, trusts `X-Forwarded-Proto` for cookie security, and only the character editor has an explicit action token | Centralize HTTP limits and mutation protection; define which proxies are trusted. Existing strict SameSite cookies and role middleware are useful defenses. |
| `wire.Reader` returns zero-filled fields after truncation; `wire.Frame` narrows length to `uint16`; movement accepts floats | Test error handling before side effects, frame boundaries, and non-finite coordinates. |
| `game/known.go:updateKnown` scans every spawned player even though `pcells` exists | Benchmark player visibility at increasing populations and assess using the existing grid. |

## Skill routing and project-specific rules

Use `golang-how-to` as the entry point, then load the skills for the actual task.
Add that routing instruction to `AGENTS.md` in the first implementation batch.
Use `.agents/skills` as the installed skill source; `.claude/skills` already links
there. The copies in `agent/skills` differ in front matter; verify their role
before changing installation layout.

Record these project-specific interpretations alongside the routing instruction:

- Target the module's Go 1.25 APIs. Some installed guidance discusses newer
  releases; verify APIs against the selected toolchain before using them.
- Retain the Aion 1.9 wire format, numeric opcodes, Java-compatible ordering and
  rounding, and required login cryptography. Security work must respect client
  compatibility and document protocol requirements.
- Keep the current manual wiring, small consumer interfaces, explicit SQL, and
  standard testing package. Add a library only for a demonstrated need.
- Preserve meaningful nil-versus-empty distinctions, particularly optional
  packet/settings data. Do not apply the style skill's blanket slice rule.
- Use targeted refactors. Avoid global naming changes, package moves, test-file
  reshuffling, or one-type-per-file rules imported from the Swift repository.
- Serialize Go commands and benchmark runs on this Apple container host. The
  runner currently accepts only `go` and `gofmt`; add explicit tooling support
  before proposing direct linter, profiler, or language-server invocations.
- Keep Go and Java game databases separate. Database tests use disposable
  fixtures, never the persistent player databases.

## Ordered implementation batches

Each numbered batch can be split into small reviews. Infrastructure, structural
refactoring, and behavior changes should have separate diffs.

| Order | Skills | Concrete work | Acceptance evidence |
| --- | --- | --- | --- |
| 1. Reproducible checks | `golang-how-to`, `golang-lint`, `golang-continuous-integration`, `golang-dependency-management`, `golang-documentation`, `golang-gopls` | Add skill routing, a pinned tools runner, format/test/vet/build targets, and CI rooted at `go/`. Start with correctness linters and an explicit base revision for incremental adoption. Inventory existing lint findings. Qualify a build toolchain pin separately from host workarounds. | All seven commands build; test JSON reports passed/failed/skipped counts; static data is available in the data-backed job; vulnerability and lint reports are reproducible. |
| 2. Packet safety | `golang-safety`, `golang-security`, `golang-testing`, `golang-error-handling` | Add `wire` boundary/round-trip tests and fuzz targets. Review handler parsing for `Reader.Err` before mutations. Reject oversized outgoing frames and non-finite movement coordinates with defined connection behavior. Handle socket write failures and test encrypted packet ordering. | Malformed inputs produce no panic or state mutation; boundary sizes and truncated UTF-16 are covered; golden packet bytes remain identical. |
| 3. Panel HTTP protection | `golang-security`, `golang-context`, `golang-testing`, `golang-error-handling` | Use an explicit HTTP server with timeouts/body limits. Apply mutation protection consistently, rate-limit authentication, define trusted proxy behavior, and validate the environment-selected schema identifier. Reuse existing role middleware and character-edit protection. Separate user messages from internal SQL errors. | `httptest` covers role restrictions, invalid/missing tokens, request limits, cookie policy, cancellation, and proxy headers. Existing panel workflows still work. |
| 4. Graceful lifecycle | `golang-concurrency`, `golang-context`, `golang-testing`, `golang-cli`, `golang-structs-interfaces` | Give each server an owned cancellation path and waitable shutdown. Make link dial/read/retry operations cancellable. Track world tasks, close active peers, flush player state with a bounded cleanup context, then close the DB. Keep process wiring in the server commands. | Repeated start/stop tests terminate every owned worker; cancellation interrupts reconnect/backoff and blocked I/O; focused race tests pass. Use Go 1.25 `synctest` for timer-only units and `net.Pipe` for I/O. |
| 5. Persistence reliability | `golang-database`, `golang-context`, `golang-error-handling`, `golang-testing`, `golang-design-patterns` | Propagate contexts through panel/login/game stores in bounded vertical slices. Configure pool and driver timeouts. Make options loading report real failures. Return item persistence errors to callers and define atomic trade/mail/quest/balance operations before changing them. Preserve and extend existing transaction helpers. | Cancellation/deadline tests, injected write/commit failures, and rollback tests pass; disposable MariaDB tests prove no partial rewards, balances, or ownership changes. No success response is sent for an uncommitted mutation. |
| 6. Useful diagnostics | `golang-observability`, `golang-error-handling`, `golang-troubleshooting`, `golang-security` | Extend existing `slog` events with stable error/disconnect/reconnect reasons. Add counters and timings for sessions, packet failures, save failures, world-task duration, and DB pool pressure. Add opt-in protected profiling. Preserve the `Total Boot Time` readiness line used by ReRun/scripts. | Tooling still detects readiness; logs redact sensitive form values; metric labels are bounded; profiling is inaccessible from public listeners by default. |
| 7. Measured improvements and extraction | `golang-benchmark`, `golang-performance`, `golang-data-structures`, `golang-refactoring`, `golang-project-layout`, `golang-naming`, `golang-code-style`, `golang-dependency-injection`, `golang-modernize` | Benchmark framing/encryption, packet builders, player visibility and startup memory. Measure mutex/block profiles before changing output queues. Evaluate `pcells` for visibility first. Extract panel construction/route wiring into testable functions; consider an `internal/panel` move only if callers and test coverage justify it. Apply Go 1.25 idioms to touched code. | Same-toolchain before/after measurements with repeated samples and benchstat; unchanged visibility/packet behavior; bounded output under slow clients; no package move without a mapped call graph and reviewable inventory. |

Start batches 2 and 3 with characterization tests. Batch 4 supplies the lifecycle
context used by the game-store changes in batch 5. Simple panel request-context
improvements can be completed earlier because `r.Context()` already exists.
Measure suspicious hot paths as needed during reliability work; optimization
changes belong in their own batch.

## Critical pass and full review coverage

Batches 1–5 supply the initial critical pass. Keep their stated dependencies,
and prioritize demonstrated packet, authorization, concurrency and persistence
defects over cosmetic changes or completing every infrastructure enhancement.
This pass closes confirmed critical findings with regression evidence; it does
not certify all Go code as reviewed.

Then maintain a review ledger covering every Go package and its source/test
files, plus Go-related build tooling. Record the reviewed scope, applicable
skills, findings and severity, outstanding actions, and verification evidence.
Revisit reviewed code when porting or shared-helper changes affect its behavior.
Source inspection, lint, race checks, failure injection, data-backed tests and
1.9 captures provide different evidence; a passing unit suite does not replace
the remaining checks.

Quest work proceeds in parallel with this review and the non-quest port.
Coordinate shared quest/store interfaces and reward transactions before changing
them. Serialise container-backed Go commands even when development proceeds in
parallel. Preserve existing worktree edits and avoid unrelated style churn.

## Remaining skills: conditional use

| Skills | Apply when |
| --- | --- |
| `golang-spf13-cobra`, `golang-spf13-viper` | A command needs a real subcommand hierarchy or layered configuration. Current flag/env handling and database-option precedence are sufficient for the server entry points. |
| `golang-google-wire`, `golang-uber-dig`, `golang-uber-fx`, `golang-samber-do` | Dependency/lifecycle wiring becomes demonstrably harder than the current constructors. Choose one approach if needed. |
| `golang-grpc`, `golang-graphql`, `golang-swagger` | A new management API requires these contracts. They do not apply to the fixed Aion client protocol or the current HTML panel. |
| `golang-samber-lo`, `golang-samber-mo`, `golang-samber-ro`, `golang-samber-hot` | A measured collection, optional-value, stream, or cache requirement warrants the dependency. Prefer current standard-library mechanisms first. |
| `golang-samber-oops`, `golang-samber-slog` | Standard wrapped errors or slog handlers are insufficient for a chosen backend or logging requirement. |
| `golang-stretchr-testify` | Testify is deliberately adopted for a concrete testing need. Existing tests currently use the standard package. |
| `golang-pkg-go-dev`, `golang-popular-libraries`, `golang-stay-updated` | Evaluating a package, checking published documentation, or planning a deliberate dependency/toolchain update. Verify current primary documentation before adopting suggestions. |

## Validation for implementation

Run focused tests during each batch, then execute the full suite, vet, formatting
check and affected builds sequentially before handoff:

```sh
go/scripts/run-go.sh go test -json -count=1 ./...
go/scripts/run-go.sh go vet ./...
go/scripts/run-go.sh gofmt -l .
go/scripts/run-go.sh go build ./cmd/...
```

Add focused `-race` jobs for lifecycle/shared-state work. The current scratch
build uses `CGO_ENABLED=0`; race tests need a separate runner with CGO and a C
compiler. Do not use a whole-world real-database race run as the default on this
4 GB host. Establish isolated small fixtures before broadening race coverage.

The DB integration job must explicitly supply its fixture connection to the
container; the current wrapper does not forward `AION_TEST_DB`. Separate pure
unit tests, static-data tests, and disposable-DB tests in automation. Missing
required fixtures should fail the corresponding CI job instead of silently
skipping the advertised coverage.

Preserve the packet goldens and quest replay tests. A Java source comparison
alone does not prove 1.9 client compatibility; use old-server captures and a real
client smoke check for externally visible protocol changes. Add tests for new
behavior, report exact pass/fail/skip counts, and run the applicable Python
tooling tests if their scripts change.

## Review status

This plan was based on static inspection. No Go tests, lint runs, vulnerability
scan, benchmark, or gopls call graph was executed; `go` and `gopls` were not found
on the host PATH. Those baselines are batch 1 deliverables. No source code,
canonical roadmap, running container, or existing Git index entry was changed.

API references: [Go 1.25 release notes](https://go.dev/doc/go1.25) for
`testing/synctest`, and [database/sql](https://pkg.go.dev/database/sql) for
cancellation, transactions, and pool controls.
