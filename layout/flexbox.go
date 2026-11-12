package layout

// Compute performs the full W3C CSS Flexbox layout algorithm starting from
// the tree's root node. availableWidth and availableHeight define the
// viewport / containing block dimensions in pixels.
//
// After Compute returns, every node's [LayoutNode.Computed] rect contains its
// final position (relative to its parent) and size. The method performs zero
// heap allocations after the initial tree construction.
func (t *Tree) Compute(availableWidth, availableHeight float32) {
	if t.root == InvalidNode {
		return
	}
	t.computeNode(t.root, availableWidth, availableHeight)
}

// computeNode recursively lays out a single node and all its descendants.
// parentWidth/parentHeight are the dimensions of the containing block,
// used to resolve percentage values.
func (t *Tree) computeNode(id NodeID, parentWidth, parentHeight float32) {
	node := &t.nodes[id]

	if node.Style.Display == DisplayNone {
		node.Computed = Rect{}
		return
	}

	// --- Resolve own outer dimensions ---
	width := t.resolveDimension(node.Style.Width, parentWidth)
	height := t.resolveDimension(node.Style.Height, parentHeight)

	// Resolve padding
	padTop := t.resolveDimensionOrZero(node.Style.Padding.Top, parentHeight)
	padRight := t.resolveDimensionOrZero(node.Style.Padding.Right, parentWidth)
	padBottom := t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentHeight)
	padLeft := t.resolveDimensionOrZero(node.Style.Padding.Left, parentWidth)

	// Call intrinsic measure func if registered and dimensions are auto
	if int(id) < len(t.measureFuncs) && t.measureFuncs[id] != nil {
		fn := t.measureFuncs[id]
		var wMode, hMode MeasureMode
		wArg := parentWidth
		hArg := parentHeight
		if node.Style.Width.Kind != DimensionAuto {
			wMode = MeasureExact
			wArg = width
		} else {
			wMode = MeasureAtMost
		}
		if node.Style.Height.Kind != DimensionAuto {
			hMode = MeasureExact
			hArg = height
		} else {
			hMode = MeasureAtMost
		}
		mw, mh := fn(t, id, wArg, wMode, hArg, hMode)
		if node.Style.Width.Kind == DimensionAuto {
			width = mw + padLeft + padRight
		}
		if node.Style.Height.Kind == DimensionAuto {
			height = mh + padTop + padBottom
		}
	}

	// Apply AspectRatio if specified
	if node.Style.AspectRatio > 0 {
		if width >= 0 && node.Style.Height.Kind == DimensionAuto {
			height = width / node.Style.AspectRatio
		} else if height >= 0 && node.Style.Width.Kind == DimensionAuto {
			width = height * node.Style.AspectRatio
		}
	}

	// If width/height are auto and we have a definite parent, default to
	// the parent size (block-level behaviour for the root / non-flex items).
	if node.Style.Width.Kind == DimensionAuto && width < 0 {
		width = parentWidth
	}
	if node.Style.Height.Kind == DimensionAuto && height < 0 {
		if id != t.root {
			height = parentHeight
		} else {
			height = 0
		}
	}

	// Content box = outer size minus padding.
	contentWidth := width - padLeft - padRight
	contentHeight := height - padTop - padBottom
	if contentWidth < 0 {
		contentWidth = 0
	}
	if contentHeight < 0 {
		contentHeight = 0
	}

	// Apply min/max constraints to the overall node size.
	width = t.clampSize(width, node.Style.MinWidth, node.Style.MaxWidth, parentWidth)
	height = t.clampSize(height, node.Style.MinHeight, node.Style.MaxHeight, parentHeight)

	// Recompute content box after clamping.
	contentWidth = width - padLeft - padRight
	contentHeight = height - padTop - padBottom
	if contentWidth < 0 {
		contentWidth = 0
	}
	if contentHeight < 0 {
		contentHeight = 0
	}

	node.Computed.Width = width
	node.Computed.Height = height

	// If no children, we're done.
	if node.FirstChild == InvalidNode {
		return
	}

	// Determine if this is a flex container.
	if node.Style.Display == DisplayGrid {
		t.layoutGrid(id, contentWidth, contentHeight, padLeft, padTop, parentWidth, parentHeight)
		return
	}

	if node.Style.Display != DisplayFlex {
		// Block containers: lay out children stacked vertically.
		t.layoutBlock(id, contentWidth, contentHeight, padLeft, padTop)
		return
	}

	t.layoutFlex(id, contentWidth, contentHeight, padLeft, padTop, parentWidth, parentHeight)
}

// layoutBlock stacks children vertically (simplified block layout).
func (t *Tree) layoutBlock(id NodeID, cw, ch, padL, padT float32) {
	yOffset := padT
	child := t.nodes[id].FirstChild
	for child != InvalidNode {
		c := &t.nodes[child]
		if c.Style.Display != DisplayNone {
			t.computeNode(child, cw, ch)
			c.Computed.X = padL
			c.Computed.Y = yOffset
			yOffset += c.Computed.Height
		}
		child = c.NextSibling
	}
}

