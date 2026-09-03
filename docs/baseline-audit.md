# Baseline Audit — Day 1

Re-run of the checks listed in `PETCONNECT_AI_AGENT_COMPLETION_BRIEF.md` §5, to verify (not assume) the repository's actual starting state before any implementation work begins.

## Environment

| Tool | Version found | Brief requires |
| --- | --- | --- |
| Flutter | 3.44.3 (stable), Dart 3.12.2 | Dart/Flutter 3.12.2+ |
| Go | go1.24.2 windows/amd64 | Go 1.24+ |

Both meet the brief's minimum versions.

## Git state

- Branch: `feature/onboarding`, 1 commit ahead of `origin/feature/onboarding`.
- Working tree clean apart from this document and the completion brief itself (both untracked, as expected).
- `git status --ignored` shows only standard Flutter/IDE/Go build artifacts ignored — nothing unexpected.
- No `docs/` directory existed prior to this commit.

## Repository shape

Confirmed the brief's top-level path description matches reality:

- `lib/` — Flutter app (`app/`, `core/`, `features/auth`, `features/onboarding`, `features/social`, `shared/`)
- `server/` — Go module (`cmd/api`, `internal/modules/*`, `internal/platform/*`, `internal/realtime`, `migrations/`)
- `test/` — one Flutter test file, `screen_smoke_test.dart`

## Checks run

| Command | Result |
| --- | --- |
| `dart format --output=none --set-exit-if-changed lib test` | Pass — 50 files, 0 would change |
| `flutter analyze` | Pass — no issues found |
| `flutter test --reporter expanded` | Pass — 12/12 tests (all in `screen_smoke_test.dart`) |
| `flutter build web --release` | Pass — see below |
| `go vet ./...` (in `server/`, `GOTELEMETRY=off`, `GOTOOLCHAIN=local`) | Pass — no issues |
| `go test ./...` (same env) | Pass — all testable packages `ok` |

### Go test coverage gap (confirmed, not just claimed)

`go test ./...` reports `ok` for every package that has tests, but only **6 of 13** `internal/modules/*` + `internal/realtime` packages actually contain a `_test.go` file with a `func Test`:

- Has tests: `account`, `auth`, `chat`, `matching`, `notifications`, `pets`, `realtime`
- No test files at all: `adoption`, `community`, `discovery`, `events`, `media`, `social`

This lines up with the brief's truth table — those six untested modules are exactly the ones marked Partial/Missing/mock-only for the client. Green CI on `go test ./...` in this repo does **not** mean those modules are exercised; it means they're silently untested. Flagging this explicitly so it isn't mistaken for coverage later.

### `flutter build web --release`

Pass — `√ Built build\web` (95.2s compile). No wasm build attempted (that's a separate, optional target); the standard release build is clean. One pre-existing note unrelated to this build's success: `flutter_secure_storage_web` uses `dart:html`/`dart:js_util`, which are incompatible with the *wasm* build target specifically — irrelevant to the JS release build used by `flutter run -d chrome` / this build, but worth remembering if a wasm web target is ever adopted later.

The brief's audit said a fresh web release build "was not conclusively completed in the last audit." It now completes cleanly.

## Conclusion

Milestone 0 baseline matches the brief's audit snapshot. No corrections needed to the brief's repository-shape claims. Proceeding to traceability matrix (Day 2).
