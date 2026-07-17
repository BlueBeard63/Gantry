package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "embed"

	"github.com/B-Commissions/Gantry/gantrytest/report"
)

func base64Std(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// This file turns a finished run into gantry_test_report.html - the
// self-contained viewer Jack designed (Run Overview + per-test Screencast
// / Screenshots / Trace). It merges two sources by test name: the
// go-test -json outcomes (authoritative status, elapsed, output) and the
// driver's per-launch report.LaunchRecord files (plane, worker, source
// location, artifacts, structured failure), inlines every artifact into
// one HTML file, and writes it to the artifact root.

//go:embed reportui/index.html
var reportIndexHTML string

//go:embed reportui/report.css
var reportCSS string

//go:embed reportui/report.js
var reportJS string

// how much of each log to inline, so a runaway app.log can't bloat the
// report to hundreds of MB.
const maxInlineLog = 512 * 1024

// maxTraceEntries caps the trace inlined per attempt; a chatty test can
// produce tens of thousands of frames and the viewer only needs enough
// to tell the story. Truncation is surfaced in the UI.
const maxTraceEntries = 8000

// maxInlineFrames caps the screencast frames inlined per attempt. An
// off-screen webview can emit dozens of near-identical frames per second
// during a wait; carrying every one as a base64 JPEG bloats the
// self-contained report. We keep a temporally even sample plus the final
// (failure) frame - plenty for a watchable scrub without the megabytes.
const maxInlineFrames = 90

// buildRunReport assembles the aggregated model. attempts maps a full
// test name to its attempts in order (index 0 == attempt 1); each
// attempt's outcome is the go-test result, nil only if the test never
// produced one (skipped before running).
func buildRunReport(artifactRoot, recordDir, command, startedAt string, duration float64, target string, attempts map[string][]*attemptOutcome) report.RunReport {
	records := loadRecords(recordDir)

	// One TestResult per test, grouped into files afterwards.
	var results []report.TestResult
	for name, outs := range attempts {
		tr := report.TestResult{Name: name, Plane: report.PlaneNative}
		for i, out := range outs {
			n := i + 1
			at := report.Attempt{N: n}
			if out != nil {
				at.Status = out.Status
				at.Duration = out.Elapsed
				at.Output = out.Output
			}
			if rec, ok := records[name][n]; ok {
				at.Worker = rec.Worker
				at.StartedAt = rec.Started
				at.Failure = rec.Failure
				tr.Plane = rec.Plane
				tr.File, tr.Line = rec.File, rec.Line
				loadArtifacts(&at, artifactRoot, rec.ArtifactDir)
			}
			tr.Attempts = append(tr.Attempts, at)
		}
		tr.Status = classify(tr.Attempts)
		if last := lastAttempt(tr.Attempts); last != nil {
			tr.Duration = last.Duration
			if tr.Status == report.StatusSkip {
				tr.SkipMsg = skipReason(last.Output)
			}
		}
		results = append(results, tr)
	}

	rep := report.RunReport{
		Command:   command,
		StartedAt: startedAt,
		Duration:  duration,
		Target:    target,
		Files:     groupByFile(results),
	}
	for _, tr := range results {
		rep.Counts.Total++
		switch tr.Status {
		case report.StatusPass:
			rep.Counts.Passed++
		case report.StatusFail:
			rep.Counts.Failed++
		case report.StatusFlaky:
			rep.Counts.Flaky++
		case report.StatusSkip:
			rep.Counts.Skipped++
		}
	}
	return rep
}

// classify collapses a test's attempts into a single status: passed
// first try -> pass; failed then later passed -> flaky; failed every
// time -> fail; skipped -> skip.
func classify(attempts []report.Attempt) report.Status {
	if len(attempts) == 0 {
		return report.StatusSkip
	}
	last := attempts[len(attempts)-1]
	switch last.Status {
	case report.StatusSkip:
		return report.StatusSkip
	case report.StatusPass:
		if len(attempts) > 1 && attempts[0].Status == report.StatusFail {
			return report.StatusFlaky
		}
		return report.StatusPass
	default:
		return report.StatusFail
	}
}

func lastAttempt(attempts []report.Attempt) *report.Attempt {
	if len(attempts) == 0 {
		return nil
	}
	return &attempts[len(attempts)-1]
}

// groupByFile buckets tests by their source file for the overview's
// collapsible groups, ordering files and tests by their first line.
func groupByFile(results []report.TestResult) []report.FileGroup {
	byFile := map[string]*report.FileGroup{}
	var order []string
	for _, tr := range results {
		key := tr.File
		if key == "" {
			key = "(unknown)"
		}
		g := byFile[key]
		if g == nil {
			g = &report.FileGroup{Path: key}
			byFile[key] = g
			order = append(order, key)
		}
		g.Tests = append(g.Tests, tr)
		g.Duration += tr.Duration
	}
	sort.Strings(order)
	var groups []report.FileGroup
	for _, key := range order {
		g := byFile[key]
		sort.SliceStable(g.Tests, func(i, j int) bool {
			if g.Tests[i].Line != g.Tests[j].Line {
				return g.Tests[i].Line < g.Tests[j].Line
			}
			return g.Tests[i].Name < g.Tests[j].Name
		})
		groups = append(groups, *g)
	}
	return groups
}

// loadRecords reads every LaunchRecord in the run-scoped dir and indexes
// it by test name and attempt, so the merge can find each attempt's
// driver-side detail.
func loadRecords(dir string) map[string]map[int]report.LaunchRecord {
	out := map[string]map[int]report.LaunchRecord{}
	if dir == "" {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var rec report.LaunchRecord
		if json.Unmarshal(data, &rec) != nil || rec.Name == "" {
			continue
		}
		n := rec.Attempt
		if n == 0 {
			n = 1
		}
		if out[rec.Name] == nil {
			out[rec.Name] = map[int]report.LaunchRecord{}
		}
		out[rec.Name][n] = rec
	}
	return out
}

// loadArtifacts inlines an attempt's output directory into the report:
// screenshots as data URIs, logs as text, trace.jsonl parsed, and the
// screencast decoded to distinct frames.
func loadArtifacts(at *report.Attempt, artifactRoot, rel string) {
	if rel == "" {
		return
	}
	at.Dir = filepath.ToSlash(filepath.Join("test-results", filepath.FromSlash(rel)))
	dir := filepath.Join(artifactRoot, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		fi, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, name)
		switch {
		case strings.HasSuffix(name, ".png"):
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			at.Artifacts = append(at.Artifacts, report.Artifact{
				Name: name, Kind: "image", Size: fi.Size(),
				Desc: screenshotDesc(name),
				Data: "data:image/png;base64," + base64Std(data),
			})
		case name == "screencast.avi":
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			at.Frames = thinFrames(report.DecodeAVI(data), maxInlineFrames)
			at.Artifacts = append(at.Artifacts, report.Artifact{
				Name: name, Kind: "video", Size: fi.Size(),
				Desc: "whole test as video (--record)",
			})
		case name == "trace.jsonl":
			entries, truncated := parseTrace(path)
			at.Trace = entries
			desc := fmt.Sprintf("%d protocol frames + driver actions", len(entries))
			if truncated {
				desc += " (truncated)"
			}
			at.Artifacts = append(at.Artifacts, report.Artifact{
				Name: name, Kind: "trace", Size: fi.Size(), Desc: desc,
			})
		case name == "app.log", name == "console.log", name == "crash.log":
			at.Artifacts = append(at.Artifacts, report.Artifact{
				Name: name, Kind: "log", Size: fi.Size(),
				Desc: logDesc(name),
				Data: readCapped(path),
			})
		}
	}
	sortArtifacts(at.Artifacts)
}

