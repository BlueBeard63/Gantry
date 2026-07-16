package appshell

import (
	"context"
	"log"

	"github.com/B-Commissions/Gantry/tray"
)

// App wires the standard desktop lifecycle: a main window that runs on
// the main goroutine, an optional tray icon, and the minimize-to-tray
// dance. The app's HTTP server (and everything else) runs on other
// goroutines; Run blocks the caller (which must be the main goroutine)
// until ctx is done.
type App struct {
	Window WindowOptions
	// Tray, when non-nil, shows a tray icon. An "Open <Title>" item is
	// prepended (unless TrayOnly) and a "Quit" item appended around the
	// caller's Menu; Quit closes the window and cancels the app context.
	// With a tray, closing the window leaves the app running - reopen it
	// from the tray. Without one, closing the window cancels ctx.
	Tray *tray.Options
	// TrayOnly skips the main window entirely: the app lives in the
	// tray and only shows widgets/popups (or opens the browser).
	TrayOnly bool
	// Browser opens the URL in the default browser instead of a native
	// window (the --browser flag).
	Browser bool
	// OnWindowClosed runs each time the main window closes while the app
	// keeps running in the tray.
	OnWindowClosed func()
}

// Run drives the app until ctx is done. Call it from main() on the main
// goroutine; cancel must cancel ctx (tray Quit uses it).
func (a *App) Run(ctx context.Context, cancel context.CancelFunc) error {
	openWindow := make(chan struct{}, 1)
	queueOpen := func() {
		// Reveal the existing window if one is alive (hidden or
		// minimized); otherwise open a fresh one.
		if ShowMainWindow() {
			return
		}
		select {
		case openWindow <- struct{}{}:
		default:
		}
	}

	hasTray := a.Tray != nil
	if hasTray {
		o := *a.Tray
		var menu []tray.MenuItem
		if !a.TrayOnly && !a.Browser {
			label := "Open " + a.Window.Title
			if a.Window.Title == "" {
				label = "Open"
			}
			menu = append(menu,
				tray.MenuItem{Label: label, OnClick: func(*tray.Item) { queueOpen() }},
				tray.MenuItem{Separator: true},
			)
		}
		menu = append(menu, o.Menu...)
		menu = append(menu,
			tray.MenuItem{Separator: true},
			tray.MenuItem{Label: "Quit", OnClick: func(*tray.Item) { tray.Quit() }},
		)
		o.Menu = menu
		userExit := o.OnExit
		o.OnExit = func() {
			if userExit != nil {
				userExit()
			}
			// Quit means QUIT: close the main window too if it's open
			// (it blocks the main goroutine until it goes away).
			CloseMainWindow()
			cancel()
		}
		go tray.Run(o)
	}

	switch {
	case a.Browser:
		if err := OpenInBrowser(a.Window.URL); err != nil {
			log.Printf("appshell: could not open browser: %v (open %s manually)", err, a.Window.URL)
		}
	case a.TrayOnly:
		// nothing to open
	default:
		openWindow <- struct{}{} // show the window on launch
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-openWindow:
			log.Printf("appshell: opening the %s window", a.Window.Title)
			// Runs on the main goroutine (the webview needs it); blocks
			// until the window closes.
			if err := RunWindow(a.Window); err != nil {
				log.Printf("appshell: native window unavailable (%v), falling back to browser", err)
				if err := OpenInBrowser(a.Window.URL); err != nil {
					log.Printf("appshell: could not open browser: %v (open %s manually)", err, a.Window.URL)
				}
			} else if hasTray {
				if a.OnWindowClosed != nil {
					a.OnWindowClosed()
				}
				log.Print("appshell: window closed - still running in the tray")
				continue
			}
			if !hasTray {
				// No tray to live in: window gone means app done.
				cancel()
			}
		}
	}
}
