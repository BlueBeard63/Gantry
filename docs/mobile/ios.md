# iOS (experimental scaffold)

`gantry build --targets ios` generates an Xcode project scaffold into `.gantry/ios/` - the shell is real, but the Go server is **not linked in yet**, so what runs today is a placeholder page. The target exists so the shell, permissions and project wiring are ready when in-app Go linking lands; treat everything on this page as experimental.

The scaffold generates on any machine (it is just files - inspect it, commit an overlay against it, or hand it to a Mac), but turning it into a running app needs a Mac with Xcode.

```
gantry build --targets ios       # generates .gantry/ios/
cd .gantry/ios
xcodegen generate                # brew install xcodegen (one-time)
xed .                            # open in Xcode, pick a device, run
```

`gantry mobile dev ios` does the ground checks (a Mac, xcodebuild), generates the scaffold and prints those next steps - running on a device stays manual for now.

## What gets generated

| File | What it is |
| --- | --- |
| `project.yml` | [XcodeGen](https://github.com/yonaskolb/XcodeGen) spec - bundle id, versions, bridging header; `xcodegen generate` turns it into the `.xcodeproj` |
| `Info.plist` | bundle identity, versions derived from gantry.json, local-networking ATS exception, and the permission usage strings |
| `Sources/AppDelegate.swift` | the whole shell: a full-screen WKWebView pointed at `127.0.0.1:<port>` |
| `Sources/GantryShim.h/.c` | the seam where the Go server attaches - today a stub returning -1, which makes the shell show the placeholder page |
| `README.md` | these build steps, offline, next to the project |

The bundle identifier defaults to `mobile.id`; set `mobile.ios.bundleId` in gantry.json when the iOS identity must differ:

```json
"mobile": {
  "id": "ec.morrison.myapp",
  "ios": { "bundleId": "ec.morrison.myapp.ios" }
}
```

`CFBundleShortVersionString` comes from the top-level `version`, and `CFBundleVersion` from the same derived code Android uses (`major*10000 + minor*100 + patch`).

## Permissions

The same friendly names as Android, translated to Info.plist usage-description strings (Apple's purpose strings). Names with no iOS meaning are simply skipped - one `permissions` list builds both platforms.

| Name | Info.plist key |
| --- | --- |
| `camera` | NSCameraUsageDescription |
| `microphone` | NSMicrophoneUsageDescription |
| `location` | NSLocationWhenInUseUsageDescription |
| `location-background` | NSLocationAlwaysAndWhenInUseUsageDescription |
| `bluetooth` | NSBluetoothAlwaysUsageDescription |
| `contacts` | NSContactsUsageDescription |
| `notifications`, `files`, `vibrate`, `boot`, `network-state`, `foreground-service` | nothing to declare |

The generated strings are generic ("Myapp uses the camera."). App Store review usually wants more specific wording - override `Info.plist` via the overlay when you get there.

## Customizing

`.gantry/ios/` is regenerated every build - never edit it. Create `mobile/ios/` in your app root instead: its contents are copied over the generated scaffold path for path, exactly like `mobile/android/`.

## Why the Go server is missing

On Android the Go server ships inside the APK as an executable and runs as a child process. iOS forbids spawning processes, so the server has to be compiled *into* the app binary: `GOOS=ios GOARCH=arm64 CGO_ENABLED=1 go build -buildmode=c-archive`, exporting a start function the shim calls, plus the embedded frontend. That linking step is deferred; `Sources/GantryShim.h` documents the plan and `gantry_start()` is the function that will light up.
