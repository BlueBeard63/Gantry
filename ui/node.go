package ui

import (
	"encoding/json"
	"fmt"
)

// Node is one element of a Go-driven UI tree. View() returns a Node;
// the gantry-web runtime renders it with real React components. Built-in
// types (column, row, text, button, ...) come from the builders below;
// Custom renders any React component the app registered - including the
// paired .tsx of a components/ folder.
type Node struct {
	Type     string
	Key      string
	Props    map[string]any
	Children []Node

	handlers map[string]handlerFn
}

type handlerFn func(payload json.RawMessage) Msg

// wireNode is the JSON shape sent to the runtime; handlers become
// generation-scoped IDs the client echoes back in events.
type wireNode struct {
	Type     string            `json:"type"`
	Key      string            `json:"key,omitempty"`
	Props    map[string]any    `json:"props,omitempty"`
	Handlers map[string]string `json:"handlers,omitempty"`
	Children []wireNode        `json:"children,omitempty"`
}

func node(typ string, children ...Node) Node {
	return Node{Type: typ, Children: children}
}

// Column stacks children vertically.
func Column(children ...Node) Node { return node("column", children...) }

// Row lays children out horizontally.
func Row(children ...Node) Node { return node("row", children...) }

// Text renders a text span.
func Text(s string) Node {
	return Node{Type: "text", Props: map[string]any{"text": s}}
}

// Textf renders formatted text.
func Textf(format string, args ...any) Node {
	return Text(fmt.Sprintf(format, args...))
}

// Heading renders larger emphasized text.
func Heading(s string) Node {
	return Node{Type: "heading", Props: map[string]any{"text": s}}
}

// Button sends onClick to Update when pressed.
func Button(label string, onClick Msg) Node {
	n := Node{Type: "button", Props: map[string]any{"label": label}}
	return n.OnEvent("click", func(json.RawMessage) Msg { return onClick })
}

// Input is a one-line text field. onChange receives the new value on
// every keystroke. The runtime renders it semi-controlled: keystrokes
// echo locally at once and the server value only overrides when it
// really differs, so typing never lags the round trip.
func Input(value string, onChange func(string) Msg) Node {
	n := Node{Type: "input", Props: map[string]any{"value": value}}
	return n.OnEvent("change", func(p json.RawMessage) Msg {
		var s string
		_ = json.Unmarshal(p, &s)
		return onChange(s)
	})
}

// Checkbox toggles; onToggle receives the new checked state.
func Checkbox(label string, checked bool, onToggle func(bool) Msg) Node {
	n := Node{Type: "checkbox", Props: map[string]any{"label": label, "checked": checked}}
	return n.OnEvent("change", func(p json.RawMessage) Msg {
		var b bool
		_ = json.Unmarshal(p, &b)
		return onToggle(b)
	})
}

// Select is a dropdown; onChange receives the newly selected option.
func Select(value string, options []string, onChange func(string) Msg) Node {
	n := Node{Type: "select", Props: map[string]any{"value": value, "options": options}}
	return n.OnEvent("change", func(p json.RawMessage) Msg {
		var s string
		_ = json.Unmarshal(p, &s)
		return onChange(s)
	})
}

// Divider renders a horizontal rule.
func Divider() Node { return Node{Type: "divider"} }

// Spacer flexes to fill leftover space in a Row or Column.
func Spacer() Node { return Node{Type: "spacer"} }

// Progress renders a bar filled to v (0..1).
func Progress(v float64) Node {
	return Node{Type: "progress", Props: map[string]any{"value": v}}
}

// Custom renders a React component registered with the runtime - by a
// paired components/ folder (use its key, e.g.
// Custom("components/gauge", ...)) or registered explicitly via
// createApp. Children render into the component's children prop.
func Custom(component string, props map[string]any, children ...Node) Node {
	return Node{Type: component, Props: props, Children: children}
}

// WithKey sets the reconciliation key (set it on list items so React
// keeps their identity when the list reorders).
func (n Node) WithKey(k string) Node {
	n.Key = k
	return n
}

// WithProps adds alternating name, value pairs to the node's props -
// style hints the built-ins understand ("gap", 8, "pad", 12, "grow",
// true, "class", "my-css-class") or any prop a Custom component wants.
func (n Node) WithProps(kv ...any) Node {
	props := make(map[string]any, len(n.Props)+len(kv)/2)
	for k, v := range n.Props {
		props[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if name, ok := kv[i].(string); ok {
			props[name] = kv[i+1]
		}
	}
	n.Props = props
	return n
}

// OnEvent attaches a raw event handler (the escape hatch behind Button,
// Input, etc.); the payload is whatever the component emitted.
func (n Node) OnEvent(event string, fn func(payload json.RawMessage) Msg) Node {
	handlers := make(map[string]handlerFn, len(n.handlers)+1)
	for k, v := range n.handlers {
		handlers[k] = v
	}
	handlers[event] = fn
	n.handlers = handlers
	return n
}
