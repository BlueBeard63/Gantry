package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The gantry skill is scaffolded into every app from
// templates/agents/SKILL.md.tmpl; a working checkout usually also keeps
// untracked copies at the repo root (.claude/skills/gantry/,
// .agents/skills/gantry/ - both dirs are gitignored) so agents working
// on Gantry itself load it. The template deliberately has no [[ ]]
// substitutions so the rendered skill is byte-identical everywhere -
// this test keeps any local copies from drifting from the template
// (edit the template, then copy it over both). Absent copies (fresh
// clone, CI) are skipped, not failed.
func TestGantrySkillCopiesMatchTemplate(t *testing.T) {
	rendered, err := renderBytes("agents/SKILL.md.tmpl", scaffold{})
	if err != nil {
		t.Fatalf("rendering skill template: %v", err)
	}
	want := normalizeEOL(rendered)

	for _, copy := range []string{
		filepath.Join("..", "..", ".claude", "skills", "gantry", "SKILL.md"),
		filepath.Join("..", "..", ".agents", "skills", "gantry", "SKILL.md"),
	} {
		got, err := os.ReadFile(copy)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Errorf("reading %s: %v", copy, err)
			continue
		}
		if normalizeEOL(got) != want {
			t.Errorf("%s differs from templates/agents/SKILL.md.tmpl - update the template and copy it over the checked-in file", copy)
		}
	}
}
