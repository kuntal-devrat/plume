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
// applying kerning, font metrics, and word wrapping at maxWidth.
// If maxWidth <= 0, no word wrapping is performed (single-line or split on explicit newlines only).
func ShapeText(font *Font, text string, fontSize float32, maxWidth float32) TextLayout {
	if font == nil || len(text) == 0 {
		return TextLayout{}
	}

	lm := font.LineMetrics(fontSize)

	// Split text by hard newlines first
	rawParagraphs := strings.Split(text, "\n")
	var allLines []Line

	for _, para := range rawParagraphs {
		if maxWidth <= 0 {
			line := shapeSingleLine(font, para, fontSize)
			line.Ascent = lm.Ascent
			line.Descent = lm.Descent
			line.Height = lm.LineHeight
			allLines = append(allLines, line)
			continue
		}

		// Word wrap paragraph into lines <= maxWidth
		wrapped := wrapParagraph(font, para, fontSize, maxWidth)
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

func shapeSingleLine(font *Font, s string, fontSize float32) Line {
	runes := []rune(s)
	glyphs := make([]ShapedGlyph, 0, len(runes))

	var curX float32
	var prevID uint16

	for i, r := range runes {
		gid := font.GlyphIndex(r)
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

	return Line{
		Glyphs: glyphs,
		Width:  curX,
	}
}

func wrapParagraph(font *Font, text string, fontSize, maxWidth float32) []Line {
	if len(text) == 0 {
		return []Line{{}}
	}

	// Split into words, preserving spaces
	words := splitIntoWords(text)
	var lines []Line

	var currentRunes []rune
	var currentWidth float32

	for _, w := range words {
		wLine := shapeSingleLine(font, w, fontSize)
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
			lineStr := strings.TrimRight(string(currentRunes), " \t")
			lines = append(lines, shapeSingleLine(font, lineStr, fontSize))

			// If word itself starts with space, trim leading space for new line
			trimmedWord := strings.TrimLeft(w, " \t")
			currentRunes = []rune(trimmedWord)
			currentWidth = shapeSingleLine(font, trimmedWord, fontSize).Width
		}
	}

	if len(currentRunes) > 0 {
		lineStr := strings.TrimRight(string(currentRunes), " \t")
		lines = append(lines, shapeSingleLine(font, lineStr, fontSize))
	}

	return lines
}

func splitIntoWords(s string) []string {
	var words []string
	var cur strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		cur.WriteRune(r)
		if unicode.IsSpace(r) {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	if cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}
