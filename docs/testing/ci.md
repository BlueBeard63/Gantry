# Testing: CI

The protocol-plane suite is deliberately CI-friendly: headless, no display server, no browser tooling, ephemeral ports. Anywhere Go and Node run, `gantry test` runs.

## A minimal GitHub Actions job

```yaml
jobs:
  e2e:
    runs-on: windows-latest   # or ubuntu-latest - the protocol plane is cross-platform
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - uses: actions/setup-node@v4
        with: { node-version: 22, cache: npm }
      - run: npm ci
      - run: go run github.com/B-Commissions/Gantry/cmd/gantry@latest test -v
      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: test-results
          path: test-results/
```

The pieces that matter:

- **Upload `test-results/` on failure.** Failing tests keep `app.log` and `trace.jsonl` (and `crash.log` when the process died); that is usually enough to diagnose without reproducing locally.
- **Caching**: the Go build cache and the npm cache cover the expensive steps. The app builds once per suite, not per test.
- **Parallelism**: `gantry test` defaults to NumCPU/2 because each parallel test is a full app process; on small runners `-p 2` is a sensible floor.
- **Headless is real headless** for the protocol plane - `--no-open` serves without a window, so Linux runners need no xvfb for these tests.

## Recommended split

- **PR gate**: the protocol-plane suite plus [widget snapshots](widgets.md) - fast, hermetic, no special runners.
- **Nightly**: `--mode production` runs (asserting production error behavior), the DOM suite, `--record` screencasts, and the [Android emulator job](#the-android-emulator-job) below.

## The Android emulator job

`gantry test --device android` runs the whole suite - protocol plane, DOM plane, and the mobile-specific helpers - against an emulator. Emulators support everything a phone does except real sensors, and boot headless on a KVM-enabled Linux runner:

```yaml
jobs:
  android-e2e:
    runs-on: ubuntu-latest   # needs KVM for a fast emulator
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version-file: go.mod, cache: true }
      - uses: actions/setup-node@v4
        with: { node-version: 22, cache: npm }
      - run: npm ci
      # KVM permissions for the hardware-accelerated emulator.
      - run: |
          echo 'KERNEL=="kvm", GROUP="kvm", MODE="0666", OPTIONS+="static_node=kvm"' | sudo tee /etc/udev/rules.d/99-kvm4all.rules
          sudo udevadm control --reload-rules && sudo udevadm trigger --name-match=kvm
      - uses: reactivecircus/android-emulator-runner@v2
        with:
          api-level: 34
          arch: x86_64
          force-avd-creation: false
          emulator-options: -no-window -no-audio -no-boot-anim -gpu swiftshader_indirect
          script: go run github.com/B-Commissions/Gantry/cmd/gantry@latest test --device android -v
      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: android-test-results
          path: examples/**/test-results/
```

The action boots the AVD, waits for it, and runs the script against it as the sole connected device. The emulator counts as a device that allows the hermetic `pm clear`, so runs are fully isolated without `--allow-device-data`. Cache the Go build cache, the npm cache, and the AVD system image (the action does the last) to keep it cheap; the app builds once per suite.

## Production-mode runs

```
gantry test --mode production
```

Development mode is the default so error detail is full; a periodic production run catches anything gated on mode - stripped stacks, disabled pages, production-only branches. Tests that assert on mode-dependent behavior can also pin it per launch with `WithMode`.
