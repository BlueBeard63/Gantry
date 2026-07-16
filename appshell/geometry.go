package appshell

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/B-Commissions/Gantry/monitors"
)

// Rect is a window rectangle in virtual-desktop pixels.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// GeometryStore persists the main window's position and size between
// runs. Implementations are best-effort: a failed Load just means the
// default geometry is used.
type GeometryStore interface {
	Load() (r Rect, maximized bool, ok bool)
	Save(r Rect, maximized bool)
}

type fileGeometry struct {
	path string
}

type geometryFile struct {
	Rect      Rect `json:"rect"`
	Maximized bool `json:"maximized"`
}

// FileGeometry persists geometry as a small JSON file (create the parent
// directory yourself or use a path under os.UserConfigDir()). Loads are
// clamped to a currently visible monitor so a position saved on an
// unplugged display cannot strand the window off-screen.
func FileGeometry(path string) GeometryStore {
	return &fileGeometry{path: path}
}

func (f *fileGeometry) Load() (Rect, bool, bool) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		return Rect{}, false, false
	}
	var g geometryFile
	if json.Unmarshal(data, &g) != nil || g.Rect.Width <= 0 || g.Rect.Height <= 0 {
		return Rect{}, false, false
	}
	return clampToVisible(g.Rect), g.Maximized, true
}

func (f *fileGeometry) Save(r Rect, maximized bool) {
	if r.Width <= 0 || r.Height <= 0 {
		return
	}
	_ = os.MkdirAll(filepath.Dir(f.path), 0o700)
	data, err := json.Marshal(geometryFile{Rect: r, Maximized: maximized})
	if err != nil {
		return
	}
	_ = os.WriteFile(f.path, data, 0o600)
}

// clampToVisible moves/sizes a rect so a meaningful part of it lies on a
// currently attached monitor's work area.
func clampToVisible(r Rect) Rect {
	all := monitors.All()
	if len(all) == 0 {
		return r
	}
	// If any monitor already shows the window's top strip (where the
	// drag band lives), keep the rect as-is.
	for _, m := range all {
		if r.X+r.Width > m.WorkX+32 && r.X < m.WorkX+m.WorkWidth-32 &&
			r.Y >= m.WorkY-8 && r.Y < m.WorkY+m.WorkHeight-32 {
			return r
		}
	}
	// Otherwise re-place on the primary work area, preserving size where
	// it fits.
	m := monitors.Pick(all, -1)
	if r.Width > m.WorkWidth {
		r.Width = m.WorkWidth
	}
	if r.Height > m.WorkHeight {
		r.Height = m.WorkHeight
	}
	r.X = m.WorkX + (m.WorkWidth-r.Width)/2
	r.Y = m.WorkY + (m.WorkHeight-r.Height)/2
	return r
}
