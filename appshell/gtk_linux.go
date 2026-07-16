//go:build linux && !nogui

package appshell

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.0
#include <gtk/gtk.h>
#include <gdk/gdk.h>
#include <webkit2/webkit2.h>
#include <stdlib.h>

// The WebKit render process can die (GPU trouble, OOM, a WebKit bug) -
// without handling, the window stays as a dead white surface. Reload
// shortly after the crash so the app recovers by itself.
static gboolean gantry_reload_cb(gpointer view) {
	if (WEBKIT_IS_WEB_VIEW(view)) {
		webkit_web_view_reload(WEBKIT_WEB_VIEW(view));
	}
	return FALSE;
}

static void gantry_on_web_crash(WebKitWebView *view,
		WebKitWebProcessTerminationReason reason, gpointer data) {
	g_warning("gantry: web process terminated (reason %d), reloading", reason);
	g_timeout_add(300, gantry_reload_cb, view);
}

static void gantry_watch_webprocess(GtkWindow *w) {
	GtkWidget *child = gtk_bin_get_child(GTK_BIN(w));
	if (child != NULL && WEBKIT_IS_WEB_VIEW(child)) {
		g_signal_connect(child, "web-process-terminated",
			G_CALLBACK(gantry_on_web_crash), NULL);
	}
}

// Starts the compositor's interactive move using the current pointer -
// the Linux equivalent of the Windows WM_NCLBUTTONDOWN hand-off.
static void gantry_begin_move_drag(GtkWindow *w) {
	GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(w));
	GdkSeat *seat = gdk_display_get_default_seat(display);
	GdkDevice *pointer = gdk_seat_get_pointer(seat);
	gint x = 0, y = 0;
	gdk_device_get_position(pointer, NULL, &x, &y);
	gtk_window_begin_move_drag(w, 1, x, y, gtk_get_current_event_time());
}

// Starts an interactive resize from the given edge (GdkWindowEdge).
static void gantry_begin_resize_drag(GtkWindow *w, int edge) {
	GdkDisplay *display = gtk_widget_get_display(GTK_WIDGET(w));
	GdkSeat *seat = gdk_display_get_default_seat(display);
	GdkDevice *pointer = gdk_seat_get_pointer(seat);
	gint x = 0, y = 0;
	gdk_device_get_position(pointer, NULL, &x, &y);
	gtk_window_begin_resize_drag(w, (GdkWindowEdge)edge, 1, x, y, gtk_get_current_event_time());
}

static void gantry_set_min_max(GtkWindow *w, int minW, int minH, int maxW, int maxH) {
	GdkGeometry geom;
	GdkWindowHints hints = (GdkWindowHints)0;
	if (minW > 0 || minH > 0) {
		geom.min_width = minW > 0 ? minW : -1;
		geom.min_height = minH > 0 ? minH : -1;
		hints = (GdkWindowHints)(hints | GDK_HINT_MIN_SIZE);
	}
	if (maxW > 0 || maxH > 0) {
		geom.max_width = maxW > 0 ? maxW : G_MAXINT;
		geom.max_height = maxH > 0 ? maxH : G_MAXINT;
		hints = (GdkWindowHints)(hints | GDK_HINT_MAX_SIZE);
	}
	if (hints != 0) {
		gtk_window_set_geometry_hints(w, NULL, &geom, hints);
	}
}

extern gboolean gantryOnDeleteEvent(GtkWidget *widget, GdkEvent *event, gpointer data);
extern gboolean gantryOnFocusOut(GtkWidget *widget, GdkEvent *event, gpointer data);

static void gantry_connect_delete(GtkWindow *w) {
	g_signal_connect(G_OBJECT(w), "delete-event", G_CALLBACK(gantryOnDeleteEvent), NULL);
}

static void gantry_connect_focus_out(GtkWindow *w) {
	g_signal_connect(G_OBJECT(w), "focus-out-event", G_CALLBACK(gantryOnFocusOut), NULL);
}

static void gantry_get_position(GtkWindow *w, int *x, int *y, int *width, int *height) {
	gtk_window_get_position(w, x, y);
	gtk_window_get_size(w, width, height);
}
*/
import "C"

import (
	"sync/atomic"
	"unsafe"
)

// GdkWindowEdge values for gantry_begin_resize_drag.
var gdkEdges = map[string]C.int{
	"nw": 0, "n": 1, "ne": 2,
	"w": 3, "e": 4,
	"sw": 5, "s": 6, "se": 7,
}

func gtkWin(p unsafe.Pointer) *C.GtkWindow { return (*C.GtkWindow)(p) }

