// Package report is the shared data model for Gantry's test report - the
// gantry_test_report.html viewer. It is imported by two sides that must
// agree on the schema without depending on each other: the gantrytest
// driver, which writes one LaunchRecord per (test, attempt) as a run
// happens, and the gantry CLI's `test` command, which merges those
// records with `go test -json` into a RunReport and renders the HTML.
//
// This package deliberately imports nothing from gantrytest (which
// imports it) so there is no cycle.
package report

// Plane is the surface a test drove.
type Plane string

const (
	PlaneNative Plane = "native" // protocol plane only (headless desktop)
	PlaneDOM    Plane = "dom"    // the DOM/CDP plane was attached (WithDOM)
	PlaneDevice Plane = "device" // a device target (android)
)

// Status is a test's outcome across all of its attempts in one run.
type Status string

const (
	StatusPass  Status = "pass"  // passed on the first attempt
	StatusFail  Status = "fail"  // failed every attempt
	StatusFlaky Status = "flaky" // failed then passed on a retry
	StatusSkip  Status = "skip"  // t.Skip / an unmet target requirement
)

// LaunchRecord is what the driver writes for one launch (one test, one
// attempt) into the run-scoped records dir (GANTRY_TEST_RUN_DIR). It
// carries everything `go test -json` cannot see: which plane the test
// drove, where its source lives, which worker ran it, and - for a
// failure the driver itself raised - a structured Failure. The CLI
// merges these with the go-test event stream by (Name, Attempt).
type LaunchRecord struct {
	Name        string   `json:"name"`        // t.Name(), e.g. "TestLogin/valid_credentials"
	Attempt     int      `json:"attempt"`     // 1-based; >1 only under --retries
	Plane       Plane    `json:"plane"`       // native | dom | device
	Worker      int      `json:"worker"`      // the launch's worker index
	File        string   `json:"file"`        // test source, relative to the app dir
	Line        int      `json:"line"`        // the Launch call site
	Started     string   `json:"started"`     // RFC3339Nano
	ArtifactDir string   `json:"artifactDir"` // relative to the artifact root ("" if discarded)
	Failure     *Failure `json:"failure,omitempty"`
}

// Failure is a driver-raised assertion failure, captured structurally at
// the point the driver gave up so the report can show the expect/got and
// a real stack rather than only the go-test output text. Author-side
// t.Errorf failures do not populate this - they fall back to the go-test
// output on the aggregated TestResult.
type Failure struct {
	Message  string       `json:"message"`            // the one-line failure
	Want     string       `json:"want,omitempty"`     // expected, when the helper knows it
	Got      string       `json:"got,omitempty"`      // actual, when the helper knows it
	Location string       `json:"location,omitempty"` // file:line the assertion is on
	Stack    []StackFrame `json:"stack,omitempty"`
}

// StackFrame is one entry of a captured failure stack.
type StackFrame struct {
	Func string `json:"func"`
	File string `json:"file"`
	Line int    `json:"line"`
}

// ---- the aggregated model the CLI builds and the HTML renders ----

// RunReport is the whole run: the command, when it ran, how long it
// took, the tallied counts and the tests grouped by source file. It is
// what the generator inlines into gantry_test_report.html.
type RunReport struct {
	Command   string      `json:"command"`   // e.g. "gantry test ./e2e --record"
	StartedAt string      `json:"startedAt"` // RFC3339
	Duration  float64     `json:"duration"`  // seconds
	Target    string      `json:"target"`    // desktop | android
	Counts    Counts      `json:"counts"`
	Files     []FileGroup `json:"files"`
}

// Counts is the status tally shown in the overview's filter chips.
type Counts struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Flaky   int `json:"flaky"`
	Skipped int `json:"skipped"`
}

// FileGroup is one collapsible source-file group in the overview.
type FileGroup struct {
	Path     string       `json:"path"`     // e.g. "e2e/login_test.go"
	Duration float64      `json:"duration"` // seconds, summed over its tests
	Tests    []TestResult `json:"tests"`
}

// TestResult is one test's outcome, with per-attempt detail and the
// artifacts of its final attempt inlined for the self-contained report.
type TestResult struct {
	Name     string    `json:"name"`     // full name incl. subtest path
	Status   Status    `json:"status"`   // pass | fail | flaky | skip
	Plane    Plane     `json:"plane"`    // native | dom | device
	File     string    `json:"file"`     // relative to the app dir
	Line     int       `json:"line"`     // the Launch call site
	Duration float64   `json:"duration"` // seconds, the final attempt
	SkipMsg  string    `json:"skipMsg,omitempty"`
	Attempts []Attempt `json:"attempts"`
}

// Attempt is one run of a test - more than one only under --retries.
type Attempt struct {
	N         int          `json:"n"`      // 1-based
	Status    Status       `json:"status"` // pass | fail | skip
	Worker    int          `json:"worker"`
	StartedAt string       `json:"startedAt"`
	Duration  float64      `json:"duration"`
	Dir       string       `json:"dir,omitempty"`     // artifact output dir, relative to the run
	Failure   *Failure     `json:"failure,omitempty"` // driver-raised, when present
	Output    string       `json:"output,omitempty"`  // go-test output for this attempt
	Artifacts []Artifact   `json:"artifacts,omitempty"`
	Trace     []TraceEntry `json:"trace,omitempty"`  // parsed trace.jsonl
	Frames    []Frame      `json:"frames,omitempty"` // distinct screencast frames
}

// Artifact is one file in an attempt's output directory, inlined (Data)
// when it is small enough to carry in the self-contained report.
type Artifact struct {
	Name string `json:"name"` // filename, e.g. "failure.png"
	Kind string `json:"kind"` // image | video | log | trace
	Size int64  `json:"size"` // bytes on disk
	Desc string `json:"desc"` // human note shown in the sidebar
	// Data is the inlined content: a data: URI for images, plain text for
	// logs. Empty for artifacts represented some other way (screencast
	// frames live on Attempt.Frames; trace on Attempt.Trace).
	Data string `json:"data,omitempty"`
}

// TraceEntry is one parsed line of trace.jsonl for the trace viewer.
type TraceEntry struct {
	Time  string      `json:"time"`            // RFC3339Nano
	T     float64     `json:"t"`               // seconds since the launch action
	Dir   string      `json:"dir"`             // send | recv | action
	Msg   string      `json:"msg,omitempty"`   // action text
	Frame *TraceFrame `json:"frame,omitempty"` // decoded wire frame (send/recv)
	Raw   string      `json:"raw,omitempty"`   // the raw frame JSON, pretty
}

// TraceFrame is the decoded wire frame carried by a send/recv trace
// entry - only the fields the viewer summarizes and filters on.
type TraceFrame struct {
	T    string `json:"t"` // frame type: render|state|error|reply|push|ready|event|call|setstate
	Seq  int    `json:"seq,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
	Code string `json:"code,omitempty"`
	// Tree is the raw Tea render tree JSON, present on render frames, so
	// the viewer can offer the raw/tree toggle without a second parse.
	Tree string `json:"tree,omitempty"`
}

// Frame is one distinct screencast frame for the canvas player: a JPEG
// as a data: URI and the time it should appear, in seconds from the
// start of the recording.
type Frame struct {
	Data string  `json:"data"` // "data:image/jpeg;base64,..."
	T    float64 `json:"t"`    // seconds
}
