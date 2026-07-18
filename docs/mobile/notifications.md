# Notifications (mobile)

On the phone your app posts real system notifications - the kind that land in the shade, survive the app being closed, and respect the user's per-channel settings. The API is the `notification` package; the generated shell does the Android side for you. This page is only about mobile system notifications - desktop notifications are a different thing (app-drawn popup windows) documented in [Shell > Notifications](../shell/notifications.md).

## Prerequisite: the permission

Android 13+ only shows notifications when the user granted `POST_NOTIFICATIONS`. Declare the `notifications` permission in `gantry.json` and the shell prompts on first launch:

```json
"mobile": {
  "id": "ec.morrison.myapp",
  "permissions": ["notifications"]
}
```

Without the grant, Android drops the notification but `Post` still returns `nil` - your code does not error. See [Android > Permissions](android.md#permissions) for the full list.

## Sending

```go
import "github.com/B-Commissions/Gantry/notification"

err := notification.Post(notification.Notification{
	ID:    "steep",
	Title: "Tea's ready",
	Body:  "The oolong steeped 4 minutes.",
})
```

`Post` validates two fields and returns an error rather than posting when either is empty: `ID` (the stable handle for updates and `Clear`) and `Title`. `Body` is optional and long bodies expand (big-text style) when the user pulls the notification open. Everything else is optional.

The whole `Notification` struct:

| Field | Type | Meaning |
| --- | --- | --- |
| `ID` | string | required; the stable handle - repost to update, `Clear` to remove |
| `Title` | string | required; the bold first line |
| `Body` | string | optional; expands as big-text when opened |
| `Channel` | string | groups notifications for the user's settings; empty = `default` |
| `ChannelName` | string | the channel's label in Android settings; empty = the app title |
| `Silent` | bool | post on a low-importance channel (no sound/heads-up) |
| `Actions` | `[]Action` | buttons; taps arrive at `OnAction` |

## Updating

Post the same `ID` again - the notification updates in place instead of stacking a second one (the shell notifies with the string ID as the tag). That is the whole mechanism: tick a countdown, bump a progress line, flip "Downloading" to "Done".

```go
notification.Post(notification.Notification{ID: "steep", Title: "Steeping...", Body: "2 minutes left"})
// later
notification.Post(notification.Notification{ID: "steep", Title: "Tea's ready"})
```

## Clearing

```go
notification.Clear("steep")   // take one down (unknown IDs are a no-op)
notification.ClearAll()       // take down everything the app posted
```

Users can always swipe notifications away themselves, and tapping the notification body opens the app and dismisses it (auto-cancel).

## Actions (buttons)

Attach `Action`s (each an `ID` and a `Label`); taps arrive at your `OnAction` handler. Android displays up to three action buttons on a notification.

```go
notification.Post(notification.Notification{
	ID:    "steep",
	Title: "Tea's ready",
	Actions: []notification.Action{
		{ID: "again", Label: "Steep again"},
		{ID: "done", Label: "Done"},
	},
})

notification.OnAction(func(notificationID, actionID string) {
	if notificationID == "steep" && actionID == "again" {
		startSteep()
	}
	notification.Clear(notificationID)
})
```

Actions work even after the app died: Android starts the app process to deliver the tap, the shell's receiver waits for your server's ready handshake and then relays the tap to `/gantry/notify/action`, so `OnAction` fires either way. Register `OnAction` once at startup (last registration wins). Two things an action tap does **not** do: it does not bring the app to the foreground (tap the notification *body* for that), and it does not auto-dismiss the notification - `Clear` it yourself when the action is handled.

## Channels and names

Every notification lives on a channel; Android's per-app notification settings let the user silence or block each channel separately. Default: a channel called `default`, named after your app title. Group things the user might want to control apart:

```go
notification.Post(notification.Notification{
	ID:          "sync-done",
	Channel:     "sync",
	ChannelName: "Sync results",   // the label in Android's settings UI
	Title:       "Backup finished",
	Silent:      true,
})
```

`Silent: true` posts on `IMPORTANCE_LOW` - visible in the shade, but no sound, vibration or heads-up banner (a normal notification uses `IMPORTANCE_DEFAULT`). A channel's importance is fixed the first time it is created on-device (Android's rule), so pick silent-vs-not per channel, not per message: a later post on the same channel keeps the importance the channel was born with.

## Icons

Notifications use the app's launcher icon (`icons/icon.png`, see [Android > Icons](android.md#icons)) as their small icon. Android renders small icons as a monochrome silhouette, so a detailed logo can look like a blob in the status bar - a simple, high-contrast glyph fares best. A dedicated notification icon file is on the roadmap.

## Notes (advanced)

### How it works, and its limits

The Go server can't touch `NotificationManager` - it is a child process without the Android framework. `Post`/`Clear`/`ClearAll` marshal the payload and write one-line control messages to stdout (`GANTRY_NOTIFY {json}`, `GANTRY_NOTIFY_CLEAR {json}`, and an empty-object clear for `ClearAll`), and the shell that spawned the server parses and executes them against `NotificationManager`. Two consequences:

- Notifications post only while your app's process is alive (the server runs as long as the app does). For truly scheduled/background notifications, pair a widget-style WorkManager trigger - planned, not built yet.
- On desktop (`GOOS != "android"`) these calls are no-ops that log what they would have posted, so shared code runs everywhere; desktop attention-getting is the [popup notification system](../shell/notifications.md).

Everything the server prints - including these control lines - is visible in `adb logcat -s gantry-go`, which makes notification flows easy to debug from `gantry mobile dev android`.
