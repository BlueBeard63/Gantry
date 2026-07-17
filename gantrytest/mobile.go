package gantrytest

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Mobile-specific driver surface: native input and lifecycle (Back,
// Rotate), posted-notification inspection (Notifications), and runtime
// permission grants. These drive Android concerns the desktop backend
// has no analogue for, so each is device-only and fails cleanly with
// MobileOnly guidance when called against a desktop target.

// android returns the adb-backed proc for a device launch, failing the
// test with the fix when this is a desktop target.
func (a *App) android() *androidProc {
	a.t.Helper()
	p, ok := a.proc.(*androidProc)
	if !ok {
		a.t.Fatalf("gantrytest: this is a mobile-only helper - run against a device (gantry test --device android); target is %s", Target())
	}
	return p
}

// Back presses the system back button (KEYCODE_BACK), exercising the
// shell's WebView history / exit behavior (MainActivity.onBackPressed).
func (a *App) Back() {
	a.t.Helper()
	p := a.android()
	a.art.trace.action("back")
	if _, err := p.b.adbOut("shell", "input", "keyevent", "KEYCODE_BACK"); err != nil {
		a.t.Fatalf("gantrytest: pressing back: %v", err)
	}
	a.snapRecording()
}

// Rotate forces a configuration change (portrait <-> landscape). The
// activity and WebView are recreated while the Go backend - owned by the
// Application - keeps running, so the protocol connection survives; the
// point of the test is that the socket is not dropped. The CDP page
// target is replaced by the recreation, so the DOM plane is re-attached
// (the process, hence the forwarded devtools socket, is unchanged).
func (a *App) Rotate() {
	a.t.Helper()
	p := a.android()
	a.art.trace.action("rotate")
	// Take manual control of rotation, then toggle it.
	if _, err := p.b.adbOut("shell", "settings", "put", "system", "accelerometer_rotation", "0"); err != nil {
		a.t.Fatalf("gantrytest: disabling auto-rotate: %v", err)
	}
	cur, _ := p.b.adbOut("shell", "settings", "get", "system", "user_rotation")
	next := "1"
	if strings.TrimSpace(cur) == "1" {
		next = "0"
	}
	if _, err := p.b.adbOut("shell", "settings", "put", "system", "user_rotation", next); err != nil {
		a.t.Fatalf("gantrytest: rotating: %v", err)
	}
	if a.cdp != nil {
		a.cdp.close()
		a.cdp = nil
		a.attachDOM()
	}
	a.snapRecording()
}

// Grant grants an Android runtime permission to the app mid-test (the
// pre-launch form is WithGrantedPermissions).
func (a *App) Grant(permission string) {
	a.t.Helper()
	p := a.android()
	a.art.trace.action("grant %s", permission)
	if _, err := p.b.adbOut("shell", "pm", "grant", p.b.appID, permission); err != nil {
		a.t.Fatalf("gantrytest: granting %s: %v", permission, err)
	}
}

// Revoke revokes an Android runtime permission from the app mid-test.
func (a *App) Revoke(permission string) {
	a.t.Helper()
	p := a.android()
	a.art.trace.action("revoke %s", permission)
	if _, err := p.b.adbOut("shell", "pm", "revoke", p.b.appID, permission); err != nil {
		a.t.Fatalf("gantrytest: revoking %s: %v", permission, err)
	}
}

// Notification is one posted system notification belonging to the app.
type Notification struct {
	Title   string
	Text    string
	Actions []string
}

// NotificationSet is the app's currently-posted notifications.
type NotificationSet struct {
	items []Notification
}

// Notifications reads the notifications the app has currently posted
// (via dumpsys). Android only surfaces these when the app holds
// POST_NOTIFICATIONS.
func (a *App) Notifications() *NotificationSet {
	a.t.Helper()
	p := a.android()
	out, err := exec.Command(p.b.adb, "-s", p.b.serial, "shell", "dumpsys", "notification", "--noredact").Output()
	if err != nil {
		a.t.Fatalf("gantrytest: reading notifications: %v", err)
	}
	return &NotificationSet{items: parseNotifications(string(out), p.b.appID)}
}

// All returns every posted notification belonging to the app.
func (ns *NotificationSet) All() []Notification { return ns.items }

// Find returns the first posted notification whose title or text
// contains substr, or nil if none does.
func (ns *NotificationSet) Find(substr string) *Notification {
	for i := range ns.items {
		if strings.Contains(ns.items[i].Title, substr) || strings.Contains(ns.items[i].Text, substr) {
			return &ns.items[i]
		}
	}
	return nil
}

var (
	notifRecordRe = regexp.MustCompile(`NotificationRecord\(.*?pkg=(\S+)`)
	notifTitleRe  = regexp.MustCompile(`android\.title=(?:String \()?([^\r\n)]*)`)
	notifTextRe   = regexp.MustCompile(`android\.text=(?:String \()?([^\r\n)]*)`)
	notifActionRe = regexp.MustCompile(`title=([^\r\n,]*?)(?:,|$)`)
)

// parseNotifications extracts the app's posted notifications from a
// `dumpsys notification --noredact` dump. The format shifts across
// Android versions, so it matches on the package plus the title/text
// extras rather than fixed offsets, and stays tolerant of misses.
func parseNotifications(dump, appID string) []Notification {
	var out []Notification
	// Split into per-record blocks so extras stay scoped to their record.
	locs := notifRecordRe.FindAllStringSubmatchIndex(dump, -1)
	for i, loc := range locs {
		pkg := dump[loc[2]:loc[3]]
		if pkg != appID {
			continue
		}
		end := len(dump)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		block := dump[loc[0]:end]
		n := Notification{
			Title: firstGroup(notifTitleRe, block),
			Text:  firstGroup(notifTextRe, block),
		}
		if idx := strings.Index(block, "actions={"); idx >= 0 {
			for _, m := range notifActionRe.FindAllStringSubmatch(block[idx:], -1) {
				if label := strings.TrimSpace(m[1]); label != "" && label != "null" {
					n.Actions = append(n.Actions, label)
				}
			}
		}
		out = append(out, n)
	}
	return out
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(strings.TrimSuffix(m[1], ")"))
	}
	return ""
}

// TapAction taps a notification action button by the notification id and
// action id (the ids the app posted them with), relayed to the app's
// action receiver via a broadcast - the same path a real tap takes.
func (a *App) TapAction(notificationID, actionID string) {
	a.t.Helper()
	p := a.android()
	a.art.trace.action("tap notification action %s/%s", notificationID, actionID)
	action := fmt.Sprintf("%s.NOTIFY_ACTION", strings.TrimSuffix(p.b.appID, ".test"))
	if _, err := p.b.adbOut("shell", "am", "broadcast",
		"-a", action,
		"--es", "notification", notificationID,
		"--es", "action", actionID,
	); err != nil {
		a.t.Fatalf("gantrytest: tapping notification action %s/%s: %v", notificationID, actionID, err)
	}
}