// thinFrames evenly downsamples a frame list to at most max, always
// keeping the first and last (the failure frame) so the scrub still
// starts and ends where the test did.
func thinFrames(frames []report.Frame, max int) []report.Frame {
	if len(frames) <= max || max < 2 {
		return frames
	}
	out := make([]report.Frame, 0, max)
	step := float64(len(frames)-1) / float64(max-1)
	for i := 0; i < max; i++ {
		idx := int(float64(i)*step + 0.5)
		if idx >= len(frames) {
			idx = len(frames) - 1
		}
		out = append(out, frames[idx])
	}
	// Guarantee the true last frame is present.
	out[len(out)-1] = frames[len(frames)-1]
	return out
}

func screenshotDesc(name string) string {
	switch name {
	case "failure.png":
		return "screen at the moment of failure"
	default:
		base := strings.TrimSuffix(name, ".png")
		return "app.Screenshot(\"" + base + "\")"
	}
}

func logDesc(name string) string {
	switch name {
	case "app.log":
		return "app process stdout + stderr"
	case "console.log":
		return "webview console + uncaught JS"
	case "crash.log":
		return "the runtime's fatal-panic trace"
	}
	return ""
}

// sortArtifacts orders the sidebar: video, screenshots (failure first),
// trace, logs - matching the design's grouping.
func sortArtifacts(as []report.Artifact) {
	rank := func(a report.Artifact) int {
		switch a.Kind {
		case "video":
			return 0
		case "image":
			if a.Name == "failure.png" {
				return 1
			}
			return 2
		case "trace":
			return 3
		default:
			return 4
		}
	}
	sort.SliceStable(as, func(i, j int) bool {
		if ri, rj := rank(as[i]), rank(as[j]); ri != rj {
			return ri < rj
		}
		return as[i].Name < as[j].Name
	})
}

