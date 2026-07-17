// Package gerr is Gantry's structured error type: a normal Go error
// carrying a stable, greppable code ("config.unknown-arg",
// "panic.call") and an optional hint for the person reading it. Codes
// are named category-prefixed strings declared as constants next to
// their producers; the full table lives in docs/advanced/errors.md.
package gerr

import (
	"errors"
	"fmt"
)

// E is a coded error. It wraps like any other error (errors.Is/As walk
// through it), so callers only ever need CodeOf to read the code back.
type E struct {
	Code string // stable "category.name" identifier
	Msg  string // human message, no code embedded
	Hint string // optional "what to do about it" line
	Err  error  // wrapped cause, if any
}

// New builds a coded error.
func New(code, format string, a ...any) *E {
	return &E{Code: code, Msg: fmt.Sprintf(format, a...)}
}

// Wrap builds a coded error around a cause.
func Wrap(code string, err error, format string, a ...any) *E {
	return &E{Code: code, Msg: fmt.Sprintf(format, a...), Err: err}
}

// WithHint attaches a "what to do about it" line and returns the error.
func (e *E) WithHint(format string, a ...any) *E {
	e.Hint = fmt.Sprintf(format, a...)
	return e
}

func (e *E) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v (%s)", e.Msg, e.Err, e.Code)
	}
	return fmt.Sprintf("%s (%s)", e.Msg, e.Code)
}

func (e *E) Unwrap() error { return e.Err }

// CodeOf returns the code of the first *E in err's chain, or "".
func CodeOf(err error) string {
	var e *E
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// HintOf returns the hint of the first *E in err's chain, or "".
func HintOf(err error) string {
	var e *E
	if errors.As(err, &e) {
		return e.Hint
	}
	return ""
}
