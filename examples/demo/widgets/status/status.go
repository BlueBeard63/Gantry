// Package status is the demo's home-screen widget. Render functions
// run via --emit-widgets in a short-lived process without the app loop,
// so they must be self-contained: read persisted state (files under
// $HOME), never live app state.
package status

import (
	"time"

	"github.com/B-Commissions/Gantry/widget"
)

var Widget = widget.New("status", func() widget.Node {
	now := time.Now()
	dayFrac := float64(now.Hour()*60+now.Minute()) / (24 * 60)
	return widget.Column(
		widget.Row(
			widget.Text("Demo").Bold().Size(14),
			widget.Spacer(),
			widget.Text(now.Format("Mon 15:04")).Dim().Size(11),
		),
		widget.Spacer(),
		widget.Textf("%.0f%% of the day gone", dayFrac*100).Size(12),
		widget.Progress(dayFrac).Color("#4ade80"),
	)
})
