package layout

// ---------------------------------------------------------------------------
// PositionType
// ---------------------------------------------------------------------------

// PositionType specifies how an element is positioned in the layout.
type PositionType uint8

const (
	// PositionRelative positions the node relative to its normal in-flow position.
	PositionRelative PositionType = iota
	// PositionAbsolute removes the node from normal flow and positions it
	// relative to its containing block.
	PositionAbsolute
)

// ---------------------------------------------------------------------------
// Display
// ---------------------------------------------------------------------------

// Display controls the layout mode of a node.
type Display uint8

const (
	// DisplayFlex makes the node a flex container.
	DisplayFlex Display = iota
	// DisplayBlock makes the node a block-level element (laid out like a
	// single-item column).
	DisplayBlock
	// DisplayNone hides the node — it is skipped during layout and occupies
	// no space.
	DisplayNone
	// DisplayGrid makes the node a CSS Grid container.
	DisplayGrid
)

// ---------------------------------------------------------------------------
// FlexDirection
// ---------------------------------------------------------------------------

// FlexDirection establishes the main axis of a flex container.
type FlexDirection uint8

const (
	// FlexDirectionRow lays out children left-to-right (main axis = horizontal).
	FlexDirectionRow FlexDirection = iota
	// FlexDirectionColumn lays out children top-to-bottom (main axis = vertical).
	FlexDirectionColumn
	// FlexDirectionRowReverse lays out children right-to-left.
	FlexDirectionRowReverse
	// FlexDirectionColumnReverse lays out children bottom-to-top.
	FlexDirectionColumnReverse
)

// ---------------------------------------------------------------------------
// FlexWrap
// ---------------------------------------------------------------------------

// FlexWrap controls whether flex items are forced onto a single line or
// can wrap onto multiple lines.
type FlexWrap uint8

const (
	// FlexWrapNoWrap forces all items onto one line (default).
	FlexWrapNoWrap FlexWrap = iota
	// FlexWrapWrap allows items to wrap onto additional lines.
	FlexWrapWrap
	// FlexWrapWrapReverse wraps items in reverse order.
	FlexWrapWrapReverse
)

// ---------------------------------------------------------------------------
// JustifyContent
// ---------------------------------------------------------------------------

// JustifyContent defines how free space is distributed along the main axis.
type JustifyContent uint8

const (
	// JustifyFlexStart packs items toward the start of the main axis.
	JustifyFlexStart JustifyContent = iota
	// JustifyFlexEnd packs items toward the end of the main axis.
	JustifyFlexEnd
	// JustifyCenter centers items along the main axis.
	JustifyCenter
	// JustifySpaceBetween distributes items evenly: first item at start,
	// last at end.
	JustifySpaceBetween
	// JustifySpaceAround distributes items with equal space around each item.
	JustifySpaceAround
	// JustifySpaceEvenly distributes items with equal space between and
	// around all items.
	JustifySpaceEvenly
	// JustifyStretch stretches auto items along the main axis.
	JustifyStretch
	// JustifyStart packs items toward the start of the writing mode / physical start.
	JustifyStart
	// JustifyEnd packs items toward the end of the writing mode / physical end.
	JustifyEnd
)

// ---------------------------------------------------------------------------
// AlignItems
// ---------------------------------------------------------------------------

// AlignItems defines the default cross-axis alignment for all children
// of a flex container.
type AlignItems uint8

const (
	// AlignStretch stretches items to fill the cross axis (default).
	AlignStretch AlignItems = iota
	// AlignFlexStart aligns items to the start of the cross axis.
	AlignFlexStart
	// AlignFlexEnd aligns items to the end of the cross axis.
	AlignFlexEnd
	// AlignCenter centers items on the cross axis.
	AlignCenter
	// AlignBaseline aligns items along their baselines. In Phase 1 this
	// behaves identically to AlignFlexStart.
	AlignBaseline
)

// ---------------------------------------------------------------------------
// AlignContent
// ---------------------------------------------------------------------------

// AlignContent controls how flex lines are distributed along the cross axis
// when there is extra space in a multi-line flex container.
type AlignContent uint8

const (
	// AlignContentStretch stretches lines to fill remaining cross-axis space.
	AlignContentStretch AlignContent = iota
	// AlignContentFlexStart packs lines toward the cross-axis start.
	AlignContentFlexStart
	// AlignContentFlexEnd packs lines toward the cross-axis end.
	AlignContentFlexEnd
	// AlignContentCenter centers lines on the cross axis.
	AlignContentCenter
	// AlignContentSpaceBetween distributes lines evenly: first at start,
	// last at end.
	AlignContentSpaceBetween
	// AlignContentSpaceAround distributes lines with equal space around each.
	AlignContentSpaceAround
	// AlignContentSpaceEvenly distributes lines with equal space between and around each.
	AlignContentSpaceEvenly
)

// ---------------------------------------------------------------------------
// AlignSelf
// ---------------------------------------------------------------------------

// AlignSelf overrides the parent container's [AlignItems] for a single child.
type AlignSelf uint8

