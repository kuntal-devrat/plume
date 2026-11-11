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
	return Dimension{Kind: DimensionAuto}
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
