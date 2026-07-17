// Mobile-specific tests: the DOM plane and the native driver surface on
// a real device. These run only under `gantry test --device android` -
// MobileOnly skips them on desktop - and exercise real touch-driven
// element actions over the device WebView plus Back, Rotate,
// Notifications and permission pre-grants.
package tests

import (
	"strings"
	"testing"

	"github.com/B-Commissions/Gantry/gantrytest"
)

// The DOM plane transfers to the phone: the same Find/Click/Text code
// path drives the Android WebView over CDP.
func TestMobileDOMClick(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	page := app.Page("/")

	btn := page.Find("button", gantrytest.Text("count is"))
	if got := btn.Text(); !strings.Contains(got, "count is 0") {
		t.Errorf("counter starts at %q, want count is 0", got)
	}
	btn.Click()
	page.Find("button", gantrytest.Text("count is 1"))
	app.ExpectNoErrors()
}

// Back exercises the shell's WebView history behavior without dropping
// the app.
func TestMobileBack(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	app.Page("/settings") // navigate away from "/"
	app.Back()            // pops back to the previous route
	app.Find(".gantry-page")
	app.ExpectNoErrors()
}

// Rotate is a configuration change: the activity and WebView are
// recreated, but the socket must survive and the DOM stay drivable.
func TestMobileRotate(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t, gantrytest.WithDOM())
	app.Page("/")
	app.Rotate()
	// The page is still there and still queryable after the recreation.
	app.Page("/").Find("button", gantrytest.Text("count is"))
	app.ExpectNoErrors()
}

// A posted notification is inspectable through the driver.
func TestMobileNotifies(t *testing.T) {
	gantrytest.MobileOnly(t)
	app := gantrytest.Launch(t,
		gantrytest.WithDOM(),
		gantrytest.WithGrantedPermissions("android.permission.POST_NOTIFICATIONS"),
	)
	app.Page("/debug").Find("button", gantrytest.Text("Notify")).Click()
	app.WaitFor("the Saved notification to post", func() bool {
		return app.Notifications().Find("Saved") != nil
	})
	app.ExpectNoErrors()
}
