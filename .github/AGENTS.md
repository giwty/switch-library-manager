# CI and Packaging

## Workflows

- `workflows/verify-pr.yml` targets master and numeric version branches, runs `actions/setup-environment`, and invokes `actions/build-app`.
- `workflows/build-master.yml` builds master and `vMAJOR.MINOR.PATCH` branches and publishes Windows, both WebKitGTK Linux variants, and macOS artifacts. Documentation-only pushes skip these builds.
- GitHub automatically expires branch artifacts after seven days and tagged-release job-transfer artifacts after one day. Published release assets are unaffected.
- Composite actions use Bash commands with `working-directory: ./src`; the release workflow runs the build on native Windows, Linux, and macOS runners.

## Build Contract

- CI uses Go `1.27`, installs the pinned Wails CLI, generates module bindings, runs `wails build`, checks Linux WebKitGTK linkage, and launches Linux GUI builds under Xvfb.
- Wails writes platform build products under `src/build/bin`; these are not committed.
- Platform targets and WebKitGTK versions in `workflows/build-master.yml` must stay synchronized with artifact names in `actions/publish-artifacts/action.yml` and release archive names.
- Release builds run natively on Windows, Linux, and macOS because Wails desktop builds require platform toolchains and WebView dependencies.

## Change Rules

- Do not commit generated build outputs or platform-specific local build products. Commit Wails JavaScript bindings because this frontend has no package-install/build step.
- Keep setup and build commands reproducible; prefer pinned or module-resolved versions when changing dependencies.
- If adding tests or vet checks, ensure they run from `src` because the repository root is not a Go module.
- PR verification builds the Linux application against WebKitGTK 4.1 on Ubuntu 24.04. Run `go test -tags webkit2_41 ./...` and `go vet -tags webkit2_41 ./...` from `src` before the build. Master and tagged-release workflows continue building both variants: WebKitGTK 4.0 on Ubuntu 22.04 and 4.1 on Ubuntu 24.04.
- `workflows/verify-pr.yml` must analyze aggregate statement coverage for `backend/db`, `backend/fileio`, `backend/process`, `backend/settings`, `backend/switchfs`, and `backend/switchfs/_crypto`, and fail when coverage is below 90%.
- For workflow edits, validate YAML structure and reason about missing artifact behavior (`if-no-files-found: error`) before merging.
