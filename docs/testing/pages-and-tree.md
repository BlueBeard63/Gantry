# Pages & the tree

The protocol plane starts here: the driver mounts a page the way the frontend does, and the app answers with a render. This page covers announcing page mounts, reading the render frames that come back, and walking the resulting tree with the full set of finders and matchers. Once you can find a node you can act on it - firing events into it and awaiting calls is [Events & calls](events-and-calls.md); reading and asserting shared state, pushes and restarts is [State, pushes & restarts](state-and-restarts.md). This all works headless on every platform; driving the real webview's DOM elements instead is [The DOM plane](dom.md), which layers onto the same `App`. If you have not launched an app yet, start at [Setup](setup.md).

## Announcing a page mount

`app.Ready("pages/index")` announces a page mount, exactly like the frontend does when a page component mounts: the server makes it the active page, and a Tea page responds with a full render. It is the first thing almost every test does after `Launch`, because until a page is active there is nothing to render or query.

```go
app := gantrytest.Launch(t)
app.Ready("pages/index")
tree := app.Tree()
```

The string is the page key - the `pages/`-rooted route the frontend would mount (`pages/index`, `pages/settings`, `pages/user/[id]`). `Ready` only sends the mount; it does not itself wait for the render, so pair it with one of the tree reads below.

## Reading renders

The driver keeps every render frame the app sends and never discards one. Three verbs read them, differing only in *which* frame you get and *how long* they wait:

```go
tree := app.Tree()          // waits for >=1 render, returns the newest tree
tree  = app.NextRender()    // the next render this test has not consumed yet
tree  = app.WaitTree("row appeared", func(n *gantrytest.Node) bool {
	return n.Find("row", gantrytest.Key("user-42")) != nil
})
```

`Tree()` waits until at least one render has arrived and returns the newest one - the right call for "what is on screen right now". `NextRender()` advances a per-test cursor and returns the next render this test has not yet consumed, so consecutive calls step through the render stream one frame at a time. `WaitTree(what, pred)` re-evaluates `pred` against the newest tree as each render lands and returns the first tree that satisfies it; `what` is the human label that shows up in the timeout message if the predicate never comes true.

### Why you wait on content, never on counts

Renders are whole-tree and coalesce under bursts: the newest state wins and intermediate frames may never exist as separate messages. So never count renders or assume one event yields exactly one frame - wait on tree *content* with `WaitTree`. This is exactly why the counter test loops on the label it expects rather than on click count:

```go
for _, want := range []string{"count is 1", "count is 2"} {
	app.Click(tree.Find("button", gantrytest.Text("count is")))
	tree = app.WaitTree(want, func(n *gantrytest.Node) bool {
		return strings.Contains(n.Text(), want)
	})
}
```

Because `WaitTree` returns the tree it matched, the idiom "act, then wait for the consequence, reusing the returned tree" keeps every subsequent query pointed at a fresh render. Caching a `Node` from an old tree and acting on it much later is the one thing to avoid - see the handler-generation note on [Events & calls](events-and-calls.md#handler-generations).

Every waiting verb here respects the test deadline (the launch timeout, capped by the Go test's own deadline) and, on timeout, fails with the last 20 protocol frames attached, so a render that never arrived shows you the frames that did. The failure format lives on [Errors & artifacts](errors-and-artifacts.md#artifacts).

## The Node tree

A render deserializes into a walkable `Node` tree that mirrors the serialized `View()` of the page's Model:

```go
type Node struct {
	Type     string            // "button", "row", "input", ...
	Key      string            // the node's key, when it has one
	Props    map[string]any    // text/label/title/placeholder/... decoded from JSON
	Handlers map[string]string // event name -> handler id ("click" -> "h7")
	Children []Node
}
```

`Props` values arrive with JSON semantics: numbers are `float64`, objects are `map[string]any`. `Handlers` maps an event name to the opaque handler id you fire with `TeaEvent` (or, for clicks, via `Click`). You rarely touch these fields directly - the finders and matchers below do it for you - but knowing the shape explains what the matchers compare against.

## Finding nodes

Two methods walk the tree, both depth-first and both including the receiver node itself:

```go
btn  := tree.Find("button", gantrytest.Text("Save"))   // first match, or nil
rows := tree.FindAll("row")                             // every match, in order
name := tree.Find("", gantrytest.Key("title")).Text()  // "" type matches anything
id   := btn.Handler("click")                            // this node's click handler id, "" if none
```

`Find(typ, matches...)` returns the first node whose type equals `typ` and which satisfies *every* matcher, or `nil` when nothing matches. An empty `typ` (`""`) matches any type, so you can select purely by matcher - `Find("", Key("title"))` finds whatever node carries that key. `FindAll` returns all matches in depth-first order, and returns `nil` (a zero-length slice you can safely `range` and `len`) when there are none. Both are safe to call on a `nil` receiver, so chaining `tree.Find(...).Find(...)` never panics - an inner miss just propagates `nil`.

## Matchers

Matchers are the second-and-later arguments to `Find`/`FindAll`. Pass as many as you like; a node must satisfy all of them.

- `Text(substr)` - the node's own text-bearing props (`text`, `label`, `title`, `placeholder`) contain `substr` as a substring. This matcher is the only one that also works on the DOM plane, where it filters elements by rendered text instead - see [The DOM plane](dom.md#finding-elements).
- `Key(key)` - exact match on the node's `Key`.
- `Prop(name, want)` - the node's `Props[name]` equals `want`, compared with JSON round-trip semantics. Numbers arrive as `float64`, so compare against `float64(3)`, not `3`; a bare integer literal will never match.
- `HasHandler(event)` - the node carries a non-empty handler for that event (`HasHandler("click")`, `HasHandler("change")`).
- `MatchFunc(func(*Node) bool)` - any custom predicate, for anything the built-ins do not express (matching on a nested child, a computed prop, a regex over text).

`Text`, `Key`, `Prop`, and `HasHandler` all return a `Match`; `MatchFunc` adapts a plain predicate into one. They compose freely: `tree.FindAll("row", gantrytest.HasHandler("click"), gantrytest.Prop("selected", true))` returns every clickable, selected row.

## Reading text and handlers off a node

Two `Node` methods read content back out once you have found a node:

```go
label := tree.Find("", gantrytest.Key("status")).Text() // whole-subtree text, space-joined
hid   := btn.Handler("click")                            // one node's handler id, "" if absent
```

`Node.Text()` concatenates the text-bearing props of the *entire subtree* rooted at the node, depth-first and space-separated - this is what the `strings.Contains(n.Text(), ...)` idiom reads, and why asserting on a container's `Text()` catches a label anywhere inside it. (The `Text` *matcher*, by contrast, looks only at a node's own text, not its descendants'.) `Node.Handler(event)` returns just that one node's handler id for the event, or `""` when it has none; feed a non-empty id to `TeaEvent`, or let `Click` read the `"click"` handler for you. Both methods are `nil`-safe and return `""` on a `nil` node.

---

Next: [Events & calls](events-and-calls.md) - firing into the nodes you just found, and awaiting Go calls. See also [State, pushes & restarts](state-and-restarts.md), [The DOM plane](dom.md), and [Errors & artifacts](errors-and-artifacts.md).
