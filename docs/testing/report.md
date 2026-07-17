# Testing: the report

Every `gantry test` run writes a self-contained `gantry_test_report.html` into `test-results/` - a single file you open by double-click, e-mail, or upload as one CI artifact. It carries the whole run inlined: results, per-test traces, logs, screenshots, and the screencast, with no server and no network.

```
gantry test --record        # run, then find the report path printed at the end
gantry test --show          # open the most recent report in your browser
gantry test --show TestX    # open it deep-linked to one test's detail
gantry test --record --open # run and open the report when it finishes
```

## Run Overview

The landing page is every test grouped by source file, with a status tally (passed / failed / flaky / skipped) and a filter box. The filter understands a small DSL:

- free text matches the test name and file (`login`, `settings`),
- `s:failed` / `s:flaky` / `s:passed` / `s:skipped` filter by status,
- `a:screencast` / `a:trace` / `a:crash` match on an artifact the test produced.

Each row shows the test's plane (`dom`, `native`, or `device`), its artifacts, its duration, and - for a flaky test - which retry it finally passed on. Click a row for the detail view.

## Test Detail

A failed or flaky test opens on its final attempt with:

- the **assertion** that failed - the driver's expect/want/got and a real stack when the driver raised it, or the go-test output otherwise;
- a **Screencast** player (with `--record`): a scrubber, frame-stepping, speed, and a marker on the failure frame;
- a **Screenshots** gallery: `failure.png` (captured automatically on any DOM-plane failure) plus every `app.Screenshot("name")` / `el.Screenshot("name")` still;
- a **Trace**: a lane timeline (actions / received / sent frames) with a draggable playhead over a filterable table of every protocol frame and driver action - expand a row for its raw JSON or the rendered Tea tree;
- the **Logs**: `app.log`, `console.log`, and `crash.log` when present;
- an **Artifacts** sidebar with every file, its size, and the output directory.

When more than one attempt ran (see retries), a switcher moves between them.

## Retries and flaky tests

Go's test runner has no retry concept; `gantry test --retries N` adds one:

```
gantry test --retries 2
```

The suite runs once; every test that failed is then re-run on its own, up to `N` more times or until it passes. A test that fails and then passes is reported **flaky** (and shown as "passed on retry k"); one that fails every attempt is a **failure**. Each attempt keeps its own artifacts under `test-results/<TestName>/attempt-k/`, so the report can show what happened on every try - the flaky screencast next to the passing one.

Retries are opt-in: without `--retries`, a run behaves exactly as before, with flat per-test artifact directories.

## What ends up in the report

The generator inlines the kept artifacts, so the report is portable. Passing tests that keep nothing (the default) still appear in the overview - the driver records each launch's plane, timing, and worker into `test-results/.gantry-run/` before the artifact directory is cleaned. Recorded screencasts are inlined as a downsampled set of distinct frames (far smaller than the padded video), and very large logs and traces are truncated in the file with the full copy left on disk.
