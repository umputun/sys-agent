# Provider URL parsing fixes and file provider glob/date templates

## Overview

Twelve defects in `app/status/external` and `app/config`, and a new file-provider capability
that depends on fixing them first.

The largest group shares one root cause: ad-hoc URL handling. `program.go`, `file_provider.go` and
`certificate_provider.go` each derive their target with `strings.TrimPrefix` and never strip the
query string, so the documented `?cron=` parameter corrupts the target in all three. `mongo` and
`docker` use `net/url` and are correct.

The rest are independent: a nil `os.FileInfo` cached by the file provider that panics and then
deadlocks the provider goroutine, `template.Must` on config-supplied input in the mongo provider
that panics the process, an unsynchronized lazy write in the mongo provider, and three URL-assembly
bugs in `config.MarshalServices`.

**Defects fixed here, enumerated for the Task 12 acceptance check:**

1. `program://` — the query string is never stripped, so `program://ps?cron=…` execs a binary
   literally named `ps?cron=…` (`program.go:32`)
2. `file://` — same, stats a path ending `.txt?cron=…` (`file_provider.go:32`)
3. `cert://` — same, dials `example.com?cron=…:443` (`certificate_provider.go:18`), and reports the
   query as part of `body.host` (`certificate_provider.go:40`)
4. `program://` — `?args=` yields exactly one argv entry, and a bare URL passes one empty string
   (`program.go:32-36`)
5. `ProgramProvider.WithShell` never applies: `cmd` is built before the branch, which only rewrites
   the reported string (`program.go:40-43`), while `main.go:90` sets it `true`
6. `config.MarshalServices` emits `?args="a b"` with literal quotes, so the documented
   `args: [a, b]` reaches `exec` as one quoted argv entry (`config.go:176-182`)
7. `file://` — a nil `os.FileInfo` is cached on the not-found path (`file_provider.go:38`), then
   `last.Size()` panics (`file_provider.go:64`) and the deferred writer deadlocks on the mutex the
   panicking frame holds (`file_provider.go:63`); the goroutine never returns and `/status` hangs
   forever
8. `file://` — a zero-byte file returns `(0, io.EOF)` and a directory returns EISDIR, both becoming
   a provider error that `service.go:146` turns into a 500 with no `Body`
9. `mongo://` — `template.Must` on the config-supplied `count` query (`mongo_provider.go:296`)
   panics; `syncs@v1.3.2` has no `recover`, so a config typo kills the process
10. `mongo://` — `m.now = time.Now` is written inside `Status` (`mongo_provider.go:36-38`) on a
    provider shared by every mongo service, so two concurrent services race on it
11. `config.MarshalServices` emits `&countQuery=` (`config.go:162`) while the provider reads `count`
    (`mongo_provider.go:192`), so a YAML `count_query` is always ignored
12. `config.MarshalServices` appends `&collection=`, `&db=` and `&countQuery=` without ensuring a
    `?` exists (`config.go:155-164`) — only `oplogMaxDelta` checks — so an entry without
    `oplog_max_delta` yields `mongodb://host/db&collection=x` with the query inside the path

The new capability is nightly-artifact monitoring: a `file://` target may carry shell glob
wildcards and `[[.YYYY]]`-style date templates, so a backup named
`1788412235_2026_09_03_19.2.4_gitlab_backup.tar` can be matched by
`file:///backups/*_[[.YYYY]]_[[.MM]]_[[.DD]]_*_gitlab_backup.tar` and its `since_modif` used as the
freshness signal.

A dependency and toolchain upgrade phase lands first, as separately green commits, so a later
regression can be attributed without unpicking it from the behavior changes.

## Context (from discovery)

- files involved: `app/status/external/{program,file_provider,certificate_provider,mongo_provider,service}.go`,
  `app/config/config.go`, `app/main.go`, `README.md`, `.github/workflows/{ci,release}.yml`, `go.mod`, `vendor/`
- `Service.lastResponses.cache` is `map[Request]Response` (`service.go:72`), so `Request` must stay
  `{Name, URL}` of comparable strings — parsing happens inside each `Status`
- providers run in a `syncs.SizedGroup` (`service.go:95`) with no `recover`, so any provider panic
  kills the process, and one provider instance is shared across all services of its scheme
- a provider error becomes `Response{StatusCode: 500}` with no `Body` (`service.go:146`), so every
  `body.*` condition downstream evaluates against a missing field
- `external.NewDayTemplate` (`mongo_provider.go:240`) emits BSON extended JSON (`{"$date":"..."}`);
  it is a mongo concern and must not be generalized or reused
