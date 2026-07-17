package gantrytest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/B-Commissions/Gantry/gantrytest/report"
)

// The report data layer: the driver writes one report.LaunchRecord per
// launch into a run-scoped directory that survives artifact cleanup, so
// `gantry test` can build gantry_test_report.html for passing tests too
// (whose artifact dirs are deleted). See gantrytest/report for the
// schema and the CLI side of the merge.

// workerSeq hands each Launch a stable index for the report's "worker"
// column. Go's test runner exposes no worker id, so this is a simple
// monotonic count of launches - enough to tell parallel instances apart.
var workerSeq atomic.Int64

// runRecordDir is where per-launch records are written (a sibling of the
// per-test artifact dirs, never cleaned by finalize). `gantry test` sets
// GANTRY_TEST_RUN_DIR; a bare `go test` falls back to a .gantry-run dir
// under the artifact root so records still accumulate.
func runRecordDir(artifactRoot string) string {
	if d := os.Getenv("GANTRY_TEST_RUN_DIR"); d != "" {
		return d
	}
	if artifactRoot == "" {
		return ""
	}
	return filepath.Join(artifactRoot, ".gantry-run")
}

// attemptFromEnv reads the 1-based attempt index `gantry test` assigns
// under --retries. Zero means "not a retried run" - artifacts stay in
// the flat per-test dir, as they always have.
func attemptFromEnv() int {
	if v := os.Getenv("GANTRY_TEST_ATTEMPT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// testSourceLoc walks the stack for the caller inside a _test.go file -
// the app author's Launch call site - and reports it relative to appDir,
// for the report's file grouping and the "login_test.go:24" line label.
func testSourceLoc(appDir string) (file string, line int) {
	var pcs [32]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		f, more := frames.Next()
		if strings.HasSuffix(f.File, "_test.go") {
			return relToApp(appDir, f.File), f.Line
		}
		if !more {
			break
		}
	}
	return "", 0
}

// relToApp makes a source path relative to the app dir (forward slashes)
// so the report groups "tests/login_test.go" rather than an absolute
// build path; it falls back to the base name when the file is elsewhere.
func relToApp(appDir, file string) string {
	if appDir != "" {
		if rel, err := filepath.Rel(appDir, file); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(file)
}

// recordFailure stashes a structured failure for the report. The driver
// calls it from its waiting/assertion helpers just before failing the
// test, so the report can show the expect/got and a real stack instead
// of only the go-test output. First failure wins (the one that stopped
// the test); later teardown noise does not overwrite it.
func (a *App) recordFailure(f *report.Failure) {
	if a.failure == nil {
		a.failure = f
	}
}

// driverFailure records a structured failure for a driver-raised
// timeout/assertion so the report shows a clean expect/got panel instead
// of the go-test output. want/got may be empty when the helper only has a
// message. The location is the app author's call site.
func (a *App) driverFailure(message, want, got string) {
	file, line := testSourceLoc(a.appDir)
	loc := ""
	if file != "" {
		loc = filepath.Base(file)
		if line > 0 {
			loc += ":" + strconv.Itoa(line)
		}
	}
	a.recordFailure(&report.Failure{
		Message:  message,
		Want:     want,
		Got:      got,
		Location: loc,
		Stack:    captureStack(3),
	})
}

// captureStack builds a failure stack starting at the driver frame that
// raised it, skipping runtime and this package's plumbing frames but
// keeping the app author's test frames.
func captureStack(skip int) []report.StackFrame {
	var pcs [32]uintptr
	n := runtime.Callers(skip+1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	var out []report.StackFrame
	for {
		f, more := frames.Next()
		if f.Function != "" && !strings.HasPrefix(f.Function, "runtime.") && !strings.HasPrefix(f.Function, "testing.") {
			out = append(out, report.StackFrame{Func: shortFunc(f.Function), File: filepath.Base(f.File), Line: f.Line})
		}
		if !more || len(out) >= 12 {
			break
		}
	}
	return out
}

// shortFunc trims a fully-qualified func name to its package-local form
// for the stack display (github.com/.../gantrytest.(*Element).Click ->
// gantrytest.(*Element).Click).
func shortFunc(fn string) string {
	if i := strings.LastIndexByte(fn, '/'); i >= 0 {
		return fn[i+1:]
	}
	return fn
}

// writeRecord emits this launch's report.LaunchRecord at teardown. It is
// best-effort: a run without a records dir (or a write error) simply
// yields no report row from the driver side, and the CLI falls back to
// the go-test event for that test.
func (a *App) writeRecord() {
	dir := runRecordDir(a.cfg.artifactRoot)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	rec := report.LaunchRecord{
		Name:        a.t.Name(),
		Attempt:     a.attempt,
		Plane:       a.plane,
		Worker:      a.worker,
		File:        a.srcFile,
		Line:        a.srcLine,
		Started:     a.started.Format(time.RFC3339Nano),
		ArtifactDir: a.artifactRel(),
		Failure:     a.failure,
	}
	if rec.Attempt == 0 {
		rec.Attempt = 1
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	// A subtest name contains '/', and an attempt of the same test must
	// not clobber attempt 1's record: key the file on name + attempt.
	base := unsafePathChars.ReplaceAllString(a.t.Name(), "_")
	if a.attempt > 0 {
		base += ".attempt-" + strconv.Itoa(a.attempt)
	}
	_ = os.WriteFile(filepath.Join(dir, base+".json"), data, 0o644)
}

// artifactRel is this launch's artifact directory relative to the
// artifact root, for the record - empty when the dir was discarded
// (a passing, non-recorded, non-kept test).
func (a *App) artifactRel() string {
	if a.art == nil {
		return ""
	}
	if !a.t.Failed() && !a.art.keep {
		return "" // finalize will remove it
	}
	root := a.cfg.artifactRoot
	if root == "" {
		return filepath.Base(a.art.dir)
	}
	if rel, err := filepath.Rel(root, a.art.dir); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(a.art.dir)
}

// planeFor names the surface a launch drove, for the report badge.
func planeFor(onDevice, dom bool) report.Plane {
	switch {
	case onDevice:
		return report.PlaneDevice
	case dom:
		return report.PlaneDOM
	default:
		return report.PlaneNative
	}
}
