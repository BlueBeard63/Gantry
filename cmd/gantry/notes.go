package main

// upgradeNotes are per-release remarks gantry upgrade prints when an
// app crosses that version: things the automatic steps cannot fully
// cover, or that deserve a heads-up. Keep each note to a line or two.
var upgradeNotes = []struct {
	version string // bare X.Y.Z
	note    string
}{
	{"0.3.4", "gantry-web is prebundled now and createApp takes the virtual:gantry-app module as its first argument. .gantry/ and the package pin are updated automatically; only custom (without-the-CLI) builds must update their main.tsx by hand - see docs/advanced/without-the-cli.md."},
}

// printUpgradeNotes prints the notes for every release in (from, to].
// An empty from (app predates version recording) includes everything
// up to the target.
func printUpgradeNotes(from, to string) {
	for _, n := range upgradeNotes {
		if from != "" && !semverLess(from, n.version) {
			continue
		}
		if semverLess(to, n.version) {
			continue
		}
		info("%s: %s", n.version, n.note)
	}
}