// layoutFlex implements the W3C CSS Flexible Box Layout algorithm.
func (t *Tree) layoutFlex(id NodeID, contentWidth, contentHeight, padLeft, padTop, parentWidth, parentHeight float32) {
	node := &t.nodes[id]
	dir := node.Style.FlexDirection

	// --- 1. Determine axes ---
	isRow := dir == FlexDirectionRow || dir == FlexDirectionRowReverse
	isReverse := dir == FlexDirectionRowReverse || dir == FlexDirectionColumnReverse

	var mainSize, crossSize float32
	if isRow {
		mainSize = contentWidth
		crossSize = contentHeight
	} else {
		mainSize = contentHeight
		crossSize = contentWidth
	}

	// --- 2. Collect flex items ---
	var itemsBuf [32]flexItem
	items := itemsBuf[:0]
	child := node.FirstChild
	for child != InvalidNode {
		c := &t.nodes[child]
		if c.Style.Display != DisplayNone && c.Style.Position != PositionAbsolute {
			fi := flexItem{
				node:       child,
				flexGrow:   c.Style.FlexGrow,
				flexShrink: c.Style.FlexShrink,
			}
			items = append(items, fi)
		}
		child = c.NextSibling
	}

	if len(items) == 0 {
		t.layoutAbsoluteChildren(id, contentWidth, contentHeight, padLeft, padTop)
		return
	}

	// --- 3. Resolve flex basis & hypothetical main size ---
	var mainGap, crossGap float32
	if isRow {
		if node.Style.ColumnGap.Kind != DimensionAuto {
			mainGap = t.resolveDimensionOrZero(node.Style.ColumnGap, mainSize)
		} else {
			mainGap = t.resolveDimensionOrZero(node.Style.Gap, mainSize)
		}
		if node.Style.RowGap.Kind != DimensionAuto {
			crossGap = t.resolveDimensionOrZero(node.Style.RowGap, crossSize)
		} else {
			crossGap = t.resolveDimensionOrZero(node.Style.Gap, crossSize)
		}
	} else {
		if node.Style.RowGap.Kind != DimensionAuto {
			mainGap = t.resolveDimensionOrZero(node.Style.RowGap, mainSize)
		} else {
			mainGap = t.resolveDimensionOrZero(node.Style.Gap, mainSize)
		}
		if node.Style.ColumnGap.Kind != DimensionAuto {
			crossGap = t.resolveDimensionOrZero(node.Style.ColumnGap, crossSize)
		} else {
			crossGap = t.resolveDimensionOrZero(node.Style.Gap, crossSize)
		}
	}
	gap := mainGap

	for i := range items {
		it := &items[i]
		cs := &t.nodes[it.node].Style

		// Resolve flex basis.
		basis := t.resolveFlexBasis(it.node, cs, isRow, mainSize)

		// Resolve margins on the main axis for this item.
		var marginMainStart, marginMainEnd float32
		if isRow {
			marginMainStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
			marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
		} else {
			marginMainStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
			marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
		}

		// Apply min/max constraints on the main axis.
		hypo := basis
		if isRow {
			hypo = t.clampSize(hypo, cs.MinWidth, cs.MaxWidth, mainSize)
		} else {
			hypo = t.clampSize(hypo, cs.MinHeight, cs.MaxHeight, mainSize)
		}
		if hypo < 0 {
			hypo = 0
		}

		it.flexBasis = basis
		it.hypotheticalMain = hypo + marginMainStart + marginMainEnd
		it.targetMain = hypo
	}

	// --- 4–5. Determine lines (single-line vs multi-line) ---
	var linesBuf [8]flexLine
	lines := linesBuf[:0]
	wrap := node.Style.FlexWrap

	if wrap == FlexWrapNoWrap {
		// Single line: all items.
		var totalMain float32
		for i := range items {
			totalMain += items[i].hypotheticalMain
		}
		totalMain += gap * float32(len(items)-1)
		lines = append(lines, flexLine{
			startIdx: 0,
			count:    len(items),
			mainUsed: totalMain,
		})
	} else {
		// Multi-line wrapping.
		lineStart := 0
		var lineMain float32
		for i := range items {
			itemMain := items[i].hypotheticalMain
			g := gap
			if i == lineStart {
				g = 0
			}
			if i > lineStart && lineMain+g+itemMain > mainSize {
				// Finish current line.
				lines = append(lines, flexLine{
					startIdx: lineStart,
					count:    i - lineStart,
					mainUsed: lineMain,
				})
				lineStart = i
				lineMain = itemMain
			} else {
				lineMain += g + itemMain
			}
		}
		// Last line.
		lines = append(lines, flexLine{
			startIdx: lineStart,
			count:    len(items) - lineStart,
			mainUsed: lineMain,
		})
	}

	// Reverse lines for WrapReverse.
	if wrap == FlexWrapWrapReverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	// --- 6. Distribute free space per line (W3C §9.7) ---
	for li := range lines {
		line := &lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]
		totalGaps := gap * float32(line.count-1)

		isUnconstrainedAuto := id == t.root && ((isRow && node.Style.Width.Kind == DimensionAuto) || (!isRow && node.Style.Height.Kind == DimensionAuto))
		if isUnconstrainedAuto {
			for i := range lineItems {
				cs := &t.nodes[lineItems[i].node].Style
				target := lineItems[i].flexBasis
				if isRow {
					target = t.clampSize(target, cs.MinWidth, cs.MaxWidth, mainSize)
				} else {
					target = t.clampSize(target, cs.MinHeight, cs.MaxHeight, mainSize)
				}
				lineItems[i].targetMain = target
			}
		} else {
			for i := range lineItems {
				cs := &t.nodes[lineItems[i].node].Style
				target := lineItems[i].flexBasis
				if isRow {
					target = t.clampSize(target, cs.MinWidth, cs.MaxWidth, mainSize)
				} else {
					target = t.clampSize(target, cs.MinHeight, cs.MaxHeight, mainSize)
				}
				lineItems[i].targetMain = target
			}
			frozen := make([]bool, len(lineItems))
			for pass := 0; pass < len(lineItems); pass++ {
				var usedSpace float32
				var unfrozenGrow, unfrozenWeightedShrink float32
				unfrozenCount := 0
				for i := range lineItems {
					if frozen[i] {
						usedSpace += lineItems[i].targetMain
					} else {
						unfrozenCount++
						usedSpace += lineItems[i].flexBasis
						unfrozenGrow += lineItems[i].flexGrow
						unfrozenWeightedShrink += lineItems[i].flexShrink * lineItems[i].flexBasis
					}
				}
				if unfrozenCount == 0 {
					break
				}
				freeSpace := mainSize - totalGaps - usedSpace

				violationFound := false
				if freeSpace > 0 && unfrozenGrow > 0 {
					for i := range lineItems {
						if frozen[i] {
							continue
						}
						target := lineItems[i].flexBasis + freeSpace*(lineItems[i].flexGrow/unfrozenGrow)
						minLimit := t.computeMinContentSize(lineItems[i].node, isRow, mainSize, crossSize)
						maxLimit := t.computeMaxContentSize(lineItems[i].node, isRow, mainSize)
						if target < minLimit {
							target = minLimit
							frozen[i] = true
							violationFound = true
						} else if maxLimit >= 0 && target > maxLimit {
							target = maxLimit
							frozen[i] = true
							violationFound = true
						}
						lineItems[i].targetMain = target
					}
				} else if freeSpace < 0 && unfrozenWeightedShrink > 0 {
					overflow := -freeSpace
					for i := range lineItems {
						if frozen[i] {
							continue
						}
						ratio := (lineItems[i].flexShrink * lineItems[i].flexBasis) / unfrozenWeightedShrink
						target := lineItems[i].flexBasis - overflow*ratio
						minLimit := t.computeMinContentSize(lineItems[i].node, isRow, mainSize, crossSize)
						maxLimit := t.computeMaxContentSize(lineItems[i].node, isRow, mainSize)
						if target < minLimit {
							target = minLimit
							frozen[i] = true
							violationFound = true
						} else if maxLimit >= 0 && target > maxLimit {
							target = maxLimit
							frozen[i] = true
							violationFound = true
						}
						lineItems[i].targetMain = target
					}
				}
				if !violationFound {
					break
				}
			}
		}

		for i := range lineItems {
			if lineItems[i].targetMain < 0 {
				lineItems[i].targetMain = 0
			}
		}

		// --- 7. Justify-content: compute main-axis offsets ---
		var totalUsed float32
		for i := range lineItems {
			totalUsed += lineItems[i].hypotheticalMainWithTarget(isRow, &t.nodes[lineItems[i].node].Style, parentWidth, parentHeight, t)
		}
		totalUsed += totalGaps
		line.mainUsed = totalUsed

		remaining := mainSize - totalUsed
		if isUnconstrainedAuto {
			remaining = 0
		}

		// W3C §8.1: Check for auto margins along main axis
		autoMarginCount := 0
		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style
			if isRow {
				if isAutoMargin(cs.Margin.Left) {
					autoMarginCount++
				}
				if isAutoMargin(cs.Margin.Right) {
					autoMarginCount++
				}
			} else {
				if isAutoMargin(cs.Margin.Top) {
					autoMarginCount++
				}
				if isAutoMargin(cs.Margin.Bottom) {
					autoMarginCount++
				}
			}
		}

		var autoMarginShare float32
		if autoMarginCount > 0 && remaining > 0 {
			autoMarginShare = remaining / float32(autoMarginCount)
			remaining = 0 // Auto margins absorb all positive free space
		}

		var offset, spaceBetween, spaceBefore float32

		if remaining < 0 {
			switch node.Style.JustifyContent {
			case JustifyFlexEnd:
				offset = remaining
			case JustifyCenter, JustifySpaceAround, JustifySpaceEvenly:
				offset = remaining / 2
			default:
				offset = 0
			}
		} else {
			switch node.Style.JustifyContent {
			case JustifyFlexStart:
				offset = 0
			case JustifyFlexEnd:
				offset = remaining
			case JustifyCenter:
				offset = remaining / 2
			case JustifySpaceBetween:
				if line.count > 1 {
					spaceBetween = remaining / float32(line.count-1)
				}
			case JustifySpaceAround:
				if line.count > 0 {
					spaceBefore = remaining / float32(line.count) / 2
					spaceBetween = remaining / float32(line.count)
					offset = spaceBefore
				}
			case JustifySpaceEvenly:
				if line.count > 0 {
					spaceBefore = remaining / float32(line.count+1)
					spaceBetween = spaceBefore
					offset = spaceBefore
				}
			}
		}

		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style
			var marginMainStart, marginMainEnd float32
			if isRow {
				if isAutoMargin(cs.Margin.Left) {
					marginMainStart = autoMarginShare
				} else {
					marginMainStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
				}
				if isAutoMargin(cs.Margin.Right) {
					marginMainEnd = autoMarginShare
				} else {
					marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
				}
			} else {
				if isAutoMargin(cs.Margin.Top) {
					marginMainStart = autoMarginShare
				} else {
					marginMainStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
				}
				if isAutoMargin(cs.Margin.Bottom) {
					marginMainEnd = autoMarginShare
				} else {
					marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
				}
			}
			lineItems[i].mainOffset = offset + marginMainStart
			offset += marginMainStart + lineItems[i].targetMain + marginMainEnd + gap + spaceBetween
		}
	}

	// --- Resolve cross sizes for each item and line cross sizes ---
	for li := range lines {
		line := &lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]

		var maxCross float32
		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style
			var cross float32
			if isRow {
				if node.Style.Height.Kind == DimensionAuto && cs.Height.Kind == DimensionPercent {
					cross = 0
				} else {
					cross = t.resolveDimension(cs.Height, crossSize)
				}
				if cs.Height.Kind == DimensionAuto || (node.Style.Height.Kind == DimensionAuto && cs.Height.Kind == DimensionPercent) {
					if cs.AspectRatio > 0 {
						cross = lineItems[i].targetMain / cs.AspectRatio
					} else if int(lineItems[i].node) < len(t.measureFuncs) && t.measureFuncs[lineItems[i].node] != nil {
						fn := t.measureFuncs[lineItems[i].node]
						_, mh := fn(t, lineItems[i].node, lineItems[i].targetMain, MeasureExact, crossSize, MeasureAtMost)
						cross = mh
					} else {
						cross = t.computeIntrinsicCross(lineItems[i].node, isRow, lineItems[i].targetMain)
					}
				}
				cross = t.clampSize(cross, cs.MinHeight, cs.MaxHeight, crossSize)
			} else {
				if node.Style.Width.Kind == DimensionAuto && cs.Width.Kind == DimensionPercent {
					cross = 0
				} else {
					cross = t.resolveDimension(cs.Width, crossSize)
				}
				if cs.Width.Kind == DimensionAuto || (node.Style.Width.Kind == DimensionAuto && cs.Width.Kind == DimensionPercent) {
					if cs.AspectRatio > 0 {
						cross = lineItems[i].targetMain * cs.AspectRatio
					} else if int(lineItems[i].node) < len(t.measureFuncs) && t.measureFuncs[lineItems[i].node] != nil {
						fn := t.measureFuncs[lineItems[i].node]
						mw, _ := fn(t, lineItems[i].node, crossSize, MeasureAtMost, lineItems[i].targetMain, MeasureExact)
						cross = mw
					} else {
						cross = t.computeIntrinsicCross(lineItems[i].node, isRow, lineItems[i].targetMain)
					}
				}
				cross = t.clampSize(cross, cs.MinWidth, cs.MaxWidth, crossSize)
			}
			if cross < 0 {
				cross = 0
			}

			// Add cross-axis margins.
			var marginCrossStart, marginCrossEnd float32
			if isRow {
				marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
				marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
			} else {
				marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
				marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
			}

			lineItems[i].crossSize = cross
			totalCross := cross + marginCrossStart + marginCrossEnd
			if totalCross > maxCross {
				maxCross = totalCross
			}
		}
		line.crossSize = maxCross
	}

	// --- 8. Align-content for lines (W3C §8.4) ---
	// For single-line containers without wrapping, the line's cross size
	// is the full container cross size if the container has a definite cross size.
	isAutoCross := id == t.root && ((isRow && node.Style.Height.Kind == DimensionAuto) || (!isRow && node.Style.Width.Kind == DimensionAuto))
	hasMinCross := (isRow && node.Style.MinHeight.Kind != DimensionAuto) || (!isRow && node.Style.MinWidth.Kind != DimensionAuto)

	var maxCrossVal float32 = -1
	if isRow {
		if node.Style.MaxHeight.Kind != DimensionAuto {
			maxCrossVal = t.resolveDimension(node.Style.MaxHeight, parentHeight)
		}
	} else {
		if node.Style.MaxWidth.Kind != DimensionAuto {
			maxCrossVal = t.resolveDimension(node.Style.MaxWidth, parentWidth)
		}
	}

	if len(lines) == 1 && node.Style.FlexWrap == FlexWrapNoWrap {
		if !isAutoCross {
			lines[0].crossSize = crossSize
		} else if hasMinCross && crossSize > lines[0].crossSize {
			lines[0].crossSize = crossSize
		}
		if maxCrossVal >= 0 && lines[0].crossSize > maxCrossVal {
			lines[0].crossSize = maxCrossVal
		}
	}

	var totalLineCross float32
	for li := range lines {
		totalLineCross += lines[li].crossSize
	}
	if len(lines) > 1 {
		totalLineCross += float32(len(lines)-1) * crossGap
	}

	crossFreeSpace := crossSize - totalLineCross
	if isAutoCross && crossFreeSpace > 0 {
		crossFreeSpace = 0
	}

	{
		var lineOffset, lineSpacing float32
		nLines := len(lines)

		if crossFreeSpace < 0 {
			switch node.Style.AlignContent {
			case AlignContentCenter:
				lineOffset = crossFreeSpace / 2
			case AlignContentFlexEnd:
				lineOffset = crossFreeSpace
			}
		} else if crossFreeSpace > 0 {
			switch node.Style.AlignContent {
			case AlignContentStretch:
				if nLines > 0 {
					extra := crossFreeSpace / float32(nLines)
					for li := range lines {
						lines[li].crossSize += extra
					}
					crossFreeSpace = 0
				}
			case AlignContentFlexStart:
				// lineOffset = 0
			case AlignContentFlexEnd:
				lineOffset = crossFreeSpace
			case AlignContentCenter:
				lineOffset = crossFreeSpace / 2
			case AlignContentSpaceBetween:
				if nLines > 1 {
					lineSpacing = crossFreeSpace / float32(nLines-1)
				}
			case AlignContentSpaceAround:
				if nLines > 0 {
					spaceBefore := crossFreeSpace / float32(nLines) / 2
					lineSpacing = crossFreeSpace / float32(nLines)
					lineOffset = spaceBefore
				}
			case AlignContentSpaceEvenly:
				if nLines > 0 {
					space := crossFreeSpace / float32(nLines+1)
					lineOffset = space
					lineSpacing = space
				}
			}
		}

		for li := range lines {
			lines[li].lineOffset = lineOffset
			lineOffset += lines[li].crossSize + lineSpacing + crossGap
		}

		if node.Style.FlexWrap == FlexWrapWrapReverse && crossSize > 0 {
			for li := range lines {
				lines[li].lineOffset = crossSize - lines[li].lineOffset - lines[li].crossSize
			}
		}
	}

	// --- 9. Align-items / align-self along cross axis within each line ---

	for li := range lines {
		line := &lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]

		// Find max baseline for items using baseline alignment
		var maxBaseline float32
		for i := range lineItems {
			it := &lineItems[i]
			cs := &t.nodes[it.node].Style
			align := node.Style.AlignItems
			switch cs.AlignSelf {
			case AlignSelfStretch:
				align = AlignStretch
			case AlignSelfFlexStart:
				align = AlignFlexStart
			case AlignSelfFlexEnd:
				align = AlignFlexEnd
			case AlignSelfCenter:
				align = AlignCenter
			case AlignSelfBaseline:
				align = AlignBaseline
			}

			if align == AlignBaseline {
				b := it.crossSize
				if int(it.node) < len(t.baselines) && t.baselines[it.node] >= 0 {
					b = t.baselines[it.node]
				}
				var mStart float32
				if isRow {
					mStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
				} else {
					mStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
				}
				if b+mStart > maxBaseline {
					maxBaseline = b + mStart
				}
			}
		}

		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style

			// Determine effective alignment.
			align := node.Style.AlignItems
			switch cs.AlignSelf {
			case AlignSelfStretch:
				align = AlignStretch
			case AlignSelfFlexStart:
				align = AlignFlexStart
			case AlignSelfFlexEnd:
				align = AlignFlexEnd
			case AlignSelfCenter:
				align = AlignCenter
			case AlignSelfBaseline:
				align = AlignBaseline
			}

			var marginCrossStart, marginCrossEnd float32
			hasAutoCrossStart := false
			hasAutoCrossEnd := false
			if isRow {
				if isAutoMargin(cs.Margin.Top) {
					hasAutoCrossStart = true
				} else {
					marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
				}
				if isAutoMargin(cs.Margin.Bottom) {
					hasAutoCrossEnd = true
				} else {
					marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
				}
			} else {
				if isAutoMargin(cs.Margin.Left) {
					hasAutoCrossStart = true
				} else {
					marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
				}
				if isAutoMargin(cs.Margin.Right) {
					hasAutoCrossEnd = true
				} else {
					marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
				}
			}

			availCross := line.crossSize - marginCrossStart - marginCrossEnd
			if availCross < 0 {
				availCross = 0
			}

			if hasAutoCrossStart || hasAutoCrossEnd {
				// W3C §8.1: Auto cross margins absorb free cross space and override alignment
				freeCross := line.crossSize - lineItems[i].crossSize - marginCrossStart - marginCrossEnd
				if freeCross > 0 {
					if hasAutoCrossStart && hasAutoCrossEnd {
						marginCrossStart += freeCross / 2
					} else if hasAutoCrossStart {
						marginCrossStart += freeCross
					}
				}
				lineItems[i].crossOffset = marginCrossStart
			} else {
				switch align {
				case AlignStretch:
					if cs.AspectRatio == 0 && (lineItems[i].crossSize == 0 || (isRow && cs.Height.Kind == DimensionAuto && availCross > 0) || (!isRow && cs.Width.Kind == DimensionAuto && availCross > 0)) {
						lineItems[i].crossSize = availCross
					}
					lineItems[i].crossOffset = marginCrossStart
				case AlignFlexStart:
					lineItems[i].crossOffset = marginCrossStart
				case AlignBaseline:
					b := lineItems[i].crossSize
					if int(lineItems[i].node) < len(t.baselines) && t.baselines[lineItems[i].node] >= 0 {
						b = t.baselines[lineItems[i].node]
					}
					lineItems[i].crossOffset = marginCrossStart + (maxBaseline - b)
				case AlignFlexEnd:
					lineItems[i].crossOffset = line.crossSize - lineItems[i].crossSize - marginCrossEnd
				case AlignCenter:
					lineItems[i].crossOffset = marginCrossStart + (availCross-lineItems[i].crossSize)/2
				}
			}
		}
	}

	// --- 10–14. Set computed rects & recurse ---
	for li := range lines {
		line := &lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]

		for i := range lineItems {
			it := &lineItems[i]
			cn := &t.nodes[it.node]

			var x, y, w, h float32
			if isRow {
				x = padLeft + it.mainOffset
				y = padTop + line.lineOffset + it.crossOffset
				w = it.targetMain
				h = it.crossSize
			} else {
				x = padLeft + line.lineOffset + it.crossOffset
				y = padTop + it.mainOffset
				w = it.crossSize
				h = it.targetMain
			}

			if w < 0 {
				w = 0
			}
			if h < 0 {
				h = 0
			}

			// Handle reverse: mirror main-axis positions.
			if isReverse {
				if isRow {
					x = padLeft + contentWidth - it.mainOffset - w
				} else {
					y = padTop + contentHeight - it.mainOffset - h
				}
			}

			// Relative positioning offsets
			if cn.Style.Position == PositionRelative {
				top := t.resolveDimensionOrZero(cn.Style.Inset.Top, parentHeight)
				bottom := t.resolveDimensionOrZero(cn.Style.Inset.Bottom, parentHeight)
				left := t.resolveDimensionOrZero(cn.Style.Inset.Left, parentWidth)
				right := t.resolveDimensionOrZero(cn.Style.Inset.Right, parentWidth)
				if cn.Style.Inset.Left.Kind != DimensionAuto {
					x += left
				} else if cn.Style.Inset.Right.Kind != DimensionAuto {
					x -= right
				}
				if cn.Style.Inset.Top.Kind != DimensionAuto {
					y += top
				} else if cn.Style.Inset.Bottom.Kind != DimensionAuto {
					y -= bottom
				}
			}

			cn.Computed.X = x
			cn.Computed.Y = y
			cn.Computed.Width = w
			cn.Computed.Height = h

			// --- 14. Recursive layout for flex container children ---
			if cn.Style.Display == DisplayFlex && cn.FirstChild != InvalidNode {
				padL := t.resolveDimensionOrZero(cn.Style.Padding.Left, w)
				padT := t.resolveDimensionOrZero(cn.Style.Padding.Top, h)
				padR := t.resolveDimensionOrZero(cn.Style.Padding.Right, w)
				padB := t.resolveDimensionOrZero(cn.Style.Padding.Bottom, h)
				cw := w - padL - padR
				ch := h - padT - padB
				if cw < 0 {
					cw = 0
				}
				if ch < 0 {
					ch = 0
				}
				t.layoutFlex(it.node, cw, ch, padL, padT, w, h)
				cn.Computed.X = x
				cn.Computed.Y = y
				cn.Computed.Width = w
				cn.Computed.Height = h
			}
		}
	}

	// Shrink-to-fit auto container dimensions
	var maxLineMain float32
	for li := range lines {
		if lines[li].mainUsed > maxLineMain {
			maxLineMain = lines[li].mainUsed
		}
	}
	var actualContentWidth, actualContentHeight float32
	if isRow {
		actualContentWidth = maxLineMain
		actualContentHeight = totalLineCross
	} else {
		actualContentWidth = totalLineCross
		actualContentHeight = maxLineMain
	}

	padRight := t.resolveDimensionOrZero(node.Style.Padding.Right, parentWidth)
	padBottom := t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentHeight)

	if node.Style.Width.Kind == DimensionAuto && actualContentWidth > 0 {
		node.Computed.Width = t.clampSize(actualContentWidth+padLeft+padRight, node.Style.MinWidth, node.Style.MaxWidth, parentWidth)
	}
	if node.Style.Height.Kind == DimensionAuto && actualContentHeight > 0 {
		node.Computed.Height = t.clampSize(actualContentHeight+padTop+padBottom, node.Style.MinHeight, node.Style.MaxHeight, parentHeight)
	}

	// Resolve deferred percentage dimensions in auto containers
	if node.Style.Height.Kind == DimensionAuto && actualContentHeight > 0 {
		for i := range items {
			cnode := &t.nodes[items[i].node]
			if cnode.Style.Height.Kind == DimensionPercent {
				cnode.Computed.Height = actualContentHeight * cnode.Style.Height.Value / 100
			}
		}
	}
	if node.Style.Width.Kind == DimensionAuto && actualContentWidth > 0 {
		for i := range items {
			cnode := &t.nodes[items[i].node]
			if cnode.Style.Width.Kind == DimensionPercent {
				cnode.Computed.Width = actualContentWidth * cnode.Style.Width.Value / 100
			}
		}
	}

	// Layout absolutely-positioned children
	t.layoutAbsoluteChildren(id, contentWidth, contentHeight, padLeft, padTop)
}

