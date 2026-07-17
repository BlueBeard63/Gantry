package gantrytest

import (
	"encoding/json"
	"strings"
)

// Node is one element of a deserialized Tea render tree - the wire
// shape of a page Model's View(). Trees come from App.Tree, NextRender
// and WaitTree; Find/FindAll walk them so a Tea-style app can be
// asserted on entirely over the protocol plane, no browser tooling.
type Node struct {
	Type     string            `json:"type"`
	Key      string            `json:"key,omitempty"`
	Props    map[string]any    `json:"props,omitempty"`
	Handlers map[string]string `json:"handlers,omitempty"`
	Children []Node            `json:"children,omitempty"`
}

// parseTree deserializes a render frame's tree field.
func parseTree(raw json.RawMessage) (*Node, error) {
	var n Node
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

// Match narrows Find beyond the node type or selector. Combine freely:
// tree.Find("button", Text("Save")). Text works on both planes (tree
// and DOM Find); the structural matchers below are tree-only.
type Match interface {
	matchNode(*Node) bool
}

// MatchFunc adapts a custom predicate into a (tree-plane) Match.
type MatchFunc func(*Node) bool

func (f MatchFunc) matchNode(n *Node) bool { return f(n) }

// textMatch carries the substring so the DOM plane can reuse it as an
// element text filter.
type textMatch string

func (m textMatch) matchNode(n *Node) bool {
	return strings.Contains(n.ownText(), string(m))
}

// Text matches nodes whose own text-bearing props (text, label, title,
// placeholder) contain substr. On the DOM plane (Page.Find) it filters
// elements by their rendered text instead.
func Text(substr string) Match {
	return textMatch(substr)
}

// Key matches nodes with this exact key.
func Key(key string) Match {
	return MatchFunc(func(n *Node) bool { return n.Key == key })
}

// Prop matches nodes whose prop equals want (compared via JSON
// round-trip semantics: numbers arrive as float64).
func Prop(name string, want any) Match {
	return MatchFunc(func(n *Node) bool { return n.Props[name] == want })
}

// HasHandler matches nodes that carry a handler for the event
// ("click", "change", ...).
func HasHandler(event string) Match {
	return MatchFunc(func(n *Node) bool { return n.Handlers[event] != "" })
}

// Find returns the first node (depth-first, including the receiver)
// with the given type that satisfies every match, or nil. An empty typ
// matches any type.
func (n *Node) Find(typ string, matches ...Match) *Node {
	if n == nil {
		return nil
	}
	if n.matches(typ, matches) {
		return n
	}
	for i := range n.Children {
		if found := n.Children[i].Find(typ, matches...); found != nil {
			return found
		}
	}
	return nil
}

// FindAll returns every matching node in depth-first order.
func (n *Node) FindAll(typ string, matches ...Match) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	if n.matches(typ, matches) {
		out = append(out, n)
	}
	for i := range n.Children {
		out = append(out, n.Children[i].FindAll(typ, matches...)...)
	}
	return out
}

func (n *Node) matches(typ string, matches []Match) bool {
	if typ != "" && n.Type != typ {
		return false
	}
	for _, m := range matches {
		if !m.matchNode(n) {
			return false
		}
	}
	return true
}

// Handler returns the node's handler id for the event ("click",
// "change", ...), or "" when the node has none. Fire it with
// App.TeaEvent, or use App.Click for the common case.
func (n *Node) Handler(event string) string {
	if n == nil {
		return ""
	}
	return n.Handlers[event]
}

// Text returns the concatenated text content of the subtree: every
// text-bearing prop, depth-first, space-separated.
func (n *Node) Text() string {
	if n == nil {
		return ""
	}
	var parts []string
	n.walk(func(c *Node) {
		if s := c.ownText(); s != "" {
			parts = append(parts, s)
		}
	})
	return strings.Join(parts, " ")
}

// ownText is the node's own text content, not the subtree's.
func (n *Node) ownText() string {
	var parts []string
	for _, prop := range []string{"text", "label", "title", "placeholder"} {
		if s, ok := n.Props[prop].(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

func (n *Node) walk(fn func(*Node)) {
	fn(n)
	for i := range n.Children {
		n.Children[i].walk(fn)
	}
}
