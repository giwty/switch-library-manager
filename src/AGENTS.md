# Go Application Code

## Entry Points and Flow

- `main.go` resolves the application data folder and does not use the current working directory. macOS stores runtime files under the user config directory; Windows and Linux use the executable directory.
- Startup loads settings, creates `slm.log`, initializes CLI flags, and selects GUI or console mode. `-m console` and `-m gui` override `settings.json`.
- The console flow downloads/loads remote titles and versions, scans the library, reports issues, optionally deletes old updates, organizes files, and checks missing updates/DLC.
- The GUI flow starts Wails, loads the embedded `frontend/index.html`, and exposes typed methods from `backend/app.App` to `app.js`. It maintains mutable local and remote DB state in `App.state`.
- `db.ProgressUpdater` is the progress callback used by both CLI and GUI. Do not assume progress totals are always nonzero or that callbacks are synchronous unless the caller guarantees it.

## Package Boundaries

- `main` owns executable startup and mode selection.
- `backend/app` owns Wails lifecycle, GUI state, frontend bindings, and presentation adapters.
- `backend/consoleapp` owns CLI workflow, progress output, and CSV export.
- `backend/db` owns remote/local models, scanning, cache reads/writes, and persistence.
- `backend/switchfs` owns parsing and decryption of external binary formats.
- `backend/fileio` adapts split files to the parser layer.
- `backend/process` owns content comparisons and destructive organization.
- `backend/settings` owns JSON configuration and key discovery.
- `backend/console` owns flags and Windows console attachment.

When adding reusable logic, put it in the narrowest package that owns the behavior. Do not put parser, database, or file-organization logic in `backend/app` or `backend/consoleapp` merely because those adapters call it.

## Shared State and JSON Contracts

- `settings.ReadSettings` and `settings.SwitchKeys` currently use process-global singletons. A later `baseFolder` is ignored after initialization. Tests that need isolation must use separate processes or first refactor toward explicit state.
- Wails method signatures and event names are an API between `backend/app` and `frontend/app.js`. Update both sides together and preserve the asynchronous callback behavior.
- Important bindings include settings/key/update/database/library/organization methods. Important events are `updateProgress`, `error`, and `rescan`.
- Keep JSON field names stable unless a coordinated frontend change or migration is included.

## Error and Concurrency Rules

- Return errors from constructors and operations rather than calling `log.Fatal` or panicking in package code.
- Do not ignore errors from BoltDB, file reads/writes, JSON decoding, `os.Rename`, `os.Remove`, or frontend message serialization.
- GUI operations are serialized through `State.Mutex`, but maps, settings, keys, and the database still have shared mutable state. Recheck locking before adding asynchronous work.
- Do not use `os.Executable` or executable-relative runtime files as implicit test fixtures. Tests should pass explicit temporary directories.

## Commands

Prefer the shared automated workflow below. Do not request desktop control for routine validation or setup failures; native desktop automation requires an explicit user request.

```text
cd src
npm test
npm run test:coverage
npm run test:race
```

- `npm run test:backend` runs Go formatting checks, `go test ./...`, explicit crypto tests, and vet. The runner normalizes CRLF only in disposable formatting copies and never rewrites source files.
- `npm run test:unit` and `npm run test:ui` run the JavaScript and headless Chromium layers independently. Node.js 24 and the pinned development dependencies are test tooling, not an application frontend build pipeline.
- Always run `go test ./...` when verifying a Go task; a task is not complete until the tests pass.
- Core logic coverage is required to be at least 90% in aggregate across `backend/db`, `backend/fileio`, `backend/process`, `backend/settings`, `backend/switchfs`, and `backend/switchfs/_crypto`. The GUI, CLI orchestration, and generated bindata are not part of that threshold.
- The repository uses synthetic fixtures and temporary directories for tests. Report whether verification used filename fallback, synthetic parser/filesystem fixtures, or real encrypted Switch files.
