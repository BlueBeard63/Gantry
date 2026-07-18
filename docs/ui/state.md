# State

`useGoState` is a `useState` whose value lives in Go and syncs both ways instantly. Reach for it when a value is owned by both sides - volume, active session, feature toggles - and every subscriber should re-render the moment it changes, from React or from Go. For asking Go a one-off question, see [Calls and services](calls-and-services.md); for Go-driven UI, a [Tea Model](tea.md).

## Declaring shared state in Go

Declare a shared state variable in Go at startup:

```go
volume := ui.NewState(app, "volume", 0.5)

volume.Set(0.8)         // every React subscriber re-renders instantly
v := volume.Get()       // current value
volume.OnChange(func(v float64) {
    applyVolume(v)      // React wrote it
})
```

## Using it from React

React uses it exactly like useState:

```tsx
const [volume, setVolume] = useGoState("volume", 0.5);
<input type="range" value={volume} onChange={(e) => setVolume(Number(e.target.value))} />
```

Every component using the same key shares one value, and a set anywhere re-renders them all. A frontend set applies locally at once (no round-trip lag) and writes through to Go, firing Go's OnChange observers; a Go-side Set() pushes to the frontend immediately - "instant updates" from timers, watchers, background work.

## Notes

- **First render is correct.** A fresh or reconnected client receives every declared state before anything else, so useGoState is correct from the first render - the fallback argument only covers the moment before the socket connects. Declare states with ui.NewState BEFORE the window opens; writes to undeclared keys are logged and dropped.
