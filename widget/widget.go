// Package widget describes Android home-screen widgets as small
// declarative layout trees. An app pairs a gantry.json widget entry
// (launcher metadata) with widgets/<name>/<name>.go exporting:
//
//	var Widget = widget.New("status", func() widget.Node {
//		return widget.Column(
//			widget.Text("Steeping").Bold(),
//			widget.Progress(0.4),
//		)
//	})
//
// The generated Kotlin shell renders the tree with Glance. Render
// functions must be self-contained - they run via --emit-widgets in a
// short-lived process without the app loop, so read persisted state
// (files under $HOME), never live app state. Widgets have no handlers
// in v1: tapping a widget opens the app.
package widget

import "fmt"

// Widget pairs a name with its render function. The name must match
// the widgets/<name>/ directory and the gantry.json entry.
type Widget struct {
	Name   string
	Render func() Node
}

// New declares a widget; assign it to the exported Widget var of a
// widgets/<name>/<name>.go file so the generated registry finds it.
func New(name string, render func() Node) Widget {
	return Widget{Name: name, Render: render}
}

// Node is one element of a widget's layout tree, serialized as JSON
// for the Kotlin renderer. Zero fields are omitted - the renderer owns
// the defaults so they stay consistent across apps.
type Node struct {
	Type     string  `json:"type"` // column | row | text | progress | spacer | divider
	Text     string  `json:"text,omitempty"`
	Value    float64 `json:"value,omitempty"` // progress fraction 0..1
	SizeSp   int     `json:"size,omitempty"`  // text size in sp
	ColorHex string  `json:"color,omitempty"` // "#rrggbb"
	IsBold   bool    `json:"bold,omitempty"`
	IsDim    bool    `json:"dim,omitempty"`
	Lines    int     `json:"maxLines,omitempty"`
	Children []Node  `json:"children,omitempty"`
}

// Column stacks children vertically.
func Column(children ...Node) Node { return Node{Type: "column", Children: children} }

// Row lays children out horizontally.
func Row(children ...Node) Node { return Node{Type: "row", Children: children} }

// Text is a text line.
func Text(s string) Node { return Node{Type: "text", Text: s} }

// Textf is Text with Sprintf formatting.
func Textf(format string, a ...any) Node { return Text(fmt.Sprintf(format, a...)) }

// Progress is a horizontal progress bar; v is clamped to 0..1.
func Progress(v float64) Node {
	return Node{Type: "progress", Value: min(1, max(0, v))}
}

// Spacer is flexible whitespace: it expands to push siblings apart.
func Spacer() Node { return Node{Type: "spacer"} }

// Divider is a thin horizontal rule.
func Divider() Node { return Node{Type: "divider"} }

// Size sets the text size in sp (Android's scalable text unit).
func (n Node) Size(sp int) Node { n.SizeSp = sp; return n }

// Color sets the text or bar colour as "#rrggbb".
func (n Node) Color(hex string) Node { n.ColorHex = hex; return n }

// Bold renders the text bold.
func (n Node) Bold() Node { n.IsBold = true; return n }

// Dim renders the text in the muted foreground colour.
func (n Node) Dim() Node { n.IsDim = true; return n }

// MaxLines truncates the text after n lines (ellipsized).
func (n Node) MaxLines(count int) Node { n.Lines = count; return n }
