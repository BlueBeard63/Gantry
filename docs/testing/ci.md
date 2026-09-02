# Testing: CI

The protocol-plane suite is deliberately CI-friendly: headless, no display server, no browser tooling, ephemeral ports. Anywhere Go and Node run, `gantry test` runs. This page covers wiring it into a pipeline; the flags it leans on are documented in [setup](setup.md#the-gantry-test-command).

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
      - run: go run github.com/BlueBeard63/Gantry/cmd/gantry@latest test -v
      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: test-results
          path: test-results/
```

The pieces that matter:

- **Upload `test-results/` on failure.** Failing tests keep `app.log` and `trace.jsonl` (and `crash.log` when the process died), and the whole run rolls up into the self-contained `gantry_test_report.html` at the root of that directory - open it locally to get the Run Overview and per-test Trace without reproducing anything. See [errors and artifacts](errors-and-artifacts.md#artifacts).
- **Caching.** The Go build cache and the npm cache cover the expensive steps, and the app binary builds once per suite (not per test), so warm runs are dominated by the tests themselves.
- **Parallelism.** `gantry test` defaults to NumCPU/2 because each parallel test is a full app process; on small runners `-p 2` is a sensible floor, and `-p 1` serializes when a runner is memory-constrained.
- **Headless is real headless** for the protocol plane - the app runs with `--no-open` and serves without a window, so Linux runners need no xvfb for these tests.

## Recommended split

- **PR gate**: the protocol-plane suite plus [widget snapshots](widgets.md) - fast, hermetic, no special runners, cross-platform.
- **Nightly**: `--mode production` runs (asserting production error behavior), the DOM suite, `--record` screencasts, and the [Android emulator job](#the-android-emulator-job) below.

The DOM plane belongs in the nightly tier because it needs a CDP-speaking webview: a Windows runner (WebView2) is the validated path, and a Linux runner (WebKitGTK under `xvfb`) is [experimental](dom.md#platforms). Protocol-plane tests share the same files and simply run everywhere - a `WithDOM()` launch on a runner without a webview skips cleanly, so a mixed suite never fails for being on the wrong OS.

## Production-mode runs

```
gantry test --mode production
```

Development mode is the default so error detail is full; a periodic production run catches anything gated on mode - stripped stacks, disabled pages, production-only branches. Tests that assert on mode-dependent behavior can also pin it per launch with `WithMode`, so a single suite can carry both.

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
          script: go run github.com/BlueBeard63/Gantry/cmd/gantry@latest test --device android -v
      - uses: actions/upload-artifact@v4
        if: failure()
        with:
          name: android-test-results
          path: examples/**/test-results/
```

The action boots the AVD, waits for it, and runs the script against it as the sole connected device. The emulator counts as a device that allows the hermetic `pm clear`, so runs are fully isolated without `--allow-device-data`; the runner builds and installs the `.test` APK for the emulator's `x86_64` ABI and uninstalls it when the suite ends. Parallelism is forced to one on a device target regardless of `-p`. Cache the Go build cache, the npm cache, and the AVD system image (the action does the last) to keep it cheap; the app binary builds once per suite. See [mobile testing](mobile.md) for the device story in full.
