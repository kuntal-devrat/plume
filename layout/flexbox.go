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

	// If width/height are auto and we have a definite parent, default to
	// the parent size (block-level behaviour for the root / non-flex items).
	if node.Style.Width.Kind == DimensionAuto && width < 0 {
		width = parentWidth
	}
	if node.Style.Height.Kind == DimensionAuto && height < 0 {
		height = parentHeight
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

	// --- 2. Collect flex items (reuse scratch) ---
	t.scratch = t.scratch[:0]
	child := node.FirstChild
	for child != InvalidNode {
		c := &t.nodes[child]
		if c.Style.Display != DisplayNone {
			fi := flexItem{
				node:     child,
				flexGrow: c.Style.FlexGrow,
				flexShrink: c.Style.FlexShrink,
			}
			t.scratch = append(t.scratch, fi)
		}
		child = c.NextSibling
	}

	items := t.scratch
	if len(items) == 0 {
		return
	}

	// --- 3. Resolve flex basis & hypothetical main size ---
	gap := t.resolveDimensionOrZero(node.Style.Gap, mainSize)

	for i := range items {
		it := &items[i]
		cs := &t.nodes[it.node].Style

		// Resolve flex basis.
		basis := t.resolveFlexBasis(cs, isRow, mainSize)

		// Resolve margins on the main axis for this item.
		var marginMainStart, marginMainEnd float32
		if isRow {
			marginMainStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
			marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
		} else {
			marginMainStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
			marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
		}
		if marginMainStart < 0 {
			marginMainStart = 0
		}
		if marginMainEnd < 0 {
			marginMainEnd = 0
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
	t.lines = t.lines[:0]
	wrap := node.Style.FlexWrap

	if wrap == FlexWrapNoWrap {
		// Single line: all items.
		var totalMain float32
		for i := range items {
			totalMain += items[i].hypotheticalMain
		}
		totalMain += gap * float32(len(items)-1)
		t.lines = append(t.lines, flexLine{
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
				t.lines = append(t.lines, flexLine{
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
		t.lines = append(t.lines, flexLine{
			startIdx: lineStart,
			count:    len(items) - lineStart,
			mainUsed: lineMain,
		})
	}

	// Reverse lines for WrapReverse.
	if wrap == FlexWrapWrapReverse {
		for i, j := 0, len(t.lines)-1; i < j; i, j = i+1, j-1 {
			t.lines[i], t.lines[j] = t.lines[j], t.lines[i]
		}
	}

	// --- 6. Distribute free space per line ---
	for li := range t.lines {
		line := &t.lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]
		totalGaps := gap * float32(line.count-1)
		freeSpace := mainSize - totalGaps

		// Sum of hypothetical main sizes (without margins for distribution,
		// but we need the inner sizes for grow/shrink).
		var sumTarget float32
		for i := range lineItems {
			sumTarget += lineItems[i].targetMain
			// Subtract margin contributions from freeSpace.
			freeSpace -= lineItems[i].hypotheticalMain
		}

		if freeSpace > 0 {
			// Positive free space → grow.
			var totalGrow float32
			for i := range lineItems {
				totalGrow += lineItems[i].flexGrow
			}
			if totalGrow > 0 {
				for i := range lineItems {
					lineItems[i].targetMain += freeSpace * (lineItems[i].flexGrow / totalGrow)
				}
			}
		} else if freeSpace < 0 {
			// Negative free space → shrink.
			overflow := -freeSpace
			var totalWeightedShrink float32
			for i := range lineItems {
				totalWeightedShrink += lineItems[i].flexShrink * lineItems[i].targetMain
			}
			if totalWeightedShrink > 0 {
				for i := range lineItems {
					ratio := (lineItems[i].flexShrink * lineItems[i].targetMain) / totalWeightedShrink
					lineItems[i].targetMain -= overflow * ratio
					if lineItems[i].targetMain < 0 {
						lineItems[i].targetMain = 0
					}
				}
			}
		}

		// Apply min/max constraints after flex resolution.
		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style
			if isRow {
				lineItems[i].targetMain = t.clampSize(lineItems[i].targetMain, cs.MinWidth, cs.MaxWidth, mainSize)
			} else {
				lineItems[i].targetMain = t.clampSize(lineItems[i].targetMain, cs.MinHeight, cs.MaxHeight, mainSize)
			}
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

		remaining := mainSize - totalUsed
		if remaining < 0 {
			remaining = 0
		}

		var offset, spaceBetween, spaceBefore float32

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

		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style
			var marginMainStart float32
			if isRow {
				marginMainStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
			} else {
				marginMainStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
			}
			lineItems[i].mainOffset = offset + marginMainStart
			var marginMainEnd float32
			if isRow {
				marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
			} else {
				marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
			}
			offset += marginMainStart + lineItems[i].targetMain + marginMainEnd + gap + spaceBetween
		}
	}

	// --- Resolve cross sizes for each item and line cross sizes ---
	for li := range t.lines {
		line := &t.lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]

		var maxCross float32
		for i := range lineItems {
			cs := &t.nodes[lineItems[i].node].Style
			var cross float32
			if isRow {
				cross = t.resolveDimension(cs.Height, crossSize)
				if cs.Height.Kind == DimensionAuto {
					cross = 0
				}
				cross = t.clampSize(cross, cs.MinHeight, cs.MaxHeight, crossSize)
			} else {
				cross = t.resolveDimension(cs.Width, crossSize)
				if cs.Width.Kind == DimensionAuto {
					cross = 0
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

	// --- 8. Align-items / align-self along cross axis ---
	// For single-line containers without wrapping, the line's cross size
	// is the full container cross size.
	if len(t.lines) == 1 && node.Style.FlexWrap == FlexWrapNoWrap {
		t.lines[0].crossSize = crossSize
	}

	for li := range t.lines {
		line := &t.lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]

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
			}

			var marginCrossStart, marginCrossEnd float32
			if isRow {
				marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Top, parentHeight)
				marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, parentHeight)
			} else {
				marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Left, parentWidth)
				marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Right, parentWidth)
			}

			availCross := line.crossSize - marginCrossStart - marginCrossEnd

			switch align {
			case AlignStretch:
				if lineItems[i].crossSize == 0 || (isRow && cs.Height.Kind == DimensionAuto) || (!isRow && cs.Width.Kind == DimensionAuto) {
					lineItems[i].crossSize = availCross
				}
				lineItems[i].crossOffset = marginCrossStart
			case AlignFlexStart, AlignBaseline:
				lineItems[i].crossOffset = marginCrossStart
			case AlignFlexEnd:
				lineItems[i].crossOffset = line.crossSize - lineItems[i].crossSize - marginCrossEnd
			case AlignCenter:
				lineItems[i].crossOffset = marginCrossStart + (availCross-lineItems[i].crossSize)/2
			}
		}
	}

	// --- 9. Align-content for multi-line containers ---
	var totalLineCross float32
	for li := range t.lines {
		totalLineCross += t.lines[li].crossSize
	}

	crossFreeSpace := crossSize - totalLineCross
	if crossFreeSpace < 0 {
		crossFreeSpace = 0
	}

	{
		var lineOffset, lineSpacing, lineBefore float32
		nLines := len(t.lines)

		switch node.Style.AlignContent {
		case AlignContentStretch:
			// Distribute extra space equally among lines.
			if nLines > 0 && crossFreeSpace > 0 {
				extra := crossFreeSpace / float32(nLines)
				for li := range t.lines {
					t.lines[li].crossSize += extra
				}
				crossFreeSpace = 0
			}
		case AlignContentFlexStart:
			// default: lineOffset = 0
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
				lineBefore = crossFreeSpace / float32(nLines) / 2
				lineSpacing = crossFreeSpace / float32(nLines)
				lineOffset = lineBefore
			}
		}

		for li := range t.lines {
			t.lines[li].lineOffset = lineOffset
			lineOffset += t.lines[li].crossSize + lineSpacing
		}
	}

	// --- 10–14. Set computed rects & recurse ---
	for li := range t.lines {
		line := &t.lines[li]
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

			// Handle reverse: mirror main-axis positions.
			if isReverse {
				if isRow {
					x = padLeft + contentWidth - it.mainOffset - w
				} else {
					y = padTop + contentHeight - it.mainOffset - h
				}
			}

			cn.Computed.X = x
			cn.Computed.Y = y
			cn.Computed.Width = w
			cn.Computed.Height = h

			// --- 14. Recursive layout for flex container children ---
			if cn.Style.Display == DisplayFlex && cn.FirstChild != InvalidNode {
				t.computeNode(it.node, w, h)
				// Restore position — computeNode may have adjusted Width/Height
				// via clamping, but X/Y must stay as we set them.
				cn.Computed.X = x
				cn.Computed.Y = y
			}
		}
	}
}

// resolveFlexBasis determines the initial main-axis size of a flex item.
func (t *Tree) resolveFlexBasis(s *Style, isRow bool, mainSize float32) float32 {
	// 1. Explicit flex-basis
	switch s.FlexBasis.Kind {
	case DimensionPx:
		return s.FlexBasis.Value
	case DimensionPercent:
		return mainSize * s.FlexBasis.Value / 100
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
	// 3. Content sizing — Phase 1 returns 0.
	return 0
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