// layoutAbsoluteChildren computes the layout of children positioned with PositionAbsolute.
func (t *Tree) layoutAbsoluteChildren(id NodeID, contentWidth, contentHeight, padLeft, padTop float32) {
	child := t.nodes[id].FirstChild
	for child != InvalidNode {
		c := &t.nodes[child]
		if c.Style.Display != DisplayNone && c.Style.Position == PositionAbsolute {
			t.computeNode(child, contentWidth, contentHeight)

			var x, y float32
			w := c.Computed.Width
			h := c.Computed.Height

			// Horizontal positioning
			hasLeft := c.Style.Inset.Left.Kind != DimensionAuto
			hasRight := c.Style.Inset.Right.Kind != DimensionAuto
			left := t.resolveDimensionOrZero(c.Style.Inset.Left, contentWidth)
			right := t.resolveDimensionOrZero(c.Style.Inset.Right, contentWidth)

			hasAutoMarginLeft := isAutoMargin(c.Style.Margin.Left)
			hasAutoMarginRight := isAutoMargin(c.Style.Margin.Right)
			hasAutoMarginTop := isAutoMargin(c.Style.Margin.Top)
			hasAutoMarginBottom := isAutoMargin(c.Style.Margin.Bottom)

			if hasLeft && hasRight && c.Style.Width.Kind == DimensionAuto {
				x = padLeft + left
				w = contentWidth - left - right
				if w < 0 {
					w = 0
				}
				c.Computed.Width = w
			} else if hasLeft && hasRight && (hasAutoMarginLeft || hasAutoMarginRight) {
				freeW := contentWidth - left - right - w
				if freeW > 0 {
					if hasAutoMarginLeft && hasAutoMarginRight {
						x = padLeft + left + freeW/2
					} else if hasAutoMarginLeft {
						x = padLeft + contentWidth - right - w
					} else {
						x = padLeft + left
					}
				} else {
					x = padLeft + left
				}
			} else if hasLeft {
				x = padLeft + left
			} else if hasRight {
				x = padLeft + contentWidth - right - w
			} else {
				x = padLeft
			}

			// Vertical positioning
			hasTop := c.Style.Inset.Top.Kind != DimensionAuto
			hasBottom := c.Style.Inset.Bottom.Kind != DimensionAuto
			top := t.resolveDimensionOrZero(c.Style.Inset.Top, contentHeight)
			bottom := t.resolveDimensionOrZero(c.Style.Inset.Bottom, contentHeight)

			if hasTop && hasBottom && c.Style.Height.Kind == DimensionAuto {
				y = padTop + top
				h = contentHeight - top - bottom
				if h < 0 {
					h = 0
				}
				c.Computed.Height = h
			} else if hasTop && hasBottom && (hasAutoMarginTop || hasAutoMarginBottom) {
				freeH := contentHeight - top - bottom - h
				if freeH > 0 {
					if hasAutoMarginTop && hasAutoMarginBottom {
						y = padTop + top + freeH/2
					} else if hasAutoMarginTop {
						y = padTop + contentHeight - bottom - h
					} else {
						y = padTop + top
					}
				} else {
					y = padTop + top
				}
			} else if hasTop {
				y = padTop + top
			} else if hasBottom {
				y = padTop + contentHeight - bottom - h
			} else {
				y = padTop
			}

			c.Computed.X = x
			c.Computed.Y = y
		}
		child = c.NextSibling
	}
}

