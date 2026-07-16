//go:build linux && !nogui

package monitors

/*
#cgo pkg-config: gdk-3.0
#include <gdk/gdk.h>

// Ensures GDK is usable even when called before the webview brings the
// display up (e.g. computing placement first).
static GdkDisplay* gantry_display(void) {
	GdkDisplay *d = gdk_display_get_default();
	if (d == NULL) {
		if (!gdk_init_check(NULL, NULL)) {
			return NULL;
		}
		d = gdk_display_get_default();
	}
	return d;
}
*/
import "C"

import "fmt"

// All enumerates displays via GDK.
func All() []Monitor {
	display := C.gantry_display()
	if display == nil {
		return nil
	}
	n := int(C.gdk_display_get_n_monitors(display))
	out := make([]Monitor, 0, n)
	for i := 0; i < n; i++ {
		mon := C.gdk_display_get_monitor(display, C.int(i))
		if mon == nil {
			continue
		}
		var geo, work C.GdkRectangle
		C.gdk_monitor_get_geometry(mon, &geo)
		C.gdk_monitor_get_workarea(mon, &work)
		name := ""
		if model := C.gdk_monitor_get_model(mon); model != nil {
			name = C.GoString(model)
		}
		if name == "" {
			name = fmt.Sprintf("Display %d", i+1)
		}
		out = append(out, Monitor{
			Index:      i,
			Name:       name,
			X:          int(geo.x),
			Y:          int(geo.y),
			Width:      int(geo.width),
			Height:     int(geo.height),
			Primary:    C.gdk_monitor_is_primary(mon) != 0,
			WorkX:      int(work.x),
			WorkY:      int(work.y),
			WorkWidth:  int(work.width),
			WorkHeight: int(work.height),
		})
	}
	return out
}
