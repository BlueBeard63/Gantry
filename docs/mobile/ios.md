# iOS (experimental scaffold)

The iOS target is an **experimental scaffold**, not a working app yet. `gantry build --targets ios` generates an Xcode project into `.gantry/ios/` - the WKWebView shell is real, but the Go server is **not linked in yet**, so what runs today is a placeholder page. The target exists so the shell, permissions and project wiring are ready when in-app Go linking lands; treat everything on this page as provisional. Building and running on Android is fully working - see [Android builds](android.md).

Every generation prints the warning `the ios target is an experimental scaffold: the shell builds, the Go server is not linked in yet`, so there is no way to mistake it for a shipping path.

## Generate the scaffold

The scaffold generates on any machine (it is just files - inspect it, commit an overlay against it, or hand it to a Mac), but turning it into a running app needs a Mac with Xcode.

```
gantry build --targets ios       # generates .gantry/ios/ (arm64 only; bare "ios" = arm64)
cd .gantry/ios
xcodegen generate                # brew install xcodegen (one-time)
xed .                            # open in Xcode, pick a device, run
```

`gantry mobile dev ios` does the ground checks first - it needs a Mac (`runtime.GOOS == "darwin"`) and `xcodebuild` on PATH (install Xcode from the App Store, then `xcode-select --install`) - then generates the scaffold and prints those next steps. Running on a device stays manual for now; there is no automated install-and-launch the way there is for Android.

## What gets generated

Each file renders from `gantry.json`; the six outputs land under `.gantry/ios/`:

| File | What it is |
| --- | --- |
| `project.yml` | [XcodeGen](https://github.com/yonaskolb/XcodeGen) spec - deployment target iOS 15.0, bundle id, versions, the bridging header, `TARGETED_DEVICE_FAMILY "1,2"` (iPhone + iPad); `xcodegen generate` turns it into `<name>.xcodeproj` |
| `Info.plist` | bundle identity, versions from gantry.json, an `NSAllowsLocalNetworking` ATS exception (the app talks plain http to `127.0.0.1`), portrait + landscape orientations, and the permission usage strings |
| `Sources/AppDelegate.swift` | the whole shell - a full-screen `WKWebView` (`AppDelegate` + `ShellViewController`) that loads `http://127.0.0.1:<port>/` when `gantry_start()` returns a port, or an inline placeholder page when it returns -1 |
| `Sources/GantryShim.h` | the bridging header declaring `int gantry_start(void)` - the seam where the Go server attaches |
| `Sources/GantryShim.c` | today a stub returning -1, which makes the shell show the placeholder page |
| `README.md` | these build steps and the linking plan, offline, next to the project |

## Configuration

The bundle identifier defaults to `mobile.id`; set `mobile.ios.bundleId` in gantry.json when the iOS identity must differ:

```json
"mobile": {
  "id": "ec.morrison.myapp",
  "ios": { "bundleId": "ec.morrison.myapp.ios" }
}
```

In `project.yml` this becomes `PRODUCT_BUNDLE_IDENTIFIER`. `MARKETING_VERSION` (`CFBundleShortVersionString`) comes from the top-level `version`, and `CURRENT_PROJECT_VERSION` (`CFBundleVersion`) from the same derived integer Android uses (`major*10000 + minor*100 + patch`), so the two platforms stay in lockstep.

## Permissions

The same friendly names as [Android](android.md#permissions), translated to Info.plist usage-description strings (Apple's purpose strings). One `permissions` list builds both platforms - names with no iOS meaning are simply skipped (not an error).

| Name | Info.plist key |
| --- | --- |
| `camera` | NSCameraUsageDescription |
| `microphone` | NSMicrophoneUsageDescription |
| `location` | NSLocationWhenInUseUsageDescription |
| `location-background` | NSLocationAlwaysAndWhenInUseUsageDescription |
| `bluetooth` | NSBluetoothAlwaysUsageDescription |
| `contacts` | NSContactsUsageDescription |
| `notifications`, `files`, `vibrate`, `boot`, `network-state`, `foreground-service` | nothing to declare |

The generated strings are generic and fill in your app title - e.g. `camera` becomes "Myapp uses the camera." App Store review usually wants more specific wording; override `Info.plist` via the overlay (below) when you get there.

## Notes (advanced)

### Customizing

`.gantry/ios/` is regenerated every build - never edit it. Create `mobile/ios/` in your app root instead: its contents are copied over the generated scaffold path for path after each synthesis, exactly like `mobile/android/`. That is how you replace `Info.plist` with better usage strings or add custom Swift.

### Why the Go server is missing

On Android the Go server ships inside the APK as an executable and runs as a child process. iOS forbids spawning processes, so the server has to be compiled *into* the app binary. The plan, documented in the generated `Sources/GantryShim.h` and `README.md`:

1. compile the app's Go module with `GOOS=ios GOARCH=arm64 CGO_ENABLED=1 go build -buildmode=c-archive -o libgantryapp.a .`, exporting a cgo function that starts the server on an ephemeral `127.0.0.1` port and returns it,
2. link the archive into the Xcode project and call it from `gantry_start()` in `Sources/GantryShim.c` (the seam is already there - today it returns -1),
3. embed the built frontend the same way the desktop builds do.

Until that lands, `gantry_start()` returns -1 and `ShellViewController` shows the placeholder page. The scaffold exists so the shell, permissions and project wiring are ready the day the linking step is done.
