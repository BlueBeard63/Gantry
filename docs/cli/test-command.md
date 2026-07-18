# The test command

`gantry test` runs the app's end-to-end tests and writes an HTML report. This page is the flag reference; for how to author tests with the `gantrytest` driver see [Setup](../testing/setup.md), and for the report viewer, retries and flakiness see [the report](../testing/report.md). Flags parse with Go's standard `flag` package, so each accepts both `-flag` and `--flag`, and a value can be written `--mode production` or `--mode=production`; boolean flags take no value.

## gantry test

Runs the app's end-to-end tests: Go tests under `tests/` using the `gantrytest` driver, against the real app process. It prepares the app exactly like a build (regenerated `.gantry/`, registries, one `vite build` so the served frontend is current), prebuilds the app binary once for the whole suite (shared with every test via `GANTRY_TEST_BIN`), then wraps `go test ./tests/...` (with `-count=1`, so runs are never cached). The optional `pattern` argument filters test names and is passed through to `go test -run`. Every run writes a self-contained `gantry_test_report.html` under `test-results/`. A missing `tests/` directory is an error with a hint to create `tests/<name>_test.go` using the `gantrytest` package.

```
gantry test
gantry test Counter        # only tests whose name matches "Counter"
gantry test --headed --record
gantry test --show         # just open the latest report
```

Flags:

- `--headed` (bool, default: false) - run apps with the real window instead of headless.
- `--record` (bool, default: false) - record a `screencast.avi` artifact for every DOM-plane test (implies keeping those artifacts).
- `--keep-artifacts` (bool, default: false) - keep passing tests' artifacts too (under `test-results/`); failing tests always keep theirs.
- `--mode M` (string, default: **development**) - app mode for the suite, `development` or `production`; any other value errors.
- `--device D` (string, default: "") - run the suite on a device instead of the desktop: `android` (the sole connected device/emulator) or `android:SERIAL` for a specific one. See Notes below and [Android builds](../mobile/android.md).
- `--allow-device-data` (bool, default: false) - allow the hermetic `pm clear` (which wipes the test app's on-device data) on a physical device; emulators always allow it. Without consent the suite still runs, just without the wipe.
- `-p N` (int, default: **NumCPU/2**, floored at 1) - test parallelism; each parallel test is a full app process. Forced to 1 for `--device` runs.
- `-v` (bool, default: false) - verbose `go test` output.
- `--update` (bool, default: false) - update golden files (widget snapshots) instead of comparing.
- `--retries N` (int, default: **0**) - re-run each failed test up to N times; a test that then passes is reported flaky rather than failed. Each retry runs one test per invocation so its artifacts stay isolated and the `-run` pattern is unambiguous.
- `--timeout D` (duration, default: **10m**) - the overall `go test` timeout, e.g. `--timeout 5m`.
- `--show` (bool, default: false) - open the most recent `gantry_test_report.html` instead of running anything (a viewer shortcut; `pattern` then selects among reports).
- `--open` (bool, default: false) - open the report in the browser when the run finishes.

The command exits non-zero when any test fails, printing a one-line tally (passed / flaky / failed / skipped) and the report path.

### Notes

**`--device`.** A device run builds and installs a debug APK under `<mobile.id>.test` (an `applicationIdSuffix` of `.test`, so it sits beside any real install and uninstalling it touches nothing else), forces `-p` to 1 (one app instance per device), and needs a `mobile` section with an `id` in `gantry.json`. `android:SERIAL` targets a specific device; bare `android` uses the sole connected device/emulator. `--allow-device-data` permits the hermetic `pm clear` on a physical device (emulators always allow it); `--timeout` bounds the whole `go test` (default 10m); `--open` opens the report when the run finishes. The test app is uninstalled again when the suite ends.
