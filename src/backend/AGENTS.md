# Backend Packages

The `backend` directory is an organizational namespace, not a single Go package. Keep domain boundaries intact:

- `app` owns Wails lifecycle, GUI state, frontend bindings, and DTOs.
- `consoleapp` owns CLI workflow and terminal presentation.
- `console` owns CLI flags and platform console support.
- `db`, `fileio`, `process`, `settings`, and `switchfs` own reusable application logic.

Keep tests beside the package they exercise. Low-level packages must not depend on Wails or frontend code. Organization and cleanup tests must use temporary directories and synthetic fixtures.

Prefer `npm run test:backend` from `src`, plus coverage and race commands when appropriate. This includes the crypto package omitted by `go test ./...`. Test settings singletons in isolated subprocesses, and use local HTTP fixtures rather than live data sources. Do not request desktop control for routine backend validation or environment failures; native automation requires an explicit user request.
