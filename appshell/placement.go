package appshell

import "github.com/B-Commissions/Gantry/monitors"

// widgetPos computes a widget's top-left corner on its monitor's work
// area from its placement.
func widgetPos(o WidgetOptions) (int, int) {
	if o.Placement == PlaceCustom {
		return o.X, o.Y
	}
	m := monitors.Pick(monitors.All(), o.Monitor)
	var x, y int
	switch o.Placement {
	case PlaceTopLeft, PlaceBottomLeft:
		x = m.WorkX + o.Margin
	case PlaceTopRight, PlaceBottomRight:
		x = m.WorkX + m.WorkWidth - o.Width - o.Margin
	default: // centered horizontally
		x = m.WorkX + (m.WorkWidth-o.Width)/2
	}
	switch o.Placement {
	case PlaceTopCenter, PlaceTopLeft, PlaceTopRight:
		y = m.WorkY + o.Margin
	case PlaceBottomCenter, PlaceBottomLeft, PlaceBottomRight:
		y = m.WorkY + m.WorkHeight - o.Height - o.Margin
	default: // PlaceCenter
		y = m.WorkY + (m.WorkHeight-o.Height)/2
	}
	return x, y
}

// popupPos computes a popup's top-left corner: horizontally centered,
// at the top or bottom of the monitor's work area, nudged by AdjustPos.
func popupPos(o PopupOptions) (int, int) {
	m := monitors.Pick(monitors.All(), o.Monitor)
	x := m.WorkX + (m.WorkWidth-o.Width)/2
	y := m.WorkY + m.WorkHeight - o.Height - o.Margin
	if o.Position == "top" {
		y = m.WorkY + o.Margin
	}
	if o.AdjustPos != nil {
		x, y = o.AdjustPos(x, y)
	}
	return x, y
}

// centerPos computes a window's centered top-left on the primary work
// area (used when no explicit position or saved geometry exists).
func centerPos(width, height int) (int, int) {
	m := monitors.Pick(monitors.All(), -1)
	return m.WorkX + (m.WorkWidth-width)/2, m.WorkY + (m.WorkHeight-height)/2
}
