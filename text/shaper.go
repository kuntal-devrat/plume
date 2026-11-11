package text

import (
	"strings"
	"unicode"
)

// ShapedGlyph represents a positioned glyph in a line of shaped text.
type ShapedGlyph struct {
	GlyphID uint16
	X, Y    float32 // Position relative to text origin
	Advance float32 // Advance width
}

// Line represents a single line of shaped text with its glyphs and layout dimensions.
type Line struct {
	Glyphs  []ShapedGlyph
	Width   float32
	Height  float32
	Ascent  float32
	Descent float32
}

// TextLayout holds the complete shaped result of a multi-line string.
type TextLayout struct {
	Lines       []Line
	TotalWidth  float32
	TotalHeight float32
}

// ShapeText shapes a string into one or more lines of positioned glyphs,
// applying OpenType GSUB ligatures, kerning, font metrics, and word wrapping at maxWidth.
// Bidirectional direction is automatically detected per UAX #9.
func ShapeText(font *Font, text string, fontSize float32, maxWidth float32) TextLayout {
	return ShapeTextWithDirection(font, text, fontSize, maxWidth, DirectionAuto)
}

// ShapeTextWithDirection shapes text with an explicit layout direction.
func ShapeTextWithDirection(font *Font, text string, fontSize float32, maxWidth float32, dir Direction) TextLayout {
	if font == nil || len(text) == 0 {
		return TextLayout{}
	}

	if dir == DirectionAuto {
		dir = DetectDirection(text)
	}

	lm := font.LineMetrics(fontSize)

	// Split text by hard newlines first
	rawParagraphs := strings.Split(text, "\n")
	var allLines []Line

	for _, para := range rawParagraphs {
		if maxWidth <= 0 {
			line := shapeSingleLine(font, para, fontSize, dir)
			line.Ascent = lm.Ascent
			line.Descent = lm.Descent
			line.Height = lm.LineHeight
			allLines = append(allLines, line)
			continue
		}

		// Word wrap paragraph into lines <= maxWidth
		wrapped := wrapParagraph(font, para, fontSize, maxWidth, dir)
		for _, wLine := range wrapped {
			wLine.Ascent = lm.Ascent
			wLine.Descent = lm.Descent
			wLine.Height = lm.LineHeight
			allLines = append(allLines, wLine)
		}
	}

	var maxW float32
	for _, l := range allLines {
		if l.Width > maxW {
			maxW = l.Width
		}
	}

	totalH := float32(len(allLines)) * lm.LineHeight
	if len(allLines) > 0 {
		// Remove bottom line gap on the last line for tighter box fitting
		totalH -= lm.LineGap
	}

	return TextLayout{
		Lines:       allLines,
		TotalWidth:  maxW,
		TotalHeight: totalH,
	}
}

func shapeSingleLine(font *Font, s string, fontSize float32, dir Direction) Line {
	runes := []rune(s)
	rawGids := make([]uint16, len(runes))
	for i, r := range runes {
		rawGids[i] = font.GlyphIndex(r)
	}

	// Apply OpenType GSUB ligatures
	gids := font.ApplyLigatures(rawGids)
	glyphs := make([]ShapedGlyph, 0, len(gids))

	var curX float32
	var prevID uint16

	for i, gid := range gids {
		if i > 0 && prevID != 0 && gid != 0 {
			curX += font.Kerning(prevID, gid, fontSize)
		}

		adv := font.GlyphAdvance(gid, fontSize)
		glyphs = append(glyphs, ShapedGlyph{
			GlyphID: gid,
			X:       curX,
			Y:       0,
			Advance: adv,
		})

		curX += adv
		prevID = gid
	}

	totalWidth := curX

	// Handle Right-to-Left reordering
	if dir == DirectionRTL && len(glyphs) > 0 {
		for i := range glyphs {
			glyphs[i].X = totalWidth - glyphs[i].X - glyphs[i].Advance
		}
	}

	return Line{
		Glyphs: glyphs,
		Width:  totalWidth,
	}
}

func wrapParagraph(font *Font, text string, fontSize, maxWidth float32, dir Direction) []Line {
	if len(text) == 0 {
		return []Line{{}}
	}

	// Split into words, preserving spaces
	words := splitIntoWords(text)
	var lines []Line

	var currentRunes []rune
	var currentWidth float32

	for _, w := range words {
		wLine := shapeSingleLine(font, w, fontSize, dir)
		if len(currentRunes) == 0 {
			currentRunes = append(currentRunes, []rune(w)...)
			currentWidth = wLine.Width
			continue
		}

		if currentWidth+wLine.Width <= maxWidth {
			currentRunes = append(currentRunes, []rune(w)...)
			currentWidth += wLine.Width
		} else {
			// Trim trailing spaces before completing line
			trimmed := strings.TrimRightFunc(string(currentRunes), unicode.IsSpace)
			lines = append(lines, shapeSingleLine(font, trimmed, fontSize, dir))
			currentRunes = []rune(strings.TrimLeftFunc(w, unicode.IsSpace))
			currentWidth = shapeSingleLine(font, string(currentRunes), fontSize, dir).Width
		}
	}

	if len(currentRunes) > 0 {
		lines = append(lines, shapeSingleLine(font, string(currentRunes), fontSize, dir))
	}

	return lines
}

func splitIntoWords(s string) []string {
	var words []string
	var cur strings.Builder
	inSpace := false

	for _, r := range s {
		isSp := unicode.IsSpace(r)
		if isSp != inSpace && cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
		inSpace = isSp
		cur.WriteRune(r)
	}

	if cur.Len() > 0 {
		words = append(words, cur.String())
	}

	return words
}
