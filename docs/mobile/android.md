# Android builds

`gantry build --targets android` packs your app into an installable APK: the Go server cross-compiles for the phone and runs on-device as a child process, and a thin generated Kotlin shell points a full-screen WebView at it on `127.0.0.1`. No gomobile, no Java in your project - the whole Android side is synthesized into `.gantry/android/` on every build, the same way the Vite root is.

```
gantry build --targets android          # arm64 (bare "android" defaults to arm64)
gantry build --targets android/arm64,android/amd64   # multi-ABI APK (amd64 = emulators)
adb install -r dist/android/myapp-0.1.0.apk
```

## The dev loop

```
gantry mobile dev android
```

The phone equivalent of `gantry dev`: it checks the toolchain (and **offers to install** what's missing - the SDK command-line tools, platform-tools/adb, the NDK; a JDK 17+ is the one thing you must bring), finds your phone, builds the APK for that phone's ABI, installs it, launches it and streams the app's logcat (`gantry-go`) into your terminal until Ctrl+C.

The phone must be **plugged in over USB** with USB debugging enabled (Settings > Developer options) - adb-over-wifi is deliberately not supported; the cable is what keeps logcat reliable. Local emulators count as plugged in. With several devices connected, unplug the extras or set `ANDROID_SERIAL`.

`gantry mobile dev ios` exists but only checks the ground (a mac with Xcode) - the ios target is an experimental scaffold.

## What the machine needs

| Piece | Where gantry looks | Fix when missing |
| --- | --- | --- |
| Android SDK | `ANDROID_HOME`, `ANDROID_SDK_ROOT`, then the Android Studio default (`%LOCALAPPDATA%\Android\Sdk`, `~/Library/Android/sdk`, `~/Android/Sdk`) | install Android Studio or the cmdline-tools |
| Android NDK | `ANDROID_NDK_HOME`, else the newest under `<sdk>/ndk/` | `sdkmanager "ndk;27.2.12479018"` |
| JDK 17+ | `JAVA_HOME`, else `java` on PATH | install any JDK 17+ |

Gradle itself is **not** on the list: the generated project ships the Gradle wrapper, which downloads Gradle on the first build.

A missing piece is never a hard failure - the android target prints a warning naming the exact fix and is skipped, while the other targets in the same run still build. Misconfiguration (a bad `mobile.id`, an unknown permission name) *is* a hard failure: that is yours to fix.

## The mobile section of gantry.json

```json
"mobile": {
  "id": "ec.morrison.myapp",             // required: reverse-DNS app id - changing it after release makes stores treat it as a different app
  "permissions": ["notifications", "camera"],
  "android": {
    "minSdk": 26,                        // default 26 (Android 8)
    "targetSdk": 35,                     // default 35 (Android 15)
    "keystore": {                        // omit = debug-signed (installable, not store-ready)
      "file": "release.jks",             // relative to the app root
      "alias": "myapp",
      "passwordEnv": "MYAPP_KEY_PASS"    // env var read at build time - the password never lives in the repo
    }
  }
}
```

The Android `versionCode` derives from the top-level `version`: `major*10000 + minor*100 + patch`, so every semver bump yields a strictly larger code.

## Permissions

Friendly names, mapped to real Android permissions at build time. INTERNET is always included (the app talks to its own local server). "Dangerous" permissions are also requested with a one-shot runtime prompt on first launch.

| Name | Android permission(s) | Runtime prompt |
| --- | --- | --- |
| `camera` | CAMERA | yes |
| `microphone` | RECORD_AUDIO | yes |
| `location` | ACCESS_FINE_LOCATION, ACCESS_COARSE_LOCATION | yes |
| `location-background` | ACCESS_BACKGROUND_LOCATION | yes |
| `notifications` | POST_NOTIFICATIONS | yes |
| `files` | READ/WRITE_EXTERNAL_STORAGE | yes |
| `bluetooth` | BLUETOOTH_CONNECT, BLUETOOTH_SCAN | yes |
| `contacts` | READ_CONTACTS | yes |
| `vibrate` | VIBRATE | no |
| `boot` | RECEIVE_BOOT_COMPLETED | no |
| `network-state` | ACCESS_NETWORK_STATE | no |
| `foreground-service` | FOREGROUND_SERVICE | no |

An unknown name is a build error listing the whole vocabulary. Web-side `getUserMedia` calls inside the WebView are granted automatically when the app already holds the matching Android permission.

## How the shell behaves

- The Go server starts with `--port 0` (ephemeral) and a random per-launch token; only the shell's own WebView can talk to it - other apps on the phone hitting the loopback port get 403.
- The window is edge-to-edge: the WebView canvas is padded inside the status/navigation bars (and above the keyboard), and the inset strips are painted with the page's own background colour, so the app *looks* like it runs under the bars without content ever hiding there. System bar icons flip dark/light to match.
- The server's stdout streams to logcat: `adb logcat -s gantry-go`.
- If the server dies it is restarted with backoff; the WebView re-points itself at the new port.
- Back gesture/button walks WebView history before leaving the app.

Home-screen widgets and system notifications each have their own page: [Widgets](widgets.md), [Notifications](notifications.md).

## Customizing the Android project

`.gantry/android/` is regenerated every build - never edit it. Instead, create `mobile/android/` in your app root: after synthesis its contents are copied over the generated tree, path for path. `mobile/android/app/src/main/res/values/strings.xml` replaces the generated one, extra Kotlin files land in the source set, and so on.

## Icons

The launcher icon renders from `icons/icon.png` at every density (48-192px). No icons directory = the same placeholder glyph the desktop surfaces use.