// parseTrace reads trace.jsonl into the viewer's entries, decoding each
// wire frame's summary fields and computing a time relative to the first
// line so the trace timeline lines up with the screencast.
func parseTrace(path string) (entries []report.TraceEntry, truncated bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var base time.Time
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(entries) >= maxTraceEntries {
			truncated = true
			break
		}
		var raw struct {
			Time  string          `json:"time"`
			Dir   string          `json:"dir"`
			Frame json.RawMessage `json:"frame"`
			Msg   string          `json:"msg"`
		}
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}
		e := report.TraceEntry{Time: raw.Time, Dir: raw.Dir, Msg: raw.Msg}
		if t, err := time.Parse(time.RFC3339Nano, raw.Time); err == nil {
			if base.IsZero() {
				base = t
			}
			e.T = t.Sub(base).Seconds()
		}
		if len(raw.Frame) > 0 {
			e.Frame = decodeTraceFrame(raw.Frame)
			e.Raw = prettyJSON(raw.Frame)
		}
		entries = append(entries, e)
	}
	return entries, truncated
}

// decodeTraceFrame pulls the summary and filter fields out of a raw wire
// frame (the client.go frame/wireMsg shapes) for the trace table.
func decodeTraceFrame(raw json.RawMessage) *report.TraceFrame {
	var f struct {
		T    string          `json:"t"`
		Seq  int             `json:"seq"`
		Key  string          `json:"key"`
		Name string          `json:"name"`
		Code string          `json:"code"`
		Tree json.RawMessage `json:"tree"`
	}
	if json.Unmarshal(raw, &f) != nil {
		return nil
	}
	tf := &report.TraceFrame{T: f.T, Seq: f.Seq, Key: f.Key, Name: f.Name, Code: f.Code}
	if len(f.Tree) > 0 {
		tf.Tree = prettyJSON(f.Tree)
	}
	return tf
}

func prettyJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}

// readCapped reads a log, truncating to maxInlineLog with a marker so a
// giant app.log cannot bloat the report.
func readCapped(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(data) > maxInlineLog {
		return string(data[:maxInlineLog]) + "\n… (truncated in the report; full file on disk)\n"
	}
	return string(data)
}

// writeReport renders the self-contained HTML and writes it to
// <artifactRoot>/gantry_test_report.html, returning the path.
func writeReport(rep report.RunReport, artifactRoot string) (string, error) {
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return "", err
	}
	// Default HTML escaping turns <, >, & into \uXXXX, so the JSON is safe
	// to embed inside a <script> element.
	data, err := json.Marshal(rep)
	if err != nil {
		return "", err
	}
	html := reportIndexHTML
	html = strings.Replace(html, "/*__GANTRY_CSS__*/", reportCSS, 1)
	html = strings.Replace(html, "__GANTRY_DATA__", string(data), 1)
	html = strings.Replace(html, "/*__GANTRY_JS__*/", reportJS, 1)
	out := filepath.Join(artifactRoot, "gantry_test_report.html")
	if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
		return "", err
	}
	return out, nil
}
