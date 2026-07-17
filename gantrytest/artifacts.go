package gantrytest

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"
)

// artifacts owns one test's output directory under the artifact root
// (test-results/<TestName>/ by default): app.log, trace.jsonl and a
// crash.log copy when the process died. Passing tests keep nothing
// unless keep is set (gantry test --keep-artifacts).
type artifacts struct {
	dir  string
	keep bool

	appLog *syncWriter
	trace  *trace
}

var unsafePathChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// newArtifacts creates a fresh directory for the test, replacing any
// leftovers from a previous run.
func newArtifacts(t testing.TB, root string, keep bool) *artifacts {
	t.Helper()
	dir := filepath.Join(root, unsafePathChars.ReplaceAllString(t.Name(), "_"))
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("gantrytest: clearing artifact dir %s: %v", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("gantrytest: creating artifact dir %s: %v", dir, err)
	}
	a := &artifacts{dir: dir, keep: keep}
	a.appLog = newSyncWriter(t, filepath.Join(dir, "app.log"))
	a.trace = newTrace(t, filepath.Join(dir, "trace.jsonl"))
	return a
}

// saveFile copies an external file (crash.log) into the artifact dir
// when it exists and is non-empty.
func (a *artifacts) saveFile(name, srcPath string) {
	data, err := os.ReadFile(srcPath)
	if err != nil || len(data) == 0 {
		return
	}
	_ = os.WriteFile(filepath.Join(a.dir, name), data, 0o644)
}

// finalize closes the writers and removes the directory when the test
// passed and artifacts are not being kept.
func (a *artifacts) finalize(t testing.TB) {
	a.appLog.Close()
	a.trace.Close()
	if !t.Failed() && !a.keep {
		_ = os.RemoveAll(a.dir)
		return
	}
	if t.Failed() {
		t.Logf("gantrytest: artifacts in %s", a.dir)
	}
}

// syncWriter serializes writes from the app's stdout and stderr into
// one log file.
type syncWriter struct {
	mu sync.Mutex
	f  *os.File
}

func newSyncWriter(t testing.TB, path string) *syncWriter {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("gantrytest: creating %s: %v", path, err)
	}
	return &syncWriter{f: f}
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return len(p), nil
	}
	return w.f.Write(p)
}

func (w *syncWriter) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
}

var _ io.Writer = (*syncWriter)(nil)

// trace is the structured test recording: every protocol frame in and
// out plus every driver action, as JSON lines with timestamps - the
// "what led here" of a test run, mirroring the app's breadcrumb trail.
type trace struct {
	mu sync.Mutex
	f  *os.File
}

// traceLine is one trace.jsonl record.
type traceLine struct {
	Time  string          `json:"time"`
	Dir   string          `json:"dir"` // "send" | "recv" | "action"
	Frame json.RawMessage `json:"frame,omitempty"`
	Msg   string          `json:"msg,omitempty"`
}

func newTrace(t testing.TB, path string) *trace {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("gantrytest: creating %s: %v", path, err)
	}
	return &trace{f: f}
}

func (tr *trace) frame(dir string, raw []byte) {
	tr.write(traceLine{Time: time.Now().Format(time.RFC3339Nano), Dir: dir, Frame: raw})
}

func (tr *trace) action(format string, args ...any) {
	tr.write(traceLine{Time: time.Now().Format(time.RFC3339Nano), Dir: "action", Msg: fmt.Sprintf(format, args...)})
}

func (tr *trace) write(line traceLine) {
	data, err := json.Marshal(line)
	if err != nil {
		return
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.f == nil {
		return
	}
	_, _ = tr.f.Write(append(data, '\n'))
}

func (tr *trace) Close() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.f != nil {
		_ = tr.f.Close()
		tr.f = nil
	}
}
