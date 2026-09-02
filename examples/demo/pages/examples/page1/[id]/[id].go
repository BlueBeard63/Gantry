//go:build ignore

// The Go half of a dynamic page. It lives in a bracket folder ([id]) that
// Go cannot import, so gantry copies it into internal/gantrydyn at
// gen/dev/build time - the //go:build ignore line above keeps `go build
// ./...` and your editor from choking on the un-importable original. The
// Model receives a ui.ParamsMsg whenever the id in the URL changes
// (/examples/page1/1 -> /2), so the Go side always knows which id is open.
package dynid

import (
	"fmt"

	"github.com/BlueBeard63/Gantry/ui"
)

var Page = ui.Page{
	// The Key is the bracket folder path and must match the paired tsx;
	// the route it serves is /examples/page1/[id].
	Key:   "pages/examples/page1/[id]",
	Model: func() ui.Model { return model{} },
}

type model struct {
	id string
}

func (m model) Init() ui.Cmd { return nil }

func (m model) Update(msg ui.Msg) (ui.Model, ui.Cmd) {
	switch v := msg.(type) {
	case ui.ParamsMsg:
		m.id = v.Params.Get("id")
	}
	return m, nil
}

func (m model) View() ui.Node {
	return ui.Column(
		ui.Text(fmt.Sprintf("The Go half sees id = %q", m.id)),
	).WithProps("class", "dyn-go")
}
