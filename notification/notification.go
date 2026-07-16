// Package notification posts system notifications from the app's Go
// code on mobile. On Android the Go server runs as a child of the
// shell, which owns the only path to NotificationManager - so this
// package speaks a one-line control protocol over stdout
// (GANTRY_NOTIFY / GANTRY_NOTIFY_CLEAR + JSON) that the shell parses
// and executes. On desktop these calls are logged no-ops: desktop
// notifications are app-drawn popup windows (see the notify package
// and the Notifications docs page).
//
//	notification.Post(notification.Notification{
//		ID:    "steep",
//		Title: "Tea's ready",
//		Body:  "The oolong steeped 4 minutes.",
//		Actions: []notification.Action{{ID: "again", Label: "Steep again"}},
//	})
//
// Posting again with the same ID updates the notification in place;
// Clear takes it down. Android only shows notifications when the app
// declared the "notifications" permission in gantry.json and the user
// accepted the prompt.
package notification

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
)

// Notification is one notification. ID is the stable handle: posting
// the same ID again replaces the old content in place (progress
// updates, ticking clocks), and Clear(id) removes it.
type Notification struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	// Channel groups notifications in Android's notification settings;
	// users can silence or block each channel on its own. Empty =
	// "default". ChannelName is the human label shown in settings,
	// defaulting to the app title.
	Channel     string `json:"channel,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
	// Silent posts to a low-importance channel: shown in the shade, no
	// sound or heads-up banner.
	Silent bool `json:"silent,omitempty"`
	// Actions are buttons on the notification; taps arrive at OnAction.
	Actions []Action `json:"actions,omitempty"`
}

// Action is one notification button.
type Action struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var (
	mu sync.Mutex
	// out and android are swapped by tests.
	out     io.Writer = os.Stdout
	android           = runtime.GOOS == "android"

	actionMu      sync.RWMutex
	actionHandler func(notificationID, actionID string)
)

// Post shows (or, for a repeated ID, updates) a notification.
func Post(n Notification) error {
	if n.ID == "" {
		return fmt.Errorf("notification needs an ID (it is the handle for updates and Clear)")
	}
	if n.Title == "" {
		return fmt.Errorf("notification %q needs a Title", n.ID)
	}
	emit("GANTRY_NOTIFY", n)
	return nil
}

// Clear removes the notification with this ID; unknown IDs are a no-op.
func Clear(id string) {
	emit("GANTRY_NOTIFY_CLEAR", struct {
		ID string `json:"id"`
	}{id})
}

// ClearAll removes every notification the app posted.
func ClearAll() {
	emit("GANTRY_NOTIFY_CLEAR", struct{}{})
}

// OnAction registers the handler for notification button taps. Taps
// wake the app's server if it is not running (Android starts the
// process to deliver the tap), so the handler also fires for
// notifications that outlived the app.
func OnAction(fn func(notificationID, actionID string)) {
	actionMu.Lock()
	actionHandler = fn
	actionMu.Unlock()
}

// Dispatch routes an action tap to the OnAction handler. It is called
// by the gantry runtime's /gantry/notify/action route (which the shell
// hits when a button is tapped); apps never call it.
func Dispatch(notificationID, actionID string) {
	actionMu.RLock()
	fn := actionHandler
	actionMu.RUnlock()
	if fn != nil {
		fn(notificationID, actionID)
	}
}

func emit(verb string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("notification: %v", err)
		return
	}
	if !android {
		log.Printf("notification (no-op off-device): %s %s", verb, data)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	fmt.Fprintf(out, "%s %s\n", verb, data)
}
