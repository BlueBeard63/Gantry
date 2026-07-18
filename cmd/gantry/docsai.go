package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// The docs assistant is an OPT-IN, on-device feature: `gantry docs --ai`
// wires a chat widget to an OpenAI-compatible chat endpoint. It defaults to
// a local Ollama server so your questions (and any pasted errors) never
// leave the machine; llama.cpp, LM Studio, or a hosted provider work too by
// overriding the env vars. The docs themselves stay fully offline - the
// assistant is purely additive.

// aiConfig points the assistant at an OpenAI-compatible /chat/completions
// endpoint.
type aiConfig struct {
	baseURL string // e.g. http://localhost:11434/v1 (no trailing slash)
	model   string // e.g. qwen2.5
	apiKey  string // optional; local servers usually need none
}

func newAIConfig() aiConfig {
	return aiConfig{
		baseURL: strings.TrimRight(aiEnvOr("GANTRY_DOCS_AI_URL", "http://localhost:11434/v1"), "/"),
		model:   aiEnvOr("GANTRY_DOCS_AI_MODEL", "qwen2.5"),
		apiKey:  os.Getenv("GANTRY_DOCS_AI_KEY"),
	}
}

func aiEnvOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// aiDoc is one documentation page prepared for retrieval: Plain is the
// lowercased body used for scoring, Raw is the full markdown fed to the
// model as grounding context.
type aiDoc struct {
	Title    string
	Route    string
	Category string
	Raw      string
	Plain    string // lowercased plain text, for scoring
}

// aiChatMessage is one turn in the OpenAI chat shape.
type aiChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type aiAskRequest struct {
	Messages []aiChatMessage `json:"messages"`
}

var aiStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true,
	"how": true, "does": true, "did": true, "can": true, "you": true, "are": true,
	"was": true, "but": true, "not": true, "has": true, "have": true, "from": true,
	"what": true, "when": true, "why": true, "where": true, "use": true, "using": true,
	"about": true, "into": true, "your": true, "gantry": true,
}

// aiTokenize lowercases and splits a query into scoring terms (>=3 chars,
// stopwords dropped).
func aiTokenize(s string) []string {
	var toks []string
	var b strings.Builder
	flush := func() {
		if b.Len() >= 3 {
			t := b.String()
			if !aiStopwords[t] {
				toks = append(toks, t)
			}
		}
		b.Reset()
	}
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return toks
}

// retrieve scores every page by term overlap with the query and returns the
// top k pages. Title hits weigh more than body hits. This is what lets a
// small local-model context window still draw on the whole knowledge base:
// only the most relevant pages are sent as context, per question.
func (s *docsSite) retrieve(query string, k int) []aiDoc {
	toks := aiTokenize(query)
	if len(toks) == 0 {
		return nil
	}
	type scored struct {
		d     aiDoc
		score int
	}
	var out []scored
	for _, d := range s.aiDocs {
		titleLower := strings.ToLower(d.Title)
		sc := 0
		for _, t := range toks {
			sc += strings.Count(d.Plain, t)
			sc += strings.Count(titleLower, t) * 8
		}
		if sc > 0 {
			out = append(out, scored{d, sc})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].score > out[j].score })
	if len(out) > k {
		out = out[:k]
	}
	res := make([]aiDoc, len(out))
	for i, o := range out {
		res[i] = o.d
	}
	return res
}

