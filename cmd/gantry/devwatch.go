package main

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// generatedGoFiles are rewritten by regenForReload, so the watcher must
// ignore them - otherwise every rebuild would bump their mtime and
// trigger another rebuild in an endless loop.
var generatedGoFiles = map[string]bool{
	"gantry_registry.go":  true,
	"gantry_icons.go":     true,
	"gantry_args.go":      true,
	"gantry_widgets.go":   true,
	"gantry_resources.go": true,
}

// regenForReload re-runs the Go-side generators so a page/component or
// resource added mid-session registers on the next build. It skips
// writeSynth: the vite root depends only on gantry.json, not on .go
// edits, and rewriting it would disturb the running vite server. Errors
// are surfaced but non-fatal - the rebuild proceeds and the compile step
// reports anything real.
func regenForReload(appDir string, cfg appConfig) {
	steps := []func() error{
		func() error { return writeRegistry(appDir) },
		func() error { return writeWidgetRegistry(appDir, cfg) },
		func() error { return writeIcons(appDir, cfg) },
		func() error { return writeResources(appDir) },
		func() error { return writeArgsRegistry(appDir, cfg) },
	}
	for _, gen := range steps {
		if err := gen(); err != nil {
			warn("regen: %v", err)
		}
	}
}

// watchGoSources polls the app tree and sends on changed whenever a
// watched file is saved, added or removed - .go files (except the
// generated gantry_*.go) and anything under resources/. Changes are
// coalesced and debounced by a short settle window so a burst of saves
// yields one rebuild. It returns when done is closed.
func watchGoSources(appDir string, changed chan<- struct{}, done <-chan struct{}) {
	const poll = 300 * time.Millisecond
	const settle = 250 * time.Millisecond

	last := scanSources(appDir)
	var dirtyAt time.Time

	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			cur := scanSources(appDir)
			if cur != last {
				last = cur
				dirtyAt = now // reset the settle window on every change
				continue
			}
			// Stable since the last change: once the settle window has
			// elapsed, fire once (non-blocking - a pending rebuild
			// already covers newer edits).
			if !dirtyAt.IsZero() && now.Sub(dirtyAt) >= settle {
				dirtyAt = time.Time{}
				select {
				case changed <- struct{}{}:
				default:
				}
			}
		}
	}
}

// scanSources fingerprints the watched files (path + mtime + size) into
// one value; any edit, add or remove changes it. Skips the generated,
// build and dependency directories so their churn never triggers a
// rebuild.
func scanSources(appDir string) uint64 {
	h := fnv.New64a()
	_ = filepath.WalkDir(appDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".gantry", "webdist", "dist", "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(appDir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		watched := strings.HasSuffix(d.Name(), ".go") && !generatedGoFiles[d.Name()]
		if !watched && strings.HasPrefix(rel, "resources/") {
			watched = true // resources/ tree: reachable from Go via gantry.Resource
		}
		if !watched {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		io.WriteString(h, rel)
		var buf [16]byte
		binary.LittleEndian.PutUint64(buf[0:8], uint64(fi.ModTime().UnixNano()))
		binary.LittleEndian.PutUint64(buf[8:16], uint64(fi.Size()))
		h.Write(buf[:])
		return nil
	})
	return h.Sum64()
}
