package text

// Direction specifies horizontal text flow direction.
type Direction uint8

const (
	// DirectionLTR flows left-to-right (Latin, Cyrillic, Greek, CJK, etc.).
	DirectionLTR Direction = iota
	// DirectionRTL flows right-to-left (Hebrew, Arabic, Persian, Urdu, etc.).
	DirectionRTL
	// DirectionAuto detects direction from the first strongly-directional character (UAX #9).
	DirectionAuto
)

// IsRTLRune returns true if r belongs to a right-to-left script.
func IsRTLRune(r rune) bool {
	return (r >= 0x0590 && r <= 0x05FF) || // Hebrew
		(r >= 0x0600 && r <= 0x06FF) || // Arabic
		(r >= 0x0700 && r <= 0x074F) || // Syriac
		(r >= 0x0750 && r <= 0x077F) || // Arabic Supplement
		(r >= 0x0780 && r <= 0x07BF) || // Thaana
		(r >= 0x08A0 && r <= 0x08FF) || // Arabic Extended-A
		(r >= 0xFB50 && r <= 0xFDFF) || // Arabic Presentation Forms-A
		(r >= 0xFE70 && r <= 0xFEFF) // Arabic Presentation Forms-B
}

// DetectDirection inspects the text and returns DirectionRTL if RTL characters are detected.
func DetectDirection(text string) Direction {
	for _, r := range text {
		if IsRTLRune(r) {
			return DirectionRTL
		}
	}
	return DirectionLTR
}