// computeMinContentSize returns the intrinsic automatic minimum size per W3C Flexbox §4.5.
func (t *Tree) computeMinContentSize(node NodeID, isRow bool, mainSize, crossSize float32) float32 {
	cs := &t.nodes[node].Style
	if isRow {
		if cs.MinWidth.Kind == DimensionPx || cs.MinWidth.Kind == DimensionPercent {
			return t.resolveDimensionOrZero(cs.MinWidth, mainSize)
		}
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			mw, _ := fn(t, node, 0, MeasureAtMost, crossSize, MeasureAtMost)
			return mw
		}
		cNode := &t.nodes[node]
		if cNode.FirstChild != InvalidNode {
			var maxChild float32
			for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
				chS := &t.nodes[ch].Style
				cw := t.resolveDimensionOrZero(chS.Width, mainSize)
				mL := t.resolveDimensionOrZero(chS.Margin.Left, mainSize)
				mR := t.resolveDimensionOrZero(chS.Margin.Right, mainSize)
				if cw+mL+mR > maxChild {
					maxChild = cw + mL + mR
				}
			}
			return maxChild
		}
		return 0
	} else {
		if cs.MinHeight.Kind == DimensionPx || cs.MinHeight.Kind == DimensionPercent {
			return t.resolveDimensionOrZero(cs.MinHeight, mainSize)
		}
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			_, mh := fn(t, node, crossSize, MeasureAtMost, 0, MeasureAtMost)
			return mh
		}
		cNode := &t.nodes[node]
		if cNode.FirstChild != InvalidNode {
			var maxChild float32
			for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
				chS := &t.nodes[ch].Style
				chH := t.resolveDimensionOrZero(chS.Height, mainSize)
				mT := t.resolveDimensionOrZero(chS.Margin.Top, mainSize)
				mB := t.resolveDimensionOrZero(chS.Margin.Bottom, mainSize)
				if chH+mT+mB > maxChild {
					maxChild = chH + mT + mB
				}
			}
			return maxChild
		}
		return 0
	}
}

