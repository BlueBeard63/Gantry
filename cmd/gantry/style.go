package main

import (
	"fmt"
	"os"

	"github.com/BlueBeard63/Gantry/gerr"
	"github.com/charmbracelet/lipgloss"
)

// Colour for every CLI message goes through these helpers. stdout and
// stderr get their own renderers because either can be piped on its
// own; lipgloss degrades to plain text by itself when the writer is
// not a TTY or NO_COLOR is set.
var (
	stdoutStyles = lipgloss.NewRenderer(os.Stdout)
	stderrStyles = lipgloss.NewRenderer(os.Stderr)

	prefixStyle  = stdoutStyles.NewStyle().Foreground(lipgloss.Color("245"))
	stepStyle    = stdoutStyles.NewStyle().Foreground(lipgloss.Color("39"))
	successStyle = stdoutStyles.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle    = stderrStyles.NewStyle().Foreground(lipgloss.Color("214"))
	failStyle    = stderrStyles.NewStyle().Foreground(lipgloss.Color("196"))
	promptQStyle = stdoutStyles.NewStyle().Bold(true)
	hintStyle    = stdoutStyles.NewStyle().Foreground(lipgloss.Color("245"))
	errHintStyle = stderrStyles.NewStyle().Foreground(lipgloss.Color("245"))
)

// step prints a blue progress line: what the CLI is doing right now.
func step(format string, a ...any) {
	fmt.Println(prefixStyle.Render("gantry:") + " " + stepStyle.Render(fmt.Sprintf(format, a...)))
}

// info prints a neutral notice.
func info(format string, a ...any) {
	fmt.Println(prefixStyle.Render("gantry:") + " " + fmt.Sprintf(format, a...))
}

// success prints a green completion line.
func success(format string, a ...any) {
	fmt.Println(prefixStyle.Render("gantry:") + " " + successStyle.Render(fmt.Sprintf(format, a...)))
}

// warn prints an orange notice to stderr: something was skipped or
// degraded, never fatal.
func warn(format string, a ...any) {
	fmt.Fprintln(os.Stderr, warnStyle.Render("gantry: "+fmt.Sprintf(format, a...)))
}

// fail prints a red error line to stderr; exit codes stay with the
// caller.
func fail(format string, a ...any) {
	fmt.Fprintln(os.Stderr, failStyle.Render("gantry: "+fmt.Sprintf(format, a...)))
}

// failErr prints a command's final error: the message (a gerr code
// rides inside it), plus the hint line when the error carries one.
func failErr(err error) {
	fail("%v", err)
	if hint := gerr.HintOf(err); hint != "" {
		fmt.Fprintln(os.Stderr, errHintStyle.Render("  hint: "+hint))
	}
	if code := gerr.CodeOf(err); code != "" {
		fmt.Fprintln(os.Stderr, errHintStyle.Render("  see: gantry docs errors ("+code+")"))
	}
}

// promptStr returns a styled wizard question ("Question? [Y/n] ") for
// the caller to print, keeping the cursor on the same line for input.
func promptStr(q, hint string) string {
	return promptQStyle.Render(q) + " " + hintStyle.Render("["+hint+"]") + " "
}
