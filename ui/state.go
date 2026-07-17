package ui

import (
	"encoding/json"
	"log"
	"sync"
)

// stateEntry is one shared state variable: the current value (as JSON)
// and the Go-side observers of frontend writes.
type stateEntry struct {
	raw      json.RawMessage
	onChange []func(json.RawMessage)
}

// State is a value shared between Go and React in real time. The
// frontend reads and writes it with useGoState(name); Go reads and
// writes it here. Every change on either side reaches the other
// instantly (Go -> React over the websocket push, React -> Go over a
// setstate message).
type State[T any] struct {
	app  *App
	name string

	mu       sync.Mutex
	val      T
	watchers []func(T)
}

// NewState declares a shared state variable with an initial value. Do
// it once, at startup, before the frontend connects:
//
//	volume := ui.NewState(app, "volume", 0.5)
//	volume.Set(0.8)                       // React re-renders instantly
//	volume.OnChange(func(v float64) {...}) // React wrote it
func NewState[T any](a *App, name string, initial T) *State[T] {
	s := &State[T]{app: a, name: name, val: initial}
	raw, err := json.Marshal(initial)
	if err != nil {
		log.Printf("ui: state %s: initial value not serializable: %v", name, err)
		raw = []byte("null")
	}
	a.mu.Lock()
	if a.states == nil {
		a.states = map[string]*stateEntry{}
	}
	entry := &stateEntry{raw: raw}
	entry.onChange = append(entry.onChange, func(p json.RawMessage) {
		var v T
		if err := json.Unmarshal(p, &v); err != nil {
			log.Printf("ui: state %s: frontend sent unusable value: %v", name, err)
			return
		}
		s.mu.Lock()
		s.val = v
		fns := append([]func(T){}, s.watchers...)
		s.mu.Unlock()
		for _, fn := range fns {
			fn(v)
		}
	})
	a.states[name] = entry
	a.mu.Unlock()
	return s
}

// Get returns the current value.
func (s *State[T]) Get() T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.val
}

// Set updates the value and pushes it to the frontend immediately.
func (s *State[T]) Set(v T) {
	s.mu.Lock()
	s.val = v
	s.mu.Unlock()
	raw, err := json.Marshal(v)
	if err != nil {
		log.Printf("ui: state %s: value not serializable: %v", s.name, err)
		return
	}
	s.app.setStateRaw(s.name, raw, true)
}

// OnChange registers fn to run whenever the FRONTEND writes the value
// (Go-side Set calls do not trigger it - you made those).
func (s *State[T]) OnChange(fn func(T)) {
	s.mu.Lock()
	s.watchers = append(s.watchers, fn)
	s.mu.Unlock()
}

// setStateRaw stores the raw value and optionally pushes to every
// client (Go-initiated changes push; frontend-initiated ones do not
// echo to their writer - see the readLoop's setstate mirroring).
func (a *App) setStateRaw(name string, raw json.RawMessage, push bool) {
	a.mu.Lock()
	entry := a.states[name]
	if entry != nil {
		entry.raw = raw
	}
	a.mu.Unlock()
	if entry == nil || !push {
		return
	}
	for _, c := range a.allClients() {
		c.write(stateMsg{T: "state", Key: name, P: raw})
	}
}

// applyFrontendState handles a setstate message from the client and
// reports whether the state exists (unknown writes are dropped).
func (a *App) applyFrontendState(name string, raw json.RawMessage) bool {
	a.mu.Lock()
	entry := a.states[name]
	if entry != nil {
		entry.raw = raw
	}
	var fns []func(json.RawMessage)
	if entry != nil {
		fns = append(fns, entry.onChange...)
	}
	a.mu.Unlock()
	if entry == nil {
		log.Printf("ui: setstate for unknown state %q (declare it with ui.NewState)", name)
		return false
	}
	for _, fn := range fns {
		fn(raw)
	}
	return true
}

// snapshotStates returns every declared state for a fresh connection.
func (a *App) snapshotStates() []stateMsg {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]stateMsg, 0, len(a.states))
	for name, e := range a.states {
		out = append(out, stateMsg{T: "state", Key: name, P: e.raw})
	}
	return out
}

type stateMsg struct {
	T   string          `json:"t"`
	Key string          `json:"key"`
	P   json.RawMessage `json:"p"`
}