func (t *Tree) computeMaxContentSize(node NodeID, isRow bool, mainSize float32) float32 {
	cs := &t.nodes[node].Style
	if isRow {
		if cs.MaxWidth.Kind == DimensionPx || cs.MaxWidth.Kind == DimensionPercent {
			return t.resolveDimension(cs.MaxWidth, mainSize)
		}
	} else {
		if cs.MaxHeight.Kind == DimensionPx || cs.MaxHeight.Kind == DimensionPercent {
			return t.resolveDimension(cs.MaxHeight, mainSize)
		}
	}
	if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
		fn := t.measureFuncs[node]
		mw, mh := fn(t, node, 0, MeasureUndefined, 0, MeasureUndefined)
		if isRow {
			return mw
		}
		return mh
	}
	return -1
}

// resolveFlexBasis determines the initial main-axis size of a flex item.
func (t *Tree) resolveFlexBasis(node NodeID, s *Style, isRow bool, mainSize float32) float32 {
	// 1. Explicit flex-basis takes precedence
	switch s.FlexBasis.Kind {
	case DimensionPx:
		return s.FlexBasis.Value
	case DimensionPercent:
		return mainSize * s.FlexBasis.Value / 100
	case DimensionContent, DimensionMaxContent:
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			mw, mh := fn(t, node, 0, MeasureUndefined, 0, MeasureUndefined)
			if isRow {
				return mw
			}
			return mh
		}
		return t.computeMaxContentSize(node, isRow, mainSize)
	case DimensionMinContent:
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			mw, mh := fn(t, node, 0, MeasureAtMost, 0, MeasureUndefined)
			if isRow {
				return mw
			}
			return mh
		}
		return t.computeMinContentSize(node, isRow, mainSize, 0)
	case DimensionFitContent:
		var minC, maxC float32
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			minW, _ := fn(t, node, 0, MeasureAtMost, 0, MeasureUndefined)
			maxW, _ := fn(t, node, 0, MeasureUndefined, 0, MeasureUndefined)
			minC = minW
			maxC = maxW
		}
		fit := mainSize
		if fit < minC {
			fit = minC
		}
		if fit > maxC {
			fit = maxC
		}
		return fit
	}

	// 2. Fall back to width/height depending on main axis
	var dim Dimension
	if isRow {
		dim = s.Width
	} else {
		dim = s.Height
	}
	switch dim.Kind {
	case DimensionPx:
		return dim.Value
	case DimensionPercent:
		return mainSize * dim.Value / 100
	}

	// 3. AspectRatio resolution when main axis size is Auto but cross axis is definite
	if s.AspectRatio > 0 {
		if isRow && s.Width.Kind == DimensionAuto && s.Height.Kind == DimensionPx {
			return s.Height.Value * s.AspectRatio
		} else if !isRow && s.Height.Kind == DimensionAuto && s.Width.Kind == DimensionPx {
			return s.Width.Value / s.AspectRatio
		}
	}
	if isRow {
		dim = s.Width
	} else {
		dim = s.Height
	}
	switch dim.Kind {
	case DimensionPx:
		return dim.Value
	case DimensionPercent:
		return mainSize * dim.Value / 100
	}
	// 3. Intrinsic measure func if registered
	if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
		fn := t.measureFuncs[node]
		mw, mh := fn(t, node, mainSize, MeasureAtMost, 0, MeasureUndefined)
		if isRow {
			return mw
		}
		return mh
	}

	// 4. Intrinsic content size for container nodes
	cNode := &t.nodes[node]
	if cNode.FirstChild != InvalidNode && cNode.Style.Display == DisplayFlex {
		isChildRow := cNode.Style.FlexDirection == FlexDirectionRow || cNode.Style.FlexDirection == FlexDirectionRowReverse
		gap := t.resolveDimensionOrZero(cNode.Style.Gap, mainSize)
		var totalMain float32
		var maxCross float32
		childCount := 0
		for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
			childCount++
			chS := &t.nodes[ch].Style
			cw := t.resolveDimensionOrZero(chS.Width, mainSize)
			chH := t.resolveDimensionOrZero(chS.Height, mainSize)
			if chS.AspectRatio > 0 {
				if cw > 0 && chH == 0 {
					chH = cw / chS.AspectRatio
				} else if chH > 0 && cw == 0 {
					cw = chH * chS.AspectRatio
				}
			}
			mLeft := t.resolveDimensionOrZero(chS.Margin.Left, mainSize)
			mRight := t.resolveDimensionOrZero(chS.Margin.Right, mainSize)
			mTop := t.resolveDimensionOrZero(chS.Margin.Top, mainSize)
			mBottom := t.resolveDimensionOrZero(chS.Margin.Bottom, mainSize)

			if isChildRow {
				totalMain += cw + mLeft + mRight
				if chH+mTop+mBottom > maxCross {
					maxCross = chH + mTop + mBottom
				}
			} else {
				totalMain += chH + mTop + mBottom
				if cw+mLeft+mRight > maxCross {
					maxCross = cw + mLeft + mRight
				}
			}
		}
		if childCount > 1 {
			totalMain += float32(childCount-1) * gap
		}
		if isRow == isChildRow {
			return totalMain
		}
		return maxCross
	}

	return 0
}