func gtkBeginMoveDrag(p unsafe.Pointer)  { C.gantry_begin_move_drag(gtkWin(p)) }
func gtkBeginResizeDrag(p unsafe.Pointer, edge string) {
	if e, ok := gdkEdges[edge]; ok {
		C.gantry_begin_resize_drag(gtkWin(p), e)
	}
}
func gtkSetDecorated(p unsafe.Pointer, on bool) {
	C.gtk_window_set_decorated(gtkWin(p), gboolean(on))
}
func gtkSetKeepAbove(p unsafe.Pointer, on bool) {
	C.gtk_window_set_keep_above(gtkWin(p), gboolean(on))
}
func gtkSetSkipTaskbar(p unsafe.Pointer, on bool) {
	C.gtk_window_set_skip_taskbar_hint(gtkWin(p), gboolean(on))
}
func gtkSetAcceptFocus(p unsafe.Pointer, on bool) {
	C.gtk_window_set_accept_focus(gtkWin(p), gboolean(on))
}
func gtkIconify(p unsafe.Pointer)    { C.gtk_window_iconify(gtkWin(p)) }
func gtkMaximize(p unsafe.Pointer)   { C.gtk_window_maximize(gtkWin(p)) }
func gtkUnmaximize(p unsafe.Pointer) { C.gtk_window_unmaximize(gtkWin(p)) }
func gtkIsMaximized(p unsafe.Pointer) bool {
	return C.gtk_window_is_maximized(gtkWin(p)) != 0
}
func gtkMove(p unsafe.Pointer, x, y int) {
	C.gtk_window_move(gtkWin(p), C.gint(x), C.gint(y))
}
func gtkResize(p unsafe.Pointer, w, h int) {
	C.gtk_window_resize(gtkWin(p), C.gint(w), C.gint(h))
}
func gtkShow(p unsafe.Pointer) { C.gtk_widget_show(gtkWidget(p)) }
func gtkHide(p unsafe.Pointer) { C.gtk_widget_hide(gtkWidget(p)) }
func gtkPresent(p unsafe.Pointer) {
	C.gtk_window_present(gtkWin(p))
}
func gtkClose(p unsafe.Pointer) { C.gtk_window_close(gtkWin(p)) }
func gtkSetUrgency(p unsafe.Pointer, on bool) {
	C.gtk_window_set_urgency_hint(gtkWin(p), gboolean(on))
}
func gtkSetMinMax(p unsafe.Pointer, minW, minH, maxW, maxH int) {
	C.gantry_set_min_max(gtkWin(p), C.int(minW), C.int(minH), C.int(maxW), C.int(maxH))
}
func gtkGetGeometry(p unsafe.Pointer) Rect {
	var x, y, w, h C.int
	C.gantry_get_position(gtkWin(p), &x, &y, &w, &h)
	return Rect{X: int(x), Y: int(y), Width: int(w), Height: int(h)}
}
func gtkConnectDelete(p unsafe.Pointer)   { C.gantry_connect_delete(gtkWin(p)) }
func gtkConnectFocusOut(p unsafe.Pointer) { C.gantry_connect_focus_out(gtkWin(p)) }

// gtkWatchWebProcess auto-reloads the page if WebKit's render process
// crashes (otherwise: permanent white window).
func gtkWatchWebProcess(p unsafe.Pointer) { C.gantry_watch_webprocess(gtkWin(p)) }

func gtkWidget(p unsafe.Pointer) *C.GtkWidget { return (*C.GtkWidget)(p) }

func gboolean(b bool) C.gboolean {
	if b {
		return 1
	}
	return 0
}

// The delete-event hook serves the ONE window this process hosts (main
// window in the app process, one widget/popup per child process), so a
// single package-level slot is enough.
var (
	deleteHook   atomic.Pointer[deleteConfig]
	focusOutHook atomic.Pointer[func()]
)

type deleteConfig struct {
	disableClose bool
	onClose      func() CloseAction
	force        *atomic.Bool
	onClosing    func()
}

//export gantryOnDeleteEvent
func gantryOnDeleteEvent(widget *C.GtkWidget, event *C.GdkEvent, data C.gpointer) C.gboolean {
	cfg := deleteHook.Load()
	if cfg == nil {
		return 0 // proceed with the close
	}
	forced := cfg.force != nil && cfg.force.Load()
	if !forced {
		if cfg.disableClose {
			return 1 // swallow
		}
		action := CloseAllow
		if cfg.onClose != nil {
			action = cfg.onClose()
		}
		switch action {
		case CloseCancel:
			return 1
		case CloseHide:
			if cfg.onClosing != nil {
				cfg.onClosing()
			}
			C.gtk_widget_hide(widget)
			return 1
		}
	}
	if cfg.onClosing != nil {
		cfg.onClosing()
	}
	return 0
}

//export gantryOnFocusOut
func gantryOnFocusOut(widget *C.GtkWidget, event *C.GdkEvent, data C.gpointer) C.gboolean {
	if fn := focusOutHook.Load(); fn != nil {
		(*fn)()
	}
	return 0
}
