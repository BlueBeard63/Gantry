package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// checkSafe asserts the invariants that keep a frame from tripping the
// Windows Terminal full-width auto-wrap bug: no row reaches the right edge,
// no row carries trailing padding, and the frame fits under the height.
func checkSafe(t *testing.T, label string, frame string, w, h int) {
	t.Helper()
	lines := strings.Split(frame, "\n")
	if h > 1 && len(lines) > h-1 {
		t.Errorf("%s: %d rows exceeds h-1=%d", label, len(lines), h-1)
	}
	limit := w - 1
	if limit < 1 {
		limit = 1 // a 1-column terminal still needs one usable cell
	}
	for i, ln := range lines {
		if wln := lipgloss.Width(ln); wln > limit {
			t.Errorf("%s: row %d width %d exceeds limit=%d", label, i, wln, limit)
		}
		clean := ansi.Strip(ln)
		if clean != strings.TrimRight(clean, " \t") {
			t.Errorf("%s: row %d has trailing padding: %q", label, i, clean)
		}
	}
}

func TestSafeFrameInvariants(t *testing.T) {
	// A pathological input: over-wide lines, full-width padding hidden
	// before ANSI resets, and more rows than the height.
	pad := "\x1b[38;5;212mcolored\x1b[0m" + strings.Repeat(" ", 300) + "\x1b[0m"
	frame := strings.Join([]string{
		strings.Repeat("x", 500),
		pad,
		"short",
		"a\x1b[0m   ",
		strings.Repeat("y", 40),
	}, "\n")
	for _, s := range []struct{ w, h int }{{10, 3}, {40, 4}, {80, 6}, {188, 10}, {1, 1}} {
		checkSafe(t, "safeFrame", safeFrame(frame, s.w, s.h), s.w, s.h)
	}
}

func TestDocsRenderInvariants(t *testing.T) {
	pages, err := loadDocs()
	if err != nil {
		t.Fatalf("loadDocs: %v", err)
	}
	if len(pages) < 10 {
		t.Fatalf("expected embedded docs, got %d", len(pages))
	}
	sizes := []struct{ w, h int }{
		{188, 41}, {120, 30}, {80, 24}, {60, 20}, {49, 15}, {40, 24}, {20, 8}, {10, 3},
	}
	for _, s := range sizes {
		var m tea.Model = newDocsModel(pages, 0)
		m, _ = m.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
		checkSafe(t, "render", m.View(), s.w, s.h)
	}
}

func TestDocsResponsiveCollapse(t *testing.T) {
	pages, _ := loadDocs()
	// Wide: two panes (sidebar > 0). Narrow: collapsed (sidebar == 0).
	wide := newDocsModel(pages, 0)
	wide.width, wide.height = 120, 30
	if wide.sidebarWidth() == 0 {
		t.Errorf("wide terminal should show a sidebar")
	}
	narrow := newDocsModel(pages, 0)
	narrow.width, narrow.height = 40, 20
	if narrow.sidebarWidth() != 0 {
		t.Errorf("narrow terminal should collapse the sidebar, got %d", narrow.sidebarWidth())
	}
}

func TestTruncRunes(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{"hello", 3, "hel"},
		{"hi", 5, "hi"},
		{"héllo", 3, "hél"}, // rune-safe: does not split the multibyte é
		{"x", -1, ""},
		{"", 4, ""},
	}
	for _, c := range cases {
		if got := truncRunes(c.in, c.w); got != c.want {
			t.Errorf("truncRunes(%q,%d) = %q, want %q", c.in, c.w, got, c.want)
		}
	}
}
