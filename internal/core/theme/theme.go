package theme

import "strings"

type Palette struct {
	Bg             string
	Fg             string
	Accent         string
	AccentFg       string
	Focus          string
	RowHover       string
	RowSelected    string
	StatusOK       string
	StatusPending  string
	StatusWarning  string
	StatusConflict string
	StatusDanger   string
}

// DefaultDarkPalette returns the default dark Bitwarden-inspired overlay palette.
func DefaultDarkPalette() Palette {
	return Palette{
		Bg:             "#07111f",
		Fg:             "#f3f6f9",
		Accent:         "#175ddc",
		AccentFg:       "#f3f6f9",
		Focus:          "#2cdde9",
		RowHover:       "#1d3358",
		RowSelected:    "rgba(23, 93, 220, 0.22)",
		StatusOK:       "#7bf1a8",
		StatusPending:  "#2cdde9",
		StatusWarning:  "#fdc700",
		StatusConflict: "#fdc700",
		StatusDanger:   "#ff6550",
	}
}

// ApplyOverrides returns a copy of p with any non-empty values from overrides applied.
// The overrides map uses snake_case keys matching the Palette field names:
// bg, fg, accent, accent_fg, focus, row_hover, row_selected,
// status_ok, status_pending, status_warning, status_conflict, status_danger.
func ApplyOverrides(p Palette, overrides map[string]string) Palette {
	if len(overrides) == 0 {
		return p
	}
	if v, ok := overrides["bg"]; ok && v != "" {
		p.Bg = v
	}
	if v, ok := overrides["fg"]; ok && v != "" {
		p.Fg = v
	}
	if v, ok := overrides["accent"]; ok && v != "" {
		p.Accent = v
	}
	if v, ok := overrides["accent_fg"]; ok && v != "" {
		p.AccentFg = v
	}
	if v, ok := overrides["focus"]; ok && v != "" {
		p.Focus = v
	}
	if v, ok := overrides["row_hover"]; ok && v != "" {
		p.RowHover = v
	}
	if v, ok := overrides["row_selected"]; ok && v != "" {
		p.RowSelected = v
	}
	if v, ok := overrides["status_ok"]; ok && v != "" {
		p.StatusOK = v
	}
	if v, ok := overrides["status_pending"]; ok && v != "" {
		p.StatusPending = v
	}
	if v, ok := overrides["status_warning"]; ok && v != "" {
		p.StatusWarning = v
	}
	if v, ok := overrides["status_conflict"]; ok && v != "" {
		p.StatusConflict = v
	}
	if v, ok := overrides["status_danger"]; ok && v != "" {
		p.StatusDanger = v
	}
	return p
}

// Map returns the palette as a map for use with GTK CSS or theme engines.
func (p Palette) Map() map[string]string {
	return map[string]string{
		"bg":              p.Bg,
		"fg":              p.Fg,
		"accent":          p.Accent,
		"accent_fg":       p.AccentFg,
		"focus":           p.Focus,
		"row_hover":       p.RowHover,
		"row_selected":    p.RowSelected,
		"status_ok":       p.StatusOK,
		"status_pending":  p.StatusPending,
		"status_warning":  p.StatusWarning,
		"status_conflict": p.StatusConflict,
		"status_danger":   p.StatusDanger,
	}
}

// FieldValue returns the palette value for a given snake_case field name.
func (p Palette) FieldValue(field string) string {
	switch strings.ToLower(field) {
	case "bg":
		return p.Bg
	case "fg":
		return p.Fg
	case "accent":
		return p.Accent
	case "accent_fg":
		return p.AccentFg
	case "focus":
		return p.Focus
	case "row_hover":
		return p.RowHover
	case "row_selected":
		return p.RowSelected
	case "status_ok":
		return p.StatusOK
	case "status_pending":
		return p.StatusPending
	case "status_warning":
		return p.StatusWarning
	case "status_conflict":
		return p.StatusConflict
	case "status_danger":
		return p.StatusDanger
	default:
		return ""
	}
}