- `vendor/` is present, so every dependency change requires `go mod vendor`, and every inventory
  command needs `-mod=mod` or it silently reports nothing
- 30 modules have within-major updates available, including `go-pkgz/rest`, `testify`,
  `go-pkgz/mongo/v2`, `lgr`, `syncs`, `fileutils`, `routegroup` and all `golang.org/x`

## Development Approach

- **testing approach**: TDD — every defect here has a reproducible failure; write the failing test,
  run it to confirm it fails, fix, run again
- complete each task fully before moving to the next
- make small, focused changes
- **CRITICAL: every task that changes code MUST include new/updated tests** in that task
  - tests are not optional - they are a required part of the checklist
  - write unit tests for new functions/methods
  - write unit tests for modified functions/methods
  - add new test cases for new code paths
  - update existing test cases if behavior changes
  - tests cover both success and error scenarios
  - tasks 1-3 change toolchain and workflow files only; their gate is the existing suite plus lint
- **CRITICAL: all tests must pass before starting next task** - no exceptions
- **CRITICAL: update this plan file when scope changes during implementation**
- run tests after each change
- one commit per task that changes files, each independently green; documentation for a behavior
  change goes in that behavior's commit, not a trailing docs-only commit. Task 12 is acceptance
  only and produces no commit of its own.

## Code-Quality Rules (HARD — verify against every task before marking complete)

These rules supplement project CLAUDE.md and are NOT optional. They are the gate for marking any task complete. If a rule is violated, the task is not done — refactor, re-test, then mark complete.

**Signatures (hard limits):**
- No function or method has 4+ parameters. `ctx context.Context` does not count toward the budget. If you need 4+, use an option struct (e.g., `type fooOpts struct { ... }`).
- No function or method has 4+ return values. Split the function into two single-purpose ones, or return a struct.
- Multiple adjacent same-type parameters (`oldLine, newLine int`) are a swap hazard — review whether they belong on a struct.

**Methods vs standalone helpers (project rule, hard):**
- If a function is called only from methods of a single struct, it MUST be a method on that struct. Calling pattern decides, not field access.
- Standalone helpers are reserved for: (a) constructors and entry points (`Parse...`, `New...`, `Decorate...`), (b) utilities shared by multiple unrelated types or by both standalone functions AND methods, (c) tiny cross-cutting helpers.
- Before adding any standalone helper, mentally walk its callers. If every caller is a method of one type, make the helper a method on that type.

**Visibility (private by default, hard):**
- Lowercase identifiers by default. Only export when an out-of-package caller exists.
- Exception (per CLAUDE.md): methods called by other structs in the same package CAN be exported for inter-component API clarity. This is the only exception. It does not extend to types, functions, constants, or variables.
- Before exporting any new identifier, grep for cross-package callers. If none, lowercase it.

**Comments (default: none, hard):**
- Default to writing no comments. Add one only when the WHY is non-obvious (a hidden invariant, a workaround, behavior that would surprise a reader).
- Exported items get godoc comments starting with the name. Unexported items get lowercase non-godoc comments — or no comment at all.
- Never describe WHAT the code does when the code itself is self-evident. Never write multi-paragraph comments on routine helpers.

**Per-task gate (before marking ANY checkbox complete):**
1. Formatter runs clean (`~/.claude/format.sh` or `gofmt -s -w` + `goimports -w`).
2. `golangci-lint run --max-issues-per-linter=0 --max-same-issues=0` reports zero issues.
3. `go test ./... -race` passes.
4. Scan the new code for the four rule classes above. Specifically:
   - Grep new function signatures: `grep -nE '^func.*\(.*,.*,.*,.*\)' app/<path>/*.go` — any hit with 4+ comma-separated params (excluding `ctx`) is a violation. Same for the return-value side.
   - For every new standalone helper, `grep -rn 'helperName(' --include='*.go'` and confirm at least one caller is NOT a method of a single type. If all callers are methods of one type, convert.
   - For every new exported identifier, grep cross-package. If no out-of-package hit, lowercase it.
5. Only after 1–4 pass: mark the task complete.

If a previous task shipped a violation (spotted later by user, reviewer, or yourself): fix it in the next commit BEFORE starting the next task. Do not let violations accumulate.

## Testing Strategy

- **unit tests**: required for every code task (see Development Approach above)
- **e2e tests**: project has none (no Playwright/Cypress); not applicable
- **test comments**: none, per the project rule — the test name states the behavior and the fixture
  carries the intent. A regression test's name should name the defect it pins.
