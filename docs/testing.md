# Agent validation

Use automated backend, JavaScript, and headless UI tests for routine validation. Do not request Windows or desktop control for normal testing, failed tests, or environment setup. Native desktop automation is used only when the user explicitly requests it. Report native behavior that remains unverified.

## First setup

Install Go 1.27 and Node.js 24. From `src`:

```text
npm ci
npx playwright install chromium
npm test
```

On Linux install the Wails GTK/WebKitGTK development libraries needed to compile Go packages. For WebKitGTK 4.1, set `GOFLAGS=-tags=webkit2_41` for test commands (`$env:GOFLAGS = '-tags=webkit2_41'` in PowerShell, `export GOFLAGS=-tags=webkit2_41` in Bash). Install Chromium OS dependencies with `npx playwright install --with-deps chromium` on Linux CI. Browser/dependency installation needs network access once; the actual suites use synthetic/local data.

The Node package is private development tooling. It adds no frontend build step or runtime dependencies to the packaged application. Do not regenerate or replace bundled frontend libraries for these tests.

## Commands

Run from `src`:

| Command | Checks |
| --- | --- |
| `npm test` | Backend, existing Node unit tests, and headless UI tests, in order; stops on failure |
| `npm run test:backend` | Go formatting, `go test ./...`, explicit crypto tests, and vet |
| `npm run test:unit` | All `frontend_tests/*.test.mjs` unit tests |
| `npm run test:ui` | Real frontend in headless Chromium with a simulated Wails bridge |
| `npm run test:coverage` | Combined core statement coverage, failing below 90% |
| `npm run test:race` | Race detector across all packages and the explicit crypto package |
| `npm run test:ui -- --grep layout` | Light/dark layout checks and screenshots at 1280x800 and 800x600 |
| `npm run test:ui -- --grep "settings failure"` | Example of selecting a single browser scenario |

The runner honors `GOFLAGS`, propagates failing exit codes, and explicitly includes `backend/switchfs/_crypto`, which `./...` omits. Formatting is checked on temporary LF copies of tracked/unignored Go source, so Windows CRLF checkouts do not require repository-wide rewrites. Coverage includes `db`, `fileio`, `process`, `settings`, `switchfs`, and `switchfs/_crypto`; GUI/CLI presentation and generated assets are excluded.

Race testing requires `CGO_ENABLED=1` and a compatible C compiler. The command fails clearly if CGO is disabled; run the Linux CI race job when the local toolchain cannot support it. Do not substitute an ordinary test run and claim race verification passed.

## Fixtures and test boundaries

Backend tests exercise real Go logic using temporary files/databases, filename-only fallback data, synthetic parser fixtures, and local HTTP servers. Organization/cleanup must never run against a user's library. No real encrypted Switch images or proprietary keys are required.

Settings and key state are process-global. Tests requiring a fresh singleton must run in a separate subprocess; do not assume a second temporary folder resets the singleton or run those cases in parallel within one process.

Browser tests serve `src/frontend` unchanged. Before its scripts run, the harness installs simulated `window.go.app.App` and `window.runtime` implementations. Generated Wails wrappers remain real. The bridge records calls and supports queued successes, failures, deferred responses, and runtime events. It simulates all native dialogs, file-manager actions, exports, and restart: it never launches the desktop executable or performs organization on host files. Unexpected bridge/runtime calls, console/page errors, HTTP failures, and external requests fail validation. Images are served locally.

`src/frontend_tests/ui/fixtures.json` is serialized from real Go DTOs in `TestFrontendFixtureContracts`. To deliberately update it after reviewing a contract change, from `src`:

```powershell
$env:SLM_UPDATE_UI_FIXTURES = '1'
go test ./backend/app -run '^TestFrontendFixtureContracts$'
Remove-Item Env:SLM_UPDATE_UI_FIXTURES
```

In Bash use `SLM_UPDATE_UI_FIXTURES=1 go test ./backend/app -run '^TestFrontendFixtureContracts$'`. Review the JSON diff and rerun backend and browser tests. Normal tests never update fixtures. Extend the typed Go fixture and explicit bridge behavior when adding a frontend binding; do not add a catch-all success handler.

## Screenshots, traces, and troubleshooting

All artifacts live beneath ignored `src/build/test-results`:

- `playwright-report/index.html`: static report, never opened automatically.
- `ui/`: layout screenshots, failure screenshots, traces, and attachments grouped by test.
- `coverage.out`: combined core coverage profile.
- `_tmp-*`: per-run temporary directories, normally removed by the runner. The underscore prevents Go package discovery from treating temporary sources as application packages.

Inspect PNGs directly with the agent's image/file tools. Traces can be opened manually when useful; do not launch a graphical report viewer as part of automated validation. Layout screenshots are inspection artifacts, not pixel-baseline comparisons. CI uploads reports and screenshots/traces even after test failures.

The runner points `TEMP`, `TMP`, `TMPDIR`, and `GOTMPDIR` at its own workspace-local directory. This avoids Windows agent sandbox rename errors in the default temporary directory. Tests pass temporary folders explicitly instead of using executable-relative application state.

Localhost socket access is required by Go HTTP fixtures and the browser server on `127.0.0.1:4173`. A sandbox may block loopback even though no public network is needed. If access is denied, use the command approval mechanism or an available CI runner; report any remaining blocked commands. Do not weaken tests, silently skip them, request desktop control, or assume these permission errors establish an application defect. Stop any stale test server before retrying; the harness deliberately refuses to reuse an existing server.

Report the exact commands/results, fixture type, and any unverified native behavior. These tests cover Go logic and browser behavior separately. They do not establish native dialog rendering, OS integration, native WebView fidelity, or the complete live browser-to-Go connection. Existing Linux CI additionally builds the packaged app and checks startup under Xvfb.
