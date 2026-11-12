package layout

// DimensionKind describes how a [Dimension] value is interpreted.
type DimensionKind uint8

const (
	// DimensionAuto indicates the dimension is automatically determined
	// by the layout algorithm (content size, flex basis, or stretch).
	DimensionAuto DimensionKind = iota

	// DimensionPx indicates an absolute pixel value.
	DimensionPx

	// DimensionPercent indicates a percentage of the parent container's
	// corresponding dimension (0–100 scale).
	DimensionPercent

	// DimensionFr indicates a fractional flex factor for CSS Grid tracks (e.g. 1fr).
	DimensionFr

	// DimensionMinContent indicates intrinsic minimum content sizing.
	DimensionMinContent

	// DimensionMaxContent indicates intrinsic maximum content sizing.
	DimensionMaxContent

	// DimensionFitContent indicates fit-content sizing.
	DimensionFitContent

	// DimensionContent indicates content-based sizing (e.g. flex-basis: content).
	DimensionContent
)

// Dimension represents a CSS dimension value that can be auto, an absolute
// pixel value, or a percentage of the parent.
type Dimension struct {
	Kind  DimensionKind
	Value float32
}

// Auto returns a [Dimension] with [DimensionAuto] kind, meaning the layout
// algorithm determines the size automatically.
func Auto() Dimension {
	return Dimension{Kind: DimensionAuto, Value: -1}
}

// isAutoMargin reports whether d represents an explicitly set auto margin.
func isAutoMargin(d Dimension) bool {
	return d.Kind == DimensionAuto && d.Value != 0
}

// Px returns a [Dimension] with an absolute pixel value.
func Px(v float32) Dimension {
	return Dimension{Kind: DimensionPx, Value: v}
}

// Percent returns a [Dimension] as a percentage (0–100) of the parent's
// corresponding dimension.
func Percent(v float32) Dimension {
	return Dimension{Kind: DimensionPercent, Value: v}
}

// Fr returns a fractional unit [Dimension] for CSS Grid track sizing.
func Fr(v float32) Dimension {
	return Dimension{Kind: DimensionFr, Value: v}
}