- **timezone**: date-template tests pin a non-UTC location so a UTC-only implementation fails
- **URL forms**: pin both accepted provider forms (`scheme://host` and `scheme:///abs/path`), and
  pin the exact input for a rejected one — `program://ps:abc` errors with `invalid port`. Do not pin
  continued *acceptance* of another malformed spelling such as `program://ps:80:90`: it parses on
  go1.27.1 but that is incidental stdlib behavior of the kind Go 1.26 began tightening.
- **no NEW live network in tests**: the inherited `TestCertificateProvider_Status`
  (`certificate_provider_test.go:13`) already dials `umputun.com` and asserts on `body["host"]`.
  The provider hardcodes `+ ":443"` so it cannot target a test listener, and replacing that fixture
  is out of scope. The `?cron=` stripping is therefore proven twice: purely in the `parseTarget`
  tests for the dial target, and by extending the existing live test with a `?cron=` variant for
  `body["host"]`. New tests add no further network calls.
- **mongo**: `MONGO_TEST` integration cases stay conditional on a reachable instance

## Progress Tracking

- mark completed items with `[x]` immediately when done
- add newly discovered tasks with ➕ prefix
- document issues/blockers with ⚠️ prefix
- update plan if implementation deviates from original scope
- keep plan in sync with actual work done

## Solution Overview

**Upgrade phase first, as three separately green commits.** A single "toolchain + deps + actions"
commit would defeat the bisection it is meant to enable.

**One shared target parser, in its own file.** Each of the three broken providers currently
re-derives the `Host`+`Path` join differently, which is how the bugs arose. `provider_url.go` holds
an unexported `parseTarget`; it does not belong in `service.go`, which owns orchestration rather
than provider URL syntax.

**Program execution contract changes.** `WithShell` is deleted rather than repaired: it has only
ever rewritten a display string, so there is no installed base, and once `args` is `[]string` a
shell mode has no coherent contract — quoting each argv defeats pipes, joining raw makes argv into
shell source. A deployment that needs a shell writes it explicitly and per-check as
`path: /bin/sh, args: ["-c", "pipeline"]`, which also names the dependency in a config the operator
can read. The official image is `umputun/baseimage:scratch-latest` and has no `sh` at all.

**File provider resolution flow** is: parsed query-free target → date-template expansion →
`filepath.Glob` → stat each match → newest mtime wins. The delta cache is keyed on the **parsed,
query-free target before template expansion**, because both the expanded pattern and the resolved
path change daily and would reset the key every night.

**Date expander stays local and minimal.** Not lifted from the sibling `cronn` repo: roughly half of
its `app/service/day.go` is trading-calendar machinery (`HolidayChecker`, `skipWeekDays`, `eodHour`,
`weekdayBackward`, the `W`-prefixed and `EOD` field sets) with no domain here. Fields are
atomic-first — real filenames use whatever separator their producer chose, and only atomic
components can be reassembled to match, which is why `[[.YYYY]]_[[.MM]]_[[.DD]]` matches the
motivating backup and `[[.YYYYMMDD]]` does not. No `UNIX`/`UNIXMSEC`: they change every poll and
cannot identify an artifact created earlier. No `ISODATE`: colons and a `Z` suffix have no plausible
filename use, and local midnight formatted with a literal `Z` would falsely claim UTC.

**Midnight window is documented, not mechanised.** An exact-day template reports `not found` from
00:00 until the job finishes. Glob-newest plus `since_modif` has no such window and is the
recommended freshness check; the README says so. The existing `cron` filter is not the answer —
`cronFilter` returns true only in the minute before each scheduled time and `service.go:107` serves
`lastResponses.cache` the rest of the time, so a late artifact leaves a stale `not found` cached
until the next tick.

## Technical Details

