package main

import (
	"fmt"
	"strings"
)

// unifiedDiff renders a small unified diff (2 context lines) between
// have and want, for gantry upgrade's per-file confirmation. Output is
// capped so a fully rewritten file cannot flood the terminal. Inputs
// are compared by line after CRLF normalization.
func unifiedDiff(name string, have, want []byte, maxLines int) string {
	a := splitLines(string(have))
	b := splitLines(string(want))

	// LCS table; scaffold files are small so O(n*m) is fine.
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}

	// Walk the table into an edit script: ' ' keep, '-' delete, '+' add.
	type edit struct {
		op   byte
		line string
	}
	var script []edit
	for i, j := 0, 0; i < n || j < m; {
		switch {
		case i < n && j < m && a[i] == b[j]:
			script = append(script, edit{' ', a[i]})
			i++
			j++
		case i < n && (j == m || lcs[i+1][j] >= lcs[i][j+1]):
			script = append(script, edit{'-', a[i]})
			i++
		default:
			script = append(script, edit{'+', b[j]})
			j++
		}
	}

	// Emit changed regions with 2 lines of context, eliding the rest.
	const ctx = 2
	keepAt := make([]bool, len(script))
	for i, e := range script {
		if e.op == ' ' {
			continue
		}
		for j := max(0, i-ctx); j <= min(len(script)-1, i+ctx); j++ {
			keepAt[j] = true
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s (current)\n+++ %s (template)\n", name, name)
	lines, elided := 0, false
	for i, e := range script {
		if !keepAt[i] {
			if !elided {
				out.WriteString("  ...\n")
				elided = true
			}
			continue
		}
		elided = false
		if lines >= maxLines {
			fmt.Fprintf(&out, "  ... (diff truncated at %d lines)\n", maxLines)
			break
		}
		out.WriteByte(e.op)
		out.WriteByte(' ')
		out.WriteString(e.line)
		out.WriteByte('\n')
		lines++
	}
	return out.String()
}

// splitLines splits normalized text into lines without a trailing
// phantom line for the final newline.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// normalizeEOL maps CRLF to LF so git's line-ending conversion never
// reads as a template change.
func normalizeEOL(b []byte) string {
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}