func (t *Tree) computeIntrinsicCross(node NodeID, isParentRow bool, targetMain float32) float32 {
	cNode := &t.nodes[node]
	if cNode.FirstChild == InvalidNode || cNode.Style.Display != DisplayFlex {
		return 0
	}
	isChildRow := cNode.Style.FlexDirection == FlexDirectionRow || cNode.Style.FlexDirection == FlexDirectionRowReverse
	var maxCross float32
	var totalChildGrow float32
	childCount := 0
	for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
		childCount++
		chS := &t.nodes[ch].Style
		totalChildGrow += chS.FlexGrow
	}
	gap := t.resolveDimensionOrZero(cNode.Style.Gap, targetMain)
	totalGaps := float32(0)
	if childCount > 1 {
		totalGaps = float32(childCount-1) * gap
	}
	availForChildren := targetMain - totalGaps

	for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
		chS := &t.nodes[ch].Style
		cw := t.resolveDimensionOrZero(chS.Width, targetMain)
		chH := t.resolveDimensionOrZero(chS.Height, targetMain)
		if isChildRow && totalChildGrow > 0 && chS.FlexGrow > 0 {
			cw = availForChildren * (chS.FlexGrow / totalChildGrow)
		}
		if chS.AspectRatio > 0 {
			if cw > 0 && chH == 0 {
				chH = cw / chS.AspectRatio
			} else if chH > 0 && cw == 0 {
				cw = chH * chS.AspectRatio
			}
		}
		mTop := t.resolveDimensionOrZero(chS.Margin.Top, targetMain)
		mBottom := t.resolveDimensionOrZero(chS.Margin.Bottom, targetMain)
		mLeft := t.resolveDimensionOrZero(chS.Margin.Left, targetMain)
		mRight := t.resolveDimensionOrZero(chS.Margin.Right, targetMain)

		if isParentRow {
			if isChildRow {
				if chH+mTop+mBottom > maxCross {
					maxCross = chH + mTop + mBottom
				}
			} else {
				if cw+mLeft+mRight > maxCross {
					maxCross = cw + mLeft + mRight
				}
			}
		} else {
			// Parent is column
			if isChildRow {
				if chH+mTop+mBottom > maxCross {
					maxCross = chH + mTop + mBottom
				}
			} else {
				if cw+mLeft+mRight > maxCross {
					maxCross = cw + mLeft + mRight
				}
			}
		}
	}
	return maxCross
}