**Shared target parser** (`app/status/external/provider_url.go`, unexported, standalone — called
from three unrelated provider types' methods):

```go
// parseTarget splits a provider url into its target and query values. wantScheme is validated
// against the url's scheme. target is host+path so both scheme://name and scheme:///abs/path work.
func parseTarget(rawURL, wantScheme string) (string, url.Values, error)
```

`wantScheme` takes the bare scheme (`"program"`, not `"program://"`) and the function errors on a
mismatch — that gives the parameter a job, without which `unparam` and revive's `unused-parameter`
fail the task's own lint gate (`.golangci.yml:92-96` excludes that rule for `_test.go` only). It
uses `url.ParseQuery(u.RawQuery)` rather than `u.Query()`, because the latter silently discards a
malformed escape while the former returns the error. It errors on an empty target, and on a
non-empty raw fragment: silently turning `file:///tmp/a#b` into `/tmp/a` is retargeting, and `%23`
is the supported way to write a literal `#`. A raw `?` stays documented as query syntax, with `%3F`
for the glob wildcard.

**Percent-encoding, verified on go1.27.1** — this is a behavior change for existing configs and
must be documented and tested, not discovered in production:

| target | today (`TrimPrefix`) | after (`url.Parse`) |
|---|---|---|
| `file:///tmp/log_%3F%3F.txt` | literal `%3F%3F` | `log_??.txt`, a working glob wildcard |
| `file:///tmp/a%23b` | literal `%23` | `a#b` |
| `file:///tmp/x%20y.txt` | file named `x%20y.txt` | file named `x y.txt` |
| `file:///backups/50%_done.tar` | works | **error**: `invalid URL escape "%_d"` |
| `file:///tmp/a?b` | literal `?b` | target `/tmp/a`, query `b` |
| `file:///tmp/a#b` | literal `#b` | **error**: raw fragment rejected |

So a raw `?` or `#` in a path must be percent-encoded, and a bare `%` — legal in a filename — now
fails. A `%3F` is how the caller asks for `filepath.Glob`'s single-character wildcard.

**Program provider** after the change:

```go
target, query, err := parseTarget(req.URL, "program")
args := query["args"]                       // repeated ?args=a&args=b, nil when absent
cmd := exec.CommandContext(ctx, target, args...)
```

`res["command"]` reports `target` joined with `args`, which is what actually ran.

**File provider resolution**:

```go
type fileMatch struct {
    path       string      // resolved path of the newest match
    info       os.FileInfo
    matchCount int         // successfully statted matches
}

func newFileDateFields(ts time.Time) fileDateFields
func (f *FileProvider) clock() time.Time
func (f *FileProvider) expandTarget(target string) (string, error)
func (f *FileProvider) resolveTarget(target string) (fileMatch, error)
```

`clock()` reads `f.now` and falls back to `time.Now` without writing the field — the lazy-write
pattern at `mongo_provider.go:36-38` is defect 10 and must not be copied. `resolveTarget` expands,
globs, stats each match skipping any that vanished between glob and stat, propagates a non-ENOENT
stat error, and returns the newest by mtime with a deterministic tie-break on path.

The delta cache maps the parsed query-free target to `{resolvedPath, os.FileInfo}`. On not-found it
**keeps the previous good entry rather than deleting it**, so the first successful poll after a gap
still reports a meaningful `size_change` instead of the full file size. It never stores a nil
`FileInfo`, which is defect 7.

**Date fields** available in a `file://` target, `[[.X]]` delimiters to match the mongo provider,
expanded at midnight of the current day in the process-local zone so `TZ` controls it:

| field | value on 2026-09-03 |
|---|---|
| `YYYY` `YY` `MM` `DD` | `2026` `26` `09` `03` |
| `YYYYMMDD` `YYYYMM` `YYMMDD` | `20260903` `202609` `260903` |

**Mongo template**: `mongo_provider.go:296` uses `template.Must(tmpl.Parse(...))` on the
config-supplied `count` query, and its `Execute` failure path logs a WARN and returns the
partially-rendered string. Both are silent-corruption paths: a parse error kills the process, an
execute error (`[[.Nope]]`) produces a broken BSON filter. `Parse` returns `(string, error)` and the
`countQuery` call site (`mongo_provider.go:191`) propagates it.

**Version pin trap**: `.github/workflows/release.yml:29` reads `version: ~> 1.25` under
`goreleaser-action` — that selects the GoReleaser CLI, not Go. Do not sweep it with the Go bump. It
is genuinely stale (GoReleaser is on 2.x) but upgrading it changes the `.goreleaser.yml` schema and
needs a release dry-run, so it is deliberately out of scope here.

## What Goes Where

- **Implementation Steps** (`[ ]` checkboxes): code, tests and documentation in this repository
- **Post-Completion** (no checkboxes): deployment verification and follow-up work

## Implementation Steps

### Task 1: Raise the Go directive to 1.27 and record a coverage baseline

**Files:**
- Modify: `go.mod`
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`
- Modify: `app/main.go`
- Modify: `app/status/external/http_provider_test.go`

Baseline with Go 1.25.0: `app` 75.9%, `actuator` 100.0%, `config` 98.4%, `server` 93.1%, `status` 70.3%, and `status/external` 85.9%.

Validation: `docker build .`, `go test ./... -race` with MongoDB, formatter checks, and golangci-lint 2.13.2 built with Go 1.27.0 passed. The Go 1.27 `embedlit` modernization required updates to three embedded `http.Client` literals.

- [x] record the pre-change baseline: `go test -cover ./app/... | tee` the result into this task's notes, so Task 12's coverage check is decidable
- [x] set the `go` directive in `go.mod` to `1.27.0`
- [x] set `go-version: "1.27"` in `ci.yml:33` and `release.yml:24`, and rename the ci step label at `ci.yml:30`
- [x] leave `release.yml:29` `version: ~> 1.25` untouched — it selects GoReleaser, not Go
- [x] run `docker build .` to confirm `umputun/baseimage:buildgo-latest` satisfies the new directive
- [x] run `go test ./... -race` and `golangci-lint run --max-issues-per-linter=0 --max-same-issues=0` — must pass before task 2

### Task 2: Refresh modules within current majors

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `vendor/` (regenerated)

Update result: 20 modules used by this repository's packages or tests moved within their existing major lines, and testify added `go.yaml.in/yaml/v3` as an indirect dependency. A second `go get -t -u ./...` pass updated test-only `fileutils`; seven newer modules remain only in dependency metadata and are neither root requirements nor vendored packages.

Dependency review: rest 1.21.0 to 1.24.0 changes the used `Recoverer` behavior only for `http.ErrAbortHandler`. The `AppInfo`, `Throttle`, `Ping`, `RenderJSON`, and `SendErrorJSON` APIs used by `server.go` remain compatible, and the server tests pass.

Validation: `go mod verify`, `go test -mod=vendor ./...`, `go test -race ./...` with MongoDB, formatter checks, and golangci-lint 2.13.2 passed.

- [x] run `GOFLAGS=-mod=mod go get -u ./...` then `go mod tidy` — without `-mod=mod` the vendor directory makes every inventory command silently report nothing
- [x] run `go mod vendor` — the repo vendors, so a stale `vendor/` breaks the build
- [x] confirm no major-version line changed: `gopsutil` stays on `/v3` and `go.mongodb.org/mongo-driver` on v1, both deferred
- [x] run `go test ./... -race` and check for behavior changes in `go-pkgz/rest` 1.21→1.24
- [x] run the linter — must pass before task 3

### Task 3: Refresh GitHub Actions tags

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml`
- Modify: `.github/workflows/ci-site.yml`

Validation: actionlint 1.7.12 passes with only the existing empty trigger and shellcheck categories ignored. Its full run reports those pre-existing findings and no version-tag issue. The race suite with MongoDB and golangci-lint 2.13.2 also pass.

- [x] bump `actions/checkout@v6` to `@v7` in all three workflows
- [x] bump `actions/setup-go@v6` to `@v7` in `ci.yml` and `release.yml`
- [x] confirm the remaining actions are already current (`golangci-lint-action@v9`, `upload-artifact@v7`, `download-artifact@v8`, all four `docker/*`)
- [x] validate locally with `actionlint` — remote CI cannot gate this task, since nothing is pushed until Eugene approves a push
- [x] run the suite and linter — must pass before task 4

### Task 4: Add the shared provider target parser

**Files:**
- Create: `app/status/external/provider_url.go`
- Create: `app/status/external/provider_url_test.go`

**Design Contract:**

Type:
- none — this task adds one unexported standalone function, no new type

Methods (full signatures):
- none

Standalone helpers planned (justification why NOT a method):
- `parseTarget(rawURL, wantScheme string) (string, url.Values, error)` — called from the `Status` methods of three unrelated provider types (`ProgramProvider`, `FileProvider`, `CertificateProvider`). It is a utility shared by multiple unrelated types, which is the standalone exception; making it a method on any one of them would force the other two to reach across types.

Exports (justification per item: who outside the package calls this?):
- none — lowercase, no out-of-package caller

- [x] write failing tests first for `parseTarget` covering `program://ps`, `program:///abs/path.sh`, `file://rel/f.txt`, `file:///abs/f.txt`, `cert://example.com`, each with and without `?cron=`
- [x] add cases pinning the percent-encoding table in Technical Details, including `%3F` becoming a glob wildcard and `50%_done.tar` returning an error
- [x] add cases for a scheme mismatch, an empty target, a non-empty raw fragment (`file:///tmp/a#b` must error rather than silently retarget to `/tmp/a`), and a malformed query escape (`?cron=%zz`) which `url.ParseQuery` reports and `u.Query()` swallows
- [x] add a case pinning `program://ps:abc` errors with `invalid port` — do not pin acceptance of `program://ps:80:90`, which parses today only incidentally
- [x] implement `parseTarget` in `provider_url.go` with a lowercase non-godoc comment, returning `u.Host + u.Path` and `url.ParseQuery(u.RawQuery)`, erroring on an empty target and a non-empty `u.Fragment`
- [x] run tests - must pass before task 5

### Task 5: Fix program provider argument handling and delete WithShell

**Files:**
- Modify: `app/status/external/program.go`
- Modify: `app/status/external/program_test.go`
- Create: `app/status/external/testdata/argcount.sh`
- Modify: `app/main.go`
- Modify: `README.md`

- [x] add `testdata/argcount.sh` printing `$#` and each argument, so argv shape is observable — no existing fixture can distinguish zero args from one empty one
- [x] write a failing test pinning that `program://ps?args=-e&args=-f` produces two argv entries, not one
- [x] write a failing test pinning that `program:///path/to/prog` passes zero argv entries rather than one empty string, asserting on `argcount.sh` output
- [x] write a failing test pinning that `program://ps?cron=0_6_*_*_*` execs `ps`, not a binary named `ps?cron=...`
- [x] merge `TestProgram_StatusWithShell` and `TestProgram_StatusWithoutShell` — both set `WithShell: true` (`program_test.go:12` and `:44`), so the package will not compile once the field is gone, and they are near-duplicates without it
- [x] replace the `TrimPrefix` and `?args=` split with `parseTarget`, build `exec.CommandContext(ctx, target, args...)`, and report the real command in `res["command"]`
- [x] delete the `WithShell` field and its branch, and drop `WithShell: true` from `app/main.go:90`
- [x] add a test running `/bin/sh` with `args: ["-c", "…"]` to pin the documented escape hatch for shell features
- [x] update README:259 and :262-263 in this commit: remove "All commands are executed in shell", document the repeated `?args=` form, the `/bin/sh -c` escape hatch, that the `.sh` example needs an interpreter the scratch image lacks, and the percent-encoding rules
- [x] run tests - must pass before task 6

### Task 6: Fix config URL assembly for program args and mongo query parameters

**Files:**
- Modify: `app/config/config.go`
- Modify: `app/config/config_test.go`

- [x] write a failing round-trip test that stays inside package `config`: take the string `MarshalServices` emits and decode it with `net/url`, asserting `u.Query()["args"]` is exactly `["arg1", "arg2"]` — `parseTarget` is unexported in package `external` and cannot be called from here
- [x] add cases for an argument containing a space and one containing `&`, to pin escaping
- [x] replace the `?args="` + join + `"` construction at `config.go:176-182` with repeated escaped `args=` pairs built through `url.Values`
- [x] add a case for a program entry with no args, asserting no `?args=` is emitted
- [x] write a failing test for defect 11: a mongo entry with `count_query` set must produce a URL whose `count` parameter carries it — `config.go:162` emits `countQuery=` while `mongo_provider.go:192` reads `count`, so the YAML field is silently ignored today
- [x] write a failing test for defect 12: a mongo entry with `collection` and `db` but no `oplog_max_delta` must produce a parseable query — `config.go:155-164` appends `&collection=` without ensuring a `?` exists, yielding `mongodb://host/db&collection=x` with the query inside the path
- [x] rebuild the mongo branch's query through `url.Values` so the separator is correct by construction rather than by a `strings.Contains(m, "?")` check
- [x] run tests - must pass before task 7

### Task 7: Strip query strings in the file and certificate providers

**Files:**
- Modify: `app/status/external/file_provider.go`
- Modify: `app/status/external/certificate_provider.go`
- Modify: `app/status/external/file_provider_test.go`
- Modify: `app/status/external/certificate_provider_test.go`

- [x] write a failing test pinning that `file:///tmp/x.txt?cron=0_6_*_*_*` stats `/tmp/x.txt`
- [x] cover the cert dial target purely in the Task 4 `parseTarget` tests — the provider hardcodes `+ ":443"` so no test listener can be targeted, and no new live-network test is added
- [x] write a failing test pinning that `body["host"]` carries no query, by extending the inherited `TestCertificateProvider_Status` (`certificate_provider_test.go:13`, which already dials `umputun.com`) with a `cert://umputun.com?cron=0_6_*_*_*` case asserting `body["host"] == "https://umputun.com"`: `certificate_provider.go:40` derives it a second time from the raw `req.URL`, so fixing only the dial leaves the query in the reported host
- [x] replace the `TrimPrefix` at `file_provider.go:32` and both raw-URL uses in `certificate_provider.go` (`:18` and `:40`) with `parseTarget`
- [x] keep the relative-path form working — `file://foo/bar.txt` must still resolve to `foo/bar.txt`
- [x] add a test pinning that a bare `%` in a target now returns an error rather than a literal path, and confirm it surfaces as a 500 with no body per `service.go:146`
- [x] run tests - must pass before task 8

### Task 8: Fix file provider read and delta-cache correctness

**Files:**
- Modify: `app/status/external/file_provider.go`
- Modify: `app/status/external/file_provider_test.go`

- [ ] write a failing test for the missing-then-present sequence: it currently panics on `last.Size()` and then deadlocks in the deferred writer on the mutex the panicking frame holds, so run it under `-timeout 30s` and assert a normal response
- [ ] write a failing test for a zero-byte file, which returns `(0, io.EOF)` and today becomes a 500 with no body
- [ ] write a failing test for a directory target, which fails at `fh.Read` with EISDIR
- [ ] remove the deferred cache writer entirely: snapshot `last` under the lock, compute the deltas outside it, and store only at the successful end of `Status` — the defer is what deadlocks, and it also records stat info when a later open or read fails
- [ ] on not-found, keep the previous good entry rather than storing nil or deleting, so the first success after a gap still reports a meaningful `size_change`
- [ ] treat `io.EOF` as success with empty content, skip the read entirely when `fi.IsDir()`, and set `body["content"] = ""` in both cases so a downstream `body.content` condition still has a field
- [ ] run tests - must pass before task 9

### Task 9: Return errors instead of panicking or silently corrupting mongo templates

**Files:**
- Modify: `app/status/external/mongo_provider.go`
- Modify: `app/status/external/mongo_provider_test.go`

- [ ] write a failing test passing `count` templates that panic today: `{"date": "[[.YYYYMMDD"}` and `[[ .YYYYMMDD | nosuchfunc ]]`
- [ ] write a failing test for `[[.Nope]]`, an Execute error that today logs a WARN and returns the partially-rendered string, producing a broken BSON filter
- [ ] change `DayTemplate.Parse` (`mongo_provider.go:293`) to return `(string, error)` for both failure modes, and update its godoc, which currently says "Parse translate template to final string"
- [ ] update the `countQuery` call site at `mongo_provider.go:191-197` to propagate the error
- [ ] add a test asserting a valid template still expands to the documented `{"$date":...}` form, so the shape is not changed
- [ ] run tests - must pass before task 10

### Task 10: Remove the shared-provider clock race in the mongo provider

**Files:**
- Modify: `app/status/external/mongo_provider.go`
- Modify: `app/status/external/mongo_provider_test.go`

- [ ] write a failing concurrency regression: call `Status` on one `*MongoProvider` from two goroutines and run it under `-race`, since `main.go:88` creates a single instance shared by every mongo service and `service.go:95` runs them concurrently
- [ ] replace the unsynchronized `m.now = time.Now` write at `mongo_provider.go:36-38` with a read-only accessor treating nil as `time.Now`, mirroring the `clock()` shape Task 11 uses for the file provider
- [ ] confirm the existing tests that inject `now` still work through the accessor
- [ ] run tests under `-race` - must pass before task 11

### Task 11: Add glob and date-template resolution to the file provider

**Files:**
- Modify: `app/status/external/file_provider.go`
- Modify: `app/status/external/file_provider_test.go`
- Modify: `README.md`

**Design Contract:**

Type:
- `fileDateFields` (unexported — a template data holder, no methods, no out-of-package caller)
- `fileMatch` (unexported — a result holder with no methods; it exists so `resolveTarget` returns two values rather than four)

Methods (full signatures):
- `(f *FileProvider) clock() time.Time`
- `(f *FileProvider) expandTarget(target string) (string, error)`
- `(f *FileProvider) resolveTarget(target string) (fileMatch, error)`

Standalone helpers planned (justification why NOT a method):
- `newFileDateFields(ts time.Time) fileDateFields` — constructor for the data holder, which is exception (a); it is called from `expandTarget` and from its own tests

Exports (justification per item: who outside the package calls this?):
- none

- [ ] add an unexported `now func() time.Time` field to `FileProvider` and a `clock()` accessor treating nil as `time.Now` — `main.go:93` builds a bare struct literal so the field is always nil in production, and the lazy-write pattern at `mongo_provider.go:36-38` would reproduce defect 10
- [ ] write failing tests with the clock pinned to a non-UTC location, so a UTC-only implementation fails
- [ ] define `fileDateFields` with `YYYY YY MM DD YYYYMMDD YYYYMM YYMMDD`, computed at midnight in the process-local zone
- [ ] implement `expandTarget` with `[[`/`]]` delimiters, returning an error on parse failure rather than using `template.Must`, and on an unknown field such as `[[.Nope]]`
- [ ] write a failing test with three files in a temp dir, asserting `*_gitlab_backup.tar` selects the newest by mtime
- [ ] write a failing test asserting `*_[[.YYYY]]_[[.MM]]_[[.DD]]_*_gitlab_backup.tar` selects exactly today's file, and that `*_[[.YYYYMMDD]]_*` selects none, since that filename separates its date with underscores
- [ ] write a failing test asserting `log_%3F%3F.txt` globs as `log_??.txt` and matches two-character names only
- [ ] implement `resolveTarget`: expand, `filepath.Glob`, stat each match skipping any that vanished, propagate a non-ENOENT stat error, pick newest mtime with a deterministic tie-break on path
- [ ] add `path` and `match_count` to the response body, and key the delta cache on the parsed query-free target before expansion, storing `{resolvedPath, FileInfo}` as the value
- [ ] write a test asserting the exact cross-artifact `size_change` and the reported `path` across two polls whose resolved filename differs, and a third poll with a not-found in between to pin that the baseline survives
- [ ] write a guard test that a glob matching nothing returns `status: not found` with code 200 — note this already holds today, since `os.Stat` on a path containing `*` returns `ErrNotExist`, so it pins behavior rather than failing first
- [ ] update the README `file://` section in this commit: fix the response example at README:356 which is labelled `"cert"`, add the undocumented `content` field plus the new `path` and `match_count`, document glob and date templates, the percent-encoding rules, the process-local timezone, the midnight window on exact-day templates with glob-newest plus `since_modif` as the recommended alternative, and that a missing file returns 200 with `body.status = "not found"` so a status-code-only condition is silently satisfied
- [ ] run tests - must pass before task 12

### Task 12: Verify acceptance criteria

- [ ] verify each of the twelve defects enumerated in Overview has a regression test that fails without the fix; the glob-not-found case in Task 11 is a feature guard, not one of the twelve, and is documented as pinning existing behavior
- [ ] verify the percent-encoding table in Technical Details is covered case for case
- [ ] run full test suite: `cd app && go test -race -timeout=60s -count 1 ./...`
- [ ] run `MONGO_TEST=mongodb://127.0.0.1:27017 go test ./...` if a local mongo is reachable; skip and note otherwise
- [ ] run `golangci-lint run --max-issues-per-linter=0 --max-same-issues=0` from the top level
- [ ] compare `go test -cover ./app/...` against the baseline recorded in Task 1

### Task 13: [Final] Update remaining documentation

- [ ] update CLAUDE.md's provider URL section for the repeated `?args=` form, the removal of shell execution, and the glob/template syntax
- [ ] confirm the README changes made in Tasks 5 and 11 are complete and consistent
- [ ] move this plan to `docs/plans/completed/`

## Post-Completion

*Items requiring manual intervention or external systems - no checkboxes, informational only*

**Manual verification**:
- deploy to one host and confirm `/status` still answers with the refreshed dependencies
- confirm existing `program://` configs still behave identically — every current config is a direct
  exec today, so none should change, but a config with a space in `?args=` changes meaning, and any
  target containing a bare `%` now errors
- confirm the nightly backup glob resolves against the real `/backups` directory

**Deferred, deliberately out of scope**:
- `gopsutil/v3` → `/v4`: 5 import lines in `app/status/status.go`, its own change
- `go.mongodb.org/mongo-driver` v1 → v2: blocked on `go-pkgz/mongo`, whose latest `v2.3.0` still
  requires driver `v1.17.9` and which has no `v3`; a migration means replacing that wrapper
- GoReleaser `~> 1.25` → `~> 2`: changes the `.goreleaser.yml` schema and needs a release dry-run
- adding the agent version to the `/status` JSON body, which today is only the `App-Version` header
- a `day-start` cutoff to close the exact-day template midnight window
- unexporting `DayTemplate`, `NewDayTemplate` and `Parse`, which have no out-of-package callers

Smells pre-check: 10 items fixed before save — `scheme` parameter given a validating job, `parseTarget` moved out of `service.go`, unexported comment style corrected, `fileDateFields` constructor named, `clock()` accessor replacing the racy lazy write, `content` key preserved for directory and empty-file cases, certificate `body["host"]` added to Task 7, `match_count` naming, README `path`/`match_count` and repeated-args gaps closed, `DayTemplate` godoc update added to Task 9.
