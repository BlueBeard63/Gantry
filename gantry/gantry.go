// Package gantry boots a Gantry app in one call. It owns the
// boilerplate every app shares - flags, the helper-window roles, the
// HTTP server, the websocket, the shell window and tray - so a main.go
// is a handful of lines:
//
//	func main() {
//		gantry.Run(gantry.Config{
//			Name:  "myapp",
//			Title: "My App",
//			Port:  8330,
//			Dist:  dist(),
//			Pairs: gantryPairs(),
//			Tray:  true,
//		})
//	}
//
// Everything is still reachable: Window tweaks the window options,
// Setup registers services/state/routes, Roles adds widget windows.
// Apps that outgrow Run can copy its body and own the loop - see the
// "Without the CLI" docs page.
package gantry

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"

	"github.com/B-Commissions/Gantry/appicon"
	"github.com/B-Commissions/Gantry/appshell"
	"github.com/B-Commissions/Gantry/tray"
	"github.com/B-Commissions/Gantry/ui"
)

// RoleArgs carries the standard child-window flags to a custom role.
type RoleArgs struct {
	Port     int
	URL      string
	Monitor  int
	Position string
}

// Config describes the app. Only Name, Title, Port, Dist and Pairs are
// required; everything else has sensible defaults.
type Config struct {
	// Name is the exe/app identity (WebView2 data folders, logs).
	Name string
	// Title is the window and tray title.
	Title string
	// Port is the local server port (and single-instance guard).
	Port int
	// Dist is the embedded frontend (embed.go's dist()).
	Dist fs.FS
	// Pairs registers every page/component - pass gantryPairs(), the
	// generated registry.
	Pairs []any

	// Tray keeps the app running in the tray when the window closes.
	Tray bool
	// TrayMenu adds actions between the standard Open and Quit items.
	TrayMenu []tray.MenuItem

	// Window tweaks the shell window before it opens (buttons, size,
	// close hook, chrome metrics) - the defaults are already sane.
	Window func(w *appshell.WindowOptions)
	// Setup runs once before serving: register services
	// (app.Service), shared state (ui.NewState), pushes, and your own
	// HTTP routes on mux.
	Setup func(app *ui.App, mux *http.ServeMux)
	// Roles adds custom child-window kinds (widgets) invoked with
	// --shellrole <name>; the "popup" role is built in.
	Roles map[string]func(a RoleArgs) error
}

// Run parses flags, dispatches helper roles, and drives the app until
// it quits. It never returns except on fatal startup errors.
func Run(cfg Config) {
	var (
		port      = flag.Int("port", cfg.Port, "local server port")
		browser   = flag.Bool("browser", false, "open in the default browser instead of a native window")
		noOpen    = flag.Bool("no-open", false, "headless: serve only, no window")
		devURL    = flag.String("dev-url", "", "dev: load the frontend from this URL (gantry dev sets it)")
		shellRole = flag.String("shellrole", "", "internal: run a helper window role instead of the app")
		roleURL   = flag.String("url", "", "internal: url for the helper window")
		monitor   = flag.Int("monitor", -1, "internal: monitor index for popups")
		position  = flag.String("position", "bottom", "internal: popup position top|bottom")
	)
	flag.Parse()

	// Helper window roles run as child processes (crash isolation):
	// the exe re-invokes itself with --shellrole and renders exactly
	// one window.
	if *shellRole != "" {
		defer appshell.RoleLog(cfg.Name, *shellRole)()
		args := RoleArgs{Port: *port, URL: *roleURL, Monitor: *monitor, Position: *position}
		var err error
		switch {
		case *shellRole == "popup":
			err = appshell.RunPopup(appshell.PopupOptions{
				AppName:  cfg.Name,
				URL:      args.URL,
				Width:    460,
				Height:   140,
				Monitor:  args.Monitor,
				Position: args.Position,
			})
		case cfg.Roles[*shellRole] != nil:
			err = cfg.Roles[*shellRole](args)
		default:
			log.Fatalf("unknown --shellrole %q", *shellRole)
		}
		if err != nil {
			log.Fatalf("%s: %v", *shellRole, err)
		}
		return
	}

	if err := run(cfg, *port, *browser, *noOpen, *devURL); err != nil {
		log.Fatal(err)
	}
}

func run(cfg Config, port int, browser, noOpen bool, devURL string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	app := ui.NewApp(cfg.Pairs...)

	mux := http.NewServeMux()
	mux.Handle("/gantry/ws", app.Handler())
	if cfg.Setup != nil {
		cfg.Setup(app, mux)
	}
	mux.Handle("/", appshell.ServeSPA(cfg.Dist))

	ln, err := appshell.Listen(port) // also the single-instance guard
	if err != nil {
		return err
	}
	server := &http.Server{Handler: mux}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ln) }()
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	url := "http://127.0.0.1:" + strconv.Itoa(port)
	if devURL != "" {
		url = devURL // gantry dev: HMR inside the native window
	}
	log.Printf("%s serving on %s", cfg.Title, url)

	if noOpen {
		return <-errCh
	}

	icon := appicon.Render(32, appicon.DefaultPalette())
	window := appshell.WindowOptions{
		AppName:   cfg.Name,
		Title:     cfg.Title,
		URL:       url,
		Width:     1100,
		Height:    720,
		MinWidth:  480,
		MinHeight: 320,
		AutoFocus: true,
		Icon:      appshell.IconSource{PNG: appicon.PNG(icon)},
		Geometry:  appshell.FileGeometry(geometryPath(cfg.Name)),
	}
	if cfg.Window != nil {
		cfg.Window(&window)
	}

	shell := &appshell.App{
		Window:  window,
		Browser: browser,
	}
	if cfg.Tray {
		shell.Tray = &tray.Options{
			Icon:    appicon.ICO(icon),
			IconPNG: appicon.PNG(icon),
			Title:   cfg.Title,
			Tooltip: cfg.Title + " is running",
			Menu:    cfg.TrayMenu,
		}
	}
	return shell.Run(ctx, cancel)
}

func geometryPath(name string) string {
	base, err := os.UserConfigDir()
	if err != nil {
		return "geometry.json"
	}
	return filepath.Join(base, name, "geometry.json")
}
