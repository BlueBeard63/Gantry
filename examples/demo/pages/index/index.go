// The Go half of the index page. Its Model owns the page state and
// logic; the paired index.tsx renders it with <TeaView />. Update this
// file, save, and gantry dev restarts the app with the new logic.
package index

import (
	"encoding/json"
	"fmt"

	"github.com/B-Commissions/Gantry/gantry"
	"github.com/B-Commissions/Gantry/ui"
)

var Page = ui.Page{
	Key:   "pages/index",
	Model: func() ui.Model { return model{msg: resourceMsg()} },
}

// resourceMsg reads a value out of the embedded resources/ tree - the
// same file the frontend reaches at /resources/info.json - to show a
// resource being consumed from the Go plane.
func resourceMsg() string {
	b, err := gantry.Resource("info.json")
	if err != nil {
		return ""
	}
	var info struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &info) != nil {
		return ""
	}
	return info.Message
}

// model is the page state.
type model struct {
	count int
	msg   string
}

// incMsg is sent when the button is clicked.
type incMsg struct{}

func (m model) Init() ui.Cmd { return nil }

func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
	switch msg.(type) {
	case incMsg:
		m.count++
	}
	return m, nil
}

func (m model) View() ui.Node {
	col := ui.Column(
		ui.Button(fmt.Sprintf("count is %d", m.count), incMsg{}),
	).WithProps("class", "counter")
	if m.msg != "" {
		col = ui.Column(
			ui.Button(fmt.Sprintf("count is %d", m.count), incMsg{}),
			ui.Text(m.msg),
		).WithProps("class", "counter")
	}
	return col
}
