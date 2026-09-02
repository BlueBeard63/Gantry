package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/BlueBeard63/Gantry/gantrytest/report"
	"github.com/charmbracelet/lipgloss"
)

// goTestEvent is one record from `go test -json` (the shape of
// testing's JSON output). Only the fields we consume are declared.
type goTestEvent struct {
	Action  string  `json:"Action"`  // run|pause|cont|pass|fail|skip|output|start
	Test    string  `json:"Test"`    // "" for package-level events
	Elapsed float64 `json:"Elapsed"` // seconds, on pass/fail/skip
	Output  string  `json:"Output"`
}

// attemptOutcome is a test's result from one `go test` invocation - one
// attempt. The report merges this (authoritative status/elapsed/output)
// with the driver's LaunchRecord (plane, artifacts, structured failure).
type attemptOutcome struct {
	Name    string
	Status  report.Status // pass | fail | skip
	Elapsed float64
	Output  string
}

var (
	passMark     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	failMark     = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("✗")
	skipMark     = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("–")
	testDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// runGoTest runs one `go test -json` invocation, streaming a readable
// per-test line to the terminal as results land and accumulating the
// structured outcome of every test. It returns the outcomes keyed by
// full test name (including subtest path). A non-nil error means the go
// tool itself failed to run (not a test failure); test failures are
// reported through the outcomes' Status.
func runGoTest(dir string, env, goArgs []string, verbose bool) (map[string]*attemptOutcome, error) {
	args := append([]string{"test", "-json"}, goArgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr // build errors and go-tool diagnostics
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	outcomes := map[string]*attemptOutcome{}
	buffers := map[string]*strings.Builder{}
	get := func(name string) *attemptOutcome {
		o := outcomes[name]
		if o == nil {
			o = &attemptOutcome{Name: name}
			outcomes[name] = o
			buffers[name] = &strings.Builder{}
		}
		return o
	}

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // render trees can be large
	for sc.Scan() {
		var ev goTestEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue // non-JSON line (rare); skip
		}
		if ev.Test == "" {
			// Package-level output (build failures, final FAIL line) is
			// worth showing verbatim, but skip the redundant summary lines.
			if ev.Action == "output" && verbose {
				fmt.Print(ev.Output)
			}
			continue
		}
		o := get(ev.Test)
		switch ev.Action {
		case "output":
			buffers[ev.Test].WriteString(ev.Output)
		case "pass":
			o.Status, o.Elapsed = report.StatusPass, ev.Elapsed
			o.Output = buffers[ev.Test].String()
			printTestLine(passMark, ev.Test, ev.Elapsed, "")
		case "skip":
			o.Status, o.Elapsed = report.StatusSkip, ev.Elapsed
			o.Output = buffers[ev.Test].String()
			printTestLine(skipMark, ev.Test, ev.Elapsed, skipReason(o.Output))
		case "fail":
			o.Status, o.Elapsed = report.StatusFail, ev.Elapsed
			o.Output = buffers[ev.Test].String()
			printTestLine(failMark, ev.Test, ev.Elapsed, "")
			printIndented(o.Output)
		}
	}
	// Wait always; a test failure surfaces as a non-zero exit we translate
	// to nil here (the outcomes carry the real status) and only a genuine
	// tooling error is returned.
	waitErr := cmd.Wait()
	if scErr := sc.Err(); scErr != nil && scErr != io.EOF {
		return outcomes, scErr
	}
	if waitErr != nil {
		if _, ok := waitErr.(*exec.ExitError); !ok {
			return outcomes, waitErr
		}
	}
	return outcomes, nil
}

func printTestLine(mark, name string, elapsed float64, note string) {
	line := fmt.Sprintf("  %s %s %s", mark, name, testDimStyle.Render(fmt.Sprintf("(%.2fs)", elapsed)))
	if note != "" {
		line += " " + testDimStyle.Render(note)
	}
	fmt.Println(line)
}

// printIndented re-prints a failed test's captured output, indented, so
// the terminal still shows why it failed even though we consumed -json.
func printIndented(out string) {
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		fmt.Println("      " + testDimStyle.Render(strings.TrimRight(ln, "\r")))
	}
}

// skipReason pulls the first "--- SKIP" / t.Skip message out of a test's
// output for a compact one-line note.
func skipReason(out string) string {
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if i := strings.Index(ln, ": "); i >= 0 && (strings.Contains(ln, "_test.go:") || strings.Contains(ln, ".go:")) {
			msg := strings.TrimSpace(ln[i+2:])
			if msg != "" {
				return msg
			}
		}
	}
	return ""
}

// runPattern builds a `go test -run` anchored regexp that matches
// exactly one full test path (including subtests), for re-running a
// single failed test under --retries. "TestLogin/session_persists" ->
// "^TestLogin$/^session_persists$".
func runPattern(name string) string {
	parts := strings.Split(name, "/")
	for i, p := range parts {
		parts[i] = "^" + quoteRunMeta(p) + "$"
	}
	return strings.Join(parts, "/")
}

// quoteRunMeta escapes regexp metacharacters in a test-path element.
func quoteRunMeta(s string) string {
	const meta = `\.+*?()|[]{}^$`
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(meta, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// sortedTestNames returns the failed tests of an attempt in a stable
// order, so retries run deterministically.
func sortedFailedNames(outcomes map[string]*attemptOutcome) []string {
	var names []string
	for name, o := range outcomes {
		if o.Status == report.StatusFail {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
