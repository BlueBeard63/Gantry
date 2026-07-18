package main

import "testing"

// TestDocsLayoutScaling exercises the sidebar/content layout math across
// terminal sizes, including the tiny ones that used to yield a zero or
// negative viewport and a garbled sidebar.
func TestDocsLayoutScaling(t *testing.T) {
	sizes := []struct{ w, h int }{
		{0, 0}, {1, 1}, {10, 3}, {31, 5}, {40, 10}, {49, 20}, {50, 24}, {80, 24}, {200, 60},
	}
	for _, s := range sizes {
		m := &docsModel{width: s.w, height: s.h}
		side := m.sidebarWidth()
		cw, ch := m.contentSize()

		if cw < docsMinContent {
			t.Errorf("w=%d,h=%d: content width %d below floor %d", s.w, s.h, cw, docsMinContent)
		}
		if ch < 1 {
			t.Errorf("w=%d,h=%d: content height %d < 1", s.w, s.h, ch)
		}
		if side < 0 {
			t.Errorf("w=%d: sidebar width %d negative", s.w, side)
		}
		if s.w < docsCollapseBelow && side != 0 {
			t.Errorf("w=%d: expected collapsed sidebar, got %d", s.w, side)
		}
		// When the sidebar shows, it plus its border and the spacer must
		// still leave the content its floor - never overflow the terminal.
		if side > 0 && side+3+docsMinContent > s.w {
			t.Errorf("w=%d: sidebar %d + 3 + content floor overflows width", s.w, side)
		}
		if w := m.sidebarTextWidth(); w < 1 {
			t.Errorf("w=%d: sidebar text width %d < 1", s.w, w)
		}
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