// aiSystemPrompt builds the grounding prompt: an index of EVERY page (so the
// model can route to any page by link) plus the full text of the retrieved
// pages.
func (s *docsSite) aiSystemPrompt(retrieved []aiDoc) string {
	var b strings.Builder
	b.WriteString(`You are the Gantry documentation assistant. Gantry is a Go framework for building native desktop (and mobile) apps whose UI is React/TSX with a Go backend, paired over one local websocket via a "paired file" convention (a <name>.go and <name>.tsx sharing a folder-path key, wired with usePaired).

Answer ONLY from the documentation provided below. Be concise and concrete, and prefer short code examples drawn from the docs. When you use a page, cite it as a markdown link to its route, e.g. [Dynamic routes](/ui/dynamic-routes). If the user pastes an error or stack trace, identify the likely cause and point them to the most relevant page (runtime/crash errors and gerr codes: /advanced/errors; Windows-specific issues: /advanced/win32-notes; the wire protocol: /advanced/protocol). If the documentation does not cover the question, say so plainly rather than guessing.

# Documentation index (every page - link to any of these by its route)
`)
	b.WriteString(s.aiTOC)
	b.WriteString("\n# Relevant pages (full text)\n")
	if len(retrieved) == 0 {
		b.WriteString("(no page strongly matched the question - use the index above to point the user to the closest page.)\n")
	}
	for _, d := range retrieved {
		fmt.Fprintf(&b, "\n---\n## %s  (%s)\n\n%s\n", d.Title, d.Route, d.Raw)
	}
	return b.String()
}

// aiTrimHistory keeps only user/assistant turns, last max, to bound context.
func aiTrimHistory(msgs []aiChatMessage, max int) []aiChatMessage {
	var conv []aiChatMessage
	for _, m := range msgs {
		if m.Role == "user" || m.Role == "assistant" {
			conv = append(conv, aiChatMessage{Role: m.Role, Content: m.Content})
		}
	}
	if len(conv) > max {
		conv = conv[len(conv)-max:]
	}
	return conv
}

// handleAsk streams an answer to the browser as Server-Sent Events. It
// retrieves the relevant pages for the latest user message, prepends them as
// a system prompt, then proxies an OpenAI-compatible streaming completion,
// forwarding only the content deltas (plus an up-front list of sources).
func (s *docsSite) handleAsk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req aiAskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	lastUser := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUser = req.Messages[i].Content
			break
		}
	}
	retrieved := s.retrieve(lastUser, 6)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, _ := w.(http.Flusher)
	sse := func(obj any) {
		b, _ := json.Marshal(obj)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}

	// Tell the widget which pages we pulled, up front.
	srcs := make([]map[string]string, len(retrieved))
	for i, d := range retrieved {
		srcs[i] = map[string]string{"title": d.Title, "route": d.Route}
	}
	sse(map[string]any{"sources": srcs})

	msgs := []aiChatMessage{{Role: "system", Content: s.aiSystemPrompt(retrieved)}}
	msgs = append(msgs, aiTrimHistory(req.Messages, 8)...)
	body, _ := json.Marshal(map[string]any{
		"model":    s.aiCfg.model,
		"messages": msgs,
		"stream":   true,
	})

	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.aiCfg.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		sse(map[string]string{"error": err.Error()})
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	if s.aiCfg.apiKey != "" {
		upReq.Header.Set("Authorization", "Bearer "+s.aiCfg.apiKey)
	}

	resp, err := http.DefaultClient.Do(upReq)
	if err != nil {
		sse(map[string]string{"error": "Could not reach a model at " + s.aiCfg.baseURL + ". Is your local model server running? (" + err.Error() + ")"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		sse(map[string]string{"error": fmt.Sprintf("Model server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))})
		return
	}

	// Parse the upstream OpenAI SSE stream and forward content deltas.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(line[len("data:"):])
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			sse(map[string]string{"delta": chunk.Choices[0].Delta.Content})
		}
	}
	sse(map[string]bool{"done": true})
}

// handleAIStatus reports whether a model server is answering, so the widget
// can show the model name or a "start a local model" hint.
func (s *docsSite) handleAIStatus(w http.ResponseWriter, r *http.Request) {
	reachable := false
	ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
	defer cancel()
	if req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.aiCfg.baseURL+"/models", nil); err == nil {
		if s.aiCfg.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+s.aiCfg.apiKey)
		}
		if resp, err := http.DefaultClient.Do(req); err == nil {
			resp.Body.Close()
			reachable = resp.StatusCode < 500
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"reachable": reachable,
		"model":     s.aiCfg.model,
		"baseURL":   s.aiCfg.baseURL,
	})
}