// resolveDimension resolves a single Dimension against a parent reference size.
// Returns -1 for Auto dimensions.
func (t *Tree) resolveDimension(d Dimension, parentRef float32) float32 {
	switch d.Kind {
	case DimensionPx:
		return d.Value
	case DimensionPercent:
		return parentRef * d.Value / 100
	default:
		return -1
	}
}

// resolveDimensionOrZero resolves a dimension against parentRef, returning 0 for Auto.
func (t *Tree) resolveDimensionOrZero(d Dimension, parentRef float32) float32 {
	switch d.Kind {
	case DimensionPx:
		return d.Value
	case DimensionPercent:
		return parentRef * d.Value / 100
	default:
		return 0
	}
}

// clampSize applies min/max constraints to a size value.
func (t *Tree) clampSize(size float32, minD, maxD Dimension, parentRef float32) float32 {
	if minD.Kind != DimensionAuto {
		minV := t.resolveDimension(minD, parentRef)
		if minV >= 0 && size < minV {
			size = minV
		}
	}
	if maxD.Kind != DimensionAuto {
		maxV := t.resolveDimension(maxD, parentRef)
		if maxV >= 0 && size > maxV {
			size = maxV
		}
	}
	return size
}

// hypotheticalMainWithTarget returns the main-axis size including margins
// but using the resolved targetMain instead of the original hypothetical.
func (fi *flexItem) hypotheticalMainWithTarget(isRow bool, s *Style, parentW, parentH float32, t *Tree) float32 {
	var marginMainStart, marginMainEnd float32
	if isRow {
		marginMainStart = t.resolveDimensionOrZero(s.Margin.Left, parentW)
		marginMainEnd = t.resolveDimensionOrZero(s.Margin.Right, parentW)
	} else {
		marginMainStart = t.resolveDimensionOrZero(s.Margin.Top, parentH)
		marginMainEnd = t.resolveDimensionOrZero(s.Margin.Bottom, parentH)
	}
	return fi.targetMain + marginMainStart + marginMainEnd
}
