# The wire protocol

What travels over /gantry/ws. You never speak this by hand - gantry-web
does - but knowing it makes debugging with the network tab trivial.

All messages are JSON text frames with a "t" discriminator.

## Client -> server

```json
{"t":"ready","page":"pages/index"}
```

Sent when a page mounts (and again on every reconnect). The server
makes this page the active one: if it has a Model, the program starts
(first mount) or re-attaches, and a full render comes back.

```json
{"t":"event","h":"h42","p":123}
```

A Tea event: h is the handler id from the current render tree, p the
payload (button clicks send none, inputs send the value).

```json
{"t":"event","key":"pages/index","name":"buttonPress","p":{"n":3}}
```

A paired event: key names the pair, name the handler in its
ui.Handlers, p the payload.

```json
{"t":"call","key":"auth","name":"login","id":"c3","p":{"user":"jack"}}
```

An awaited call (usePaired().call, useService, useCall): key is a pair
key or a service name, id correlates the reply.

```json
{"t":"setstate","key":"volume","p":0.8}
```

A frontend write to a shared state variable (useGoState). Not echoed
back to the sender - it already applied locally.

## Server -> client

```json
{"t":"render","seq":7,"tree":{"type":"column","children":[...]}}
```

A full Tea render for the active page. seq increases per connection;
the tree is the serialized View():

```json
{
  "type": "button",
  "key": "save-btn",
  "props": {"label": "Save"},
  "handlers": {"click": "h43"},
  "children": []
}
```

```json
{"t":"push","key":"components/gauge","name":"state","p":{"value":0.7}}
```

A paired push from app.Push. Pushes named "state" feed
usePaired().state; everything else goes to on() subscribers.

```json
{"t":"reply","id":"c3","ok":true,"p":{"name":"jack"}}
{"t":"reply","id":"c3","ok":false,"err":"bad password"}
```

The answer to a call: resolves or rejects the awaiting promise. The
client times a call out after 30 seconds if no reply lands.

```json
{"t":"state","key":"volume","p":0.8}
```

A shared state value (ui.NewState / useGoState): sent for every
declared state when a client connects, then on every Go-side Set.

## Semantics worth knowing

- One client. A new connection replaces the old one (the server closes
  it). This is what makes webview reloads, React StrictMode's double
  mount, and dev restarts all Just Work - the newest connection wins
  and announces its page with ready.
- Whole-tree renders, coalesced. Every Update sends the full tree; a
  burst of updates collapses so the newest state wins. There is no
  diffing to debug.
- Handler generations. Each render assigns fresh handler ids and the
  server keeps exactly one previous generation, so an event racing a
  re-render (you clicked while the tree was being replaced) still
  resolves. Ids older than that are dropped silently - by design, since
  their meaning is gone.
- Reconnect. The client retries with backoff (300ms doubling to 5s)
  and re-sends ready on connect; the server responds with a fresh
  render. Nothing is queued for a dead client - pushes while
  disconnected are dropped, so treat Push as UI mirroring, not a
  message queue.
- Keepalive. The server pings every 30 seconds.

## Debugging

Set WindowOptions.Debug = true, open devtools in the window, and watch
the WS tab: every render, event and push is right there. On the Go
side, unresolved paired events log with the exact key and name that
missed.
