package ui

import "fmt"

// IconSet selects the glyphs drawn before picker and playlist rows.
type IconSet string

const (
	// IconsUnicode uses standard Unicode symbols, so it needs no patched
	// terminal font. The folder glyph has text presentation and is single
	// width, unlike the emoji file-folder.
	IconsUnicode IconSet = "unicode"
	// IconsNerd uses Nerd Fonts glyphs, which only render when the terminal's
	// font is patched.
	IconsNerd IconSet = "nerd"
	// IconsNone keeps rows text-only.
	IconsNone IconSet = "none"
)

// ParseIconSet validates a value from a flag, the config file, or the options
// screen.
func ParseIconSet(value string) (IconSet, error) {
	switch set := IconSet(value); set {
	case IconsUnicode, IconsNerd, IconsNone:
		return set, nil
	default:
		return "", fmt.Errorf("unknown icon set %q, want unicode, nerd, or none", value)
	}
}

// iconSetOrder is the cycle the options screen walks, from the portable set to
// the one that needs a patched font and finally back to no icons.
var iconSetOrder = []IconSet{IconsUnicode, IconsNerd, IconsNone}

// next returns the icon set delta steps around iconSetOrder. Unknown values
// start from the unicode set instead of failing, so a hand-edited config cannot
// leave the options screen with no value to move from.
func (s IconSet) next(delta int) IconSet {
	index := 0
	for i, set := range iconSetOrder {
		if set == s {
			index = i
			break
		}
	}

	n := len(iconSetOrder)
	index = (index + delta%n + n) % n
	return iconSetOrder[index]
}

// folderPrefix is the column prefix for a directory row. It includes the
// separating space so callers can budget its width, or is empty when icons are
// off.
func (s IconSet) folderPrefix() string {
	switch s {
	case IconsNerd:
		return "\uf07b "
	case IconsNone:
		return ""
	default:
		// U+1F5C0 is named FOLDER and, unlike the emoji file-folder, has a
		// text presentation and a single-cell width.
		return "\U0001F5C0 "
	}
}

// trackPrefix is folderPrefix for a playlist row.
func (s IconSet) trackPrefix() string {
	switch s {
	case IconsNerd:
		return "\uf001 "
	case IconsNone:
		return ""
	default:
		return "♪ "
	}
}
