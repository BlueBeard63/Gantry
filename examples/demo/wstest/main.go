// Wire-protocol smoke test used during development: connects like the
// frontend would, mounts pages/index, clicks the counter button twice,
// and checks the re-renders. Run: go run ./wstest (with demo --no-open
// --port 8331 running).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/coder/websocket"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := "ws://127.0.0.1:8331/gantry/ws"
	if len(os.Args) > 1 {
		url = "ws://127.0.0.1:" + os.Args[1] + "/gantry/ws"
	}
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		fmt.Println("DIAL FAILED:", err)
		os.Exit(1)
	}
	defer c.Close(websocket.StatusNormalClosure, "")
	c.SetReadLimit(1 << 20)

	send := func(v string) {
		if err := c.Write(ctx, websocket.MessageText, []byte(v)); err != nil {
			fmt.Println("WRITE FAILED:", err)
			os.Exit(1)
		}
	}
	read := func() map[string]any {
		_, data, err := c.Read(ctx)
		if err != nil {
			fmt.Println("READ FAILED:", err)
			os.Exit(1)
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		m["_raw"] = string(data)
		return m
	}

	countRe := regexp.MustCompile(`count is (\d+)`)
	clickRe := regexp.MustCompile(`"click":"(h\d+)"`)

	send(`{"t":"ready","page":"pages/index"}`)
	// New connections receive the shared-state snapshot (and possibly
	// other frames) before the first render - skip to the render.
	var raw1 string
	for tries := 0; tries < 8; tries++ {
		r := read()
		if r["t"] == "render" {
			raw1 = r["_raw"].(string)
			break
		}
		fmt.Println("skipping pre-render frame:", r["_raw"])
	}
	if raw1 == "" {
		fmt.Println("NEVER got a render")
		os.Exit(1)
	}
	// The Model keeps state across connections (that is the point), so
	// read the current count instead of assuming zero.
	m1 := countRe.FindStringSubmatch(raw1)
	if m1 == nil {
		fmt.Println("EXPECTED 'count is N' in first render:", raw1)
		os.Exit(1)
	}
	fmt.Printf("PASS: initial render, count is %s\n", m1[1])

	// Press the button twice, expecting the count to step each time.
	// Renders are whole-tree and may coalesce or duplicate around
	// activation, so read until the count moves (bounded).
	raw := raw1
	for i := 0; i < 2; i++ {
		id := clickRe.FindStringSubmatch(raw)
		if id == nil {
			fmt.Println("NO click handler in render:", raw)
			os.Exit(1)
		}
		before := countRe.FindStringSubmatch(raw)[1]
		send(fmt.Sprintf(`{"t":"event","h":"%s"}`, id[1]))
		advanced := false
		for tries := 0; tries < 4 && !advanced; tries++ {
			raw = read()["_raw"].(string)
			after := countRe.FindStringSubmatch(raw)
			advanced = after != nil && after[1] != before
		}
		if !advanced {
			fmt.Printf("EXPECTED count to advance from %s: %s\n", before, raw)
			os.Exit(1)
		}
		fmt.Printf("PASS: click re-rendered, count is %s\n", countRe.FindStringSubmatch(raw)[1])
	}

	// Paired event to a component handler (logs on the server side).
	send(`{"t":"event","key":"components/example","name":"ping","p":123}`)
	// And an unknown one - server should log, not die.
	send(`{"t":"event","key":"components/nope","name":"x"}`)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("PASS: paired events sent without connection drop")

	// The error pipeline: a panicking call handler must reject the call
	// with code "panic.call" AND push an {"t":"error"} frame carrying
	// the page and the breadcrumb trail.
	send(`{"t":"call","key":"pages/debug","name":"callBoom","id":"c99"}`)
	var gotReply, gotError bool
	for tries := 0; tries < 10 && !(gotReply && gotError); tries++ {
		r := read()
		switch r["t"] {
		case "reply":
			if r["id"] != "c99" {
				continue
			}
			if r["ok"] == true {
				fmt.Println("EXPECTED callBoom to fail:", r["_raw"])
				os.Exit(1)
			}
			if r["code"] != "panic.call" {
				fmt.Println("EXPECTED code panic.call on reply:", r["_raw"])
				os.Exit(1)
			}
			gotReply = true
		case "error":
			p, _ := r["p"].(map[string]any)
			if p["kind"] != "call-panic" || p["code"] != "panic.call" {
				fmt.Println("UNEXPECTED error frame:", r["_raw"])
				os.Exit(1)
			}
			if p["page"] != "pages/index" {
				fmt.Println("EXPECTED page pages/index on error frame:", r["_raw"])
				os.Exit(1)
			}
			if trail, ok := p["trail"].([]any); !ok || len(trail) == 0 {
				fmt.Println("EXPECTED a non-empty trail on error frame:", r["_raw"])
				os.Exit(1)
			}
			gotError = true
		}
	}
	if !gotReply || !gotError {
		fmt.Printf("EXPECTED panic reply and error frame (reply=%v error=%v)\n", gotReply, gotError)
		os.Exit(1)
	}
	fmt.Println("PASS: call panic rejected with panic.call and pushed an error frame with page + trail")

	// The connection survived the panic; a normal call still answers.
	send(`{"t":"call","key":"gantry","name":"env","id":"c100"}`)
	for tries := 0; tries < 6; tries++ {
		r := read()
		if r["t"] == "reply" && r["id"] == "c100" {
			if r["ok"] != true {
				fmt.Println("EXPECTED env call to succeed:", r["_raw"])
				os.Exit(1)
			}
			p, _ := r["p"].(map[string]any)
			if p["mode"] == nil || p["args"] == nil {
				fmt.Println("EXPECTED mode and args in env reply:", r["_raw"])
				os.Exit(1)
			}
			fmt.Printf("PASS: env call answered after panic (mode=%v)\n", p["mode"])
			fmt.Println("ALL WS TESTS PASSED")
			return
		}
	}
	fmt.Println("NEVER got the env reply")
	os.Exit(1)
}