const (
	// AlignSelfAuto inherits the parent's AlignItems value (default).
	AlignSelfAuto AlignSelf = iota
	// AlignSelfStretch stretches this item to fill the cross axis.
	AlignSelfStretch
	// AlignSelfFlexStart aligns this item to the cross-axis start.
	AlignSelfFlexStart
	// AlignSelfFlexEnd aligns this item to the cross-axis end.
	AlignSelfFlexEnd
	// AlignSelfCenter centers this item on the cross axis.
	AlignSelfCenter
	// AlignSelfBaseline aligns this item to the baseline.
	AlignSelfBaseline
)

// ---------------------------------------------------------------------------
// GridAutoFlow
// ---------------------------------------------------------------------------

// GridAutoFlow controls the placement algorithm for grid items without
// explicit positions.
type GridAutoFlow uint8

const (
	// GridAutoFlowRow packs items by filling each row in turn (default).
	GridAutoFlowRow GridAutoFlow = iota
	// GridAutoFlowColumn packs items by filling each column in turn.
	GridAutoFlowColumn
	// GridAutoFlowRowDense fills holes earlier in the grid if smaller items appear later.
	GridAutoFlowRowDense
	// GridAutoFlowColumnDense fills holes column-wise.
	GridAutoFlowColumnDense
)

// ---------------------------------------------------------------------------
// Edges
// ---------------------------------------------------------------------------

// Edges holds four [Dimension] values corresponding to the CSS box-model
// edges: top, right, bottom, left.
type Edges struct {
	Top    Dimension
	Right  Dimension
	Bottom Dimension
	Left   Dimension
}

// Uniform returns an [Edges] value with all four sides set to Px(v).
func Uniform(v float32) Edges {
	d := Px(v)
	return Edges{Top: d, Right: d, Bottom: d, Left: d}
}

// EdgesPx returns an [Edges] value with each side set to its own pixel value.
func EdgesPx(top, right, bottom, left float32) Edges {
	return Edges{
		Top:    Px(top),
		Right:  Px(right),
		Bottom: Px(bottom),
		Left:   Px(left),
	}
}

// ---------------------------------------------------------------------------
// Overflow
// ---------------------------------------------------------------------------

// Overflow describes how content overflowing the box is handled.
type Overflow uint8

const (
	OverflowVisible Overflow = iota
	OverflowHidden
	OverflowScroll
	OverflowAuto
)

// ---------------------------------------------------------------------------
// BoxSizing
// ---------------------------------------------------------------------------

// BoxSizing specifies whether width and height include padding and borders.
type BoxSizing uint8

const (
	// BoxSizingBorderBox (default) includes padding and border in width/height.
	BoxSizingBorderBox BoxSizing = iota
	// BoxSizingContentBox excludes padding and border from width/height.
	BoxSizingContentBox
)

// ---------------------------------------------------------------------------
// Style
// ---------------------------------------------------------------------------

// Style holds all CSS Flexbox and Grid properties for a layout node. Use
// [DefaultStyle] to get a value pre-filled with sensible CSS defaults.
type Style struct {
	// Sizing model
	BoxSizing BoxSizing

	// Positioning
	Position PositionType
	Inset    Edges // Top, Right, Bottom, Left offsets

	// Overflow
	OverflowX Overflow
	OverflowY Overflow

	// Container properties
	Display        Display
	FlexDirection  FlexDirection
	FlexWrap       FlexWrap
	JustifyContent JustifyContent
	AlignItems     AlignItems
	AlignContent   AlignContent

	// Item properties
	AlignSelf  AlignSelf
	FlexGrow   float32
	FlexShrink float32
	FlexBasis  Dimension

	// Sizing
	Width       Dimension
	Height      Dimension
	MinWidth    Dimension
	MinHeight   Dimension
	MaxWidth    Dimension
	MaxHeight   Dimension
	AspectRatio float32 // Aspect ratio (width / height). If > 0, calculates auto dimension.

	// Box model
	Margin  Edges
	Padding Edges

	// Gap is the spacing between flex items on the main axis.
	// For simplicity, row-gap and column-gap are treated as equal if RowGap/ColumnGap are unset.
	Gap Dimension

	// Grid Container properties
	GridTemplateColumns []Dimension
	GridTemplateRows    []Dimension
	GridAutoColumns     []Dimension
	GridAutoRows        []Dimension
	GridAutoFlow        GridAutoFlow
	AutoFitCols         bool
	AutoFitRows         bool
	RowGap              Dimension
	ColumnGap           Dimension
	JustifyItems        AlignItems // Default inline-axis alignment for all grid items

	// Grid Item properties
	GridColumnStart int16 // 1-based start line (0 = auto)
	GridColumnEnd   int16 // 1-based end line (0 = auto)
	GridColumnSpan  uint16 // Span count if specified (default 1)
	GridRowStart    int16 // 1-based start line (0 = auto)
	GridRowEnd      int16 // 1-based end line (0 = auto)
	GridRowSpan     uint16 // Span count if specified (default 1)
	JustifySelf     AlignSelf
}

// DefaultStyle returns a [Style] with sensible CSS defaults:
//   - Display = DisplayFlex
//   - FlexShrink = 1
//   - All dimensions = Auto
//   - Everything else zero-valued
func DefaultStyle() Style {
	return Style{
		Position:   PositionRelative,
		Display:    DisplayFlex,
		FlexShrink: 1,
	}
}
