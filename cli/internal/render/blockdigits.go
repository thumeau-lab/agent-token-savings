package render

// blockDigitGlyph is one character's 3-row-tall rendering: all three
// rows share the same rune width (the renderer relies on this to keep
// every glyph's column aligned inside the dashboard's hero tiles).
type blockDigitGlyph [3]string

// blockDigitGlyphs is the small table the card asks for: digits 0-9
// plus the punctuation FormatUSDLang/FormatPercentLang ever emit
// (",", ".", "%", "$") and a plain space, each as a simple
// box-drawing/half-block 3-row glyph. Not a full LCD font -- just
// enough to read a cost/percentage figure in large type.
var blockDigitGlyphs = map[rune]blockDigitGlyph{
	'0': {"█▀█", "█ █", "█▄█"},
	'1': {" ▄█", "  █", "  █"},
	'2': {"█▀█", " ▄▀", "█▄▄"},
	'3': {"█▀█", " ▀█", "█▄█"},
	'4': {"█ █", "█▀█", "  █"},
	'5': {"█▀▀", "█▀█", "▄▄█"},
	'6': {"█▀▀", "█▀█", "█▄█"},
	'7': {"▀▀█", "  █", "  █"},
	'8': {"█▀█", "█▀█", "█▄█"},
	'9': {"█▀█", "▀▀█", "▄▄█"},
	',': {" ", " ", "▄"},
	'.': {" ", " ", "▄"},
	'%': {"▀▄█", " ▄▀", "█▄▀"},
	'$': {"▄█▄", "▀█▄", "▄█▀"},
	' ': {" ", " ", " "},
}

// blockDigitFallback renders a rune outside blockDigitGlyphs as a
// single-column glyph with the literal rune on the middle row: never
// panics, never silently drops the character, and stays a well-formed
// 3-row glyph like every other entry.
func blockDigitFallback(r rune) blockDigitGlyph {
	return blockDigitGlyph{" ", string(r), " "}
}

// BlockDigits renders s as a 3-row-tall block-digit banner, one glyph
// per rune (see blockDigitGlyphs), separated by a single-space gutter.
// Used by the dashboard's hero tiles (cost/saved/hit-rate) on wide
// terminals; callers fall back to a plain bold line below the card's
// 120-column threshold. A rune not in the table (should not happen for
// the money/percent strings this is fed, but never trust formatting
// code blindly) renders via blockDigitFallback instead of panicking.
func BlockDigits(s string) []string {
	var rows [3]string
	for i, r := range []rune(s) {
		g, ok := blockDigitGlyphs[r]
		if !ok {
			g = blockDigitFallback(r)
		}
		for line := 0; line < 3; line++ {
			if i > 0 {
				rows[line] += " "
			}
			rows[line] += g[line]
		}
	}
	return rows[:]
}
