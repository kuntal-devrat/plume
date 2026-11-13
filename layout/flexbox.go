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

	// Resolve padding (in CSS, all 4 sides of percentage padding resolve against containing block width)
	padTop := t.resolveDimensionOrZero(node.Style.Padding.Top, parentWidth)
	padRight := t.resolveDimensionOrZero(node.Style.Padding.Right, parentWidth)
	padBottom := t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentWidth)
	padLeft := t.resolveDimensionOrZero(node.Style.Padding.Left, parentWidth)

	var boxPadHoriz, boxPadVert float32
	if node.Style.BoxSizing == BoxSizingContentBox {
		boxPadHoriz = padLeft + padRight
		boxPadVert = padTop + padBottom
	}
	if width >= 0 {
		width += boxPadHoriz
	}
	if height >= 0 {
		height += boxPadVert
	}

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
		if node.Style.Position == PositionAbsolute {
			if node.Style.MinWidth.Kind != DimensionAuto {
				width = t.resolveDimensionOrZero(node.Style.MinWidth, parentWidth)
			} else {
				width = 0
			}
		} else {
			width = parentWidth
		}
	}
	if node.Style.Height.Kind == DimensionAuto && height < 0 {
		if node.Style.Position == PositionAbsolute {
			if node.Style.MinHeight.Kind != DimensionAuto && node.FirstChild == InvalidNode {
				height = t.resolveDimensionOrZero(node.Style.MinHeight, parentHeight)
			} else {
				height = 0
			}
		} else if id != t.root {
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
	width = t.clampNodeSize(width, node.Style.MinWidth, node.Style.MaxWidth, parentWidth, boxPadHoriz)
	height = t.clampNodeSize(height, node.Style.MinHeight, node.Style.MaxHeight, parentHeight, boxPadVert)

	// Recompute content box after clamping.
	contentWidth = width - padLeft - padRight
	contentHeight = height - padTop - padBottom
	if contentWidth < 0 {
		contentWidth = 0
	}
	if contentHeight < 0 {
		contentHeight = 0
	}

	// For containers with auto height, layout must treat contentHeight as indefinite (0)
	// so that children's intrinsic sizes determine the container height before min/max clamping.
	if node.Style.Height.Kind == DimensionAuto && node.FirstChild != InvalidNode && (node.Style.Display == DisplayFlex || node.Style.Display == DisplayGrid) {
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
			childW := cw
			if c.Style.BoxSizing == BoxSizingContentBox && c.Style.Width.Kind == DimensionPercent {
				padH := t.resolveDimensionOrZero(c.Style.Padding.Left, cw) + t.resolveDimensionOrZero(c.Style.Padding.Right, cw)
				childW = cw - padH
			}
			t.computeNode(child, childW, ch)
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


	padRight := t.resolveDimensionOrZero(node.Style.Padding.Right, parentWidth)
	padBottom := t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentWidth)

	childMarginRef := contentWidth
	if childMarginRef <= 0 && parentWidth > 0 {
		childMarginRef = parentWidth
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

		basisRef := mainSize
		if !isRow && node.Style.Height.Kind == DimensionAuto && parentHeight <= 0 {
			basisRef = 0
		}
		// Resolve flex basis.
		basis := t.resolveFlexBasis(it.node, cs, isRow, basisRef, crossSize)

		// Main axis padding for content-box clamping
		var boxPadMain float32
		if cs.BoxSizing == BoxSizingContentBox {
			cbW := crossSize
			if isRow {
				cbW = mainSize
			}
			if isRow {
				boxPadMain = t.resolveDimensionOrZero(cs.Padding.Left, cbW) + t.resolveDimensionOrZero(cs.Padding.Right, cbW)
			} else {
				boxPadMain = t.resolveDimensionOrZero(cs.Padding.Top, cbW) + t.resolveDimensionOrZero(cs.Padding.Bottom, cbW)
			}
		}

		// Resolve margins on the main axis for this item.
		var marginMainStart, marginMainEnd float32
		if isRow {
			marginMainStart = t.resolveDimensionOrZero(cs.Margin.Left, childMarginRef)
			marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Right, childMarginRef)
		} else {
			marginMainStart = t.resolveDimensionOrZero(cs.Margin.Top, childMarginRef)
			marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, childMarginRef)
		}

		// Apply min/max constraints on the main axis.
		hypo := basis
		if isRow {
			hypo = t.clampNodeSize(hypo, cs.MinWidth, cs.MaxWidth, mainSize, boxPadMain)
			if cs.MinWidth.Kind == DimensionAuto && cs.OverflowX == OverflowVisible {
				autoMin := t.computeMinContentSize(it.node, true, mainSize, crossSize)
				if cs.Width.Kind == DimensionPx && autoMin > cs.Width.Value {
					autoMin = cs.Width.Value
				}
				if cs.MaxWidth.Kind != DimensionAuto {
					maxW := t.resolveDimension(cs.MaxWidth, mainSize)
					if maxW >= 0 {
						maxW += boxPadMain
						if autoMin > maxW {
							autoMin = maxW
						}
					}
				}
				if autoMin > 0 && hypo < autoMin {
					hypo = autoMin
				}
			}
		} else {
			hypo = t.clampNodeSize(hypo, cs.MinHeight, cs.MaxHeight, mainSize, boxPadMain)
			if cs.MinHeight.Kind == DimensionAuto && cs.OverflowY == OverflowVisible {
				autoMin := t.computeMinContentSize(it.node, false, mainSize, crossSize)
				if cs.Height.Kind == DimensionPx && autoMin > cs.Height.Value {
					autoMin = cs.Height.Value
				}
				if cs.MaxHeight.Kind != DimensionAuto {
					maxH := t.resolveDimension(cs.MaxHeight, mainSize)
					if maxH >= 0 {
						maxH += boxPadMain
						if autoMin > maxH {
							autoMin = maxH
						}
					}
				}
				if autoMin > 0 && hypo < autoMin {
					hypo = autoMin
				}
			}
		}
		if hypo < 0 {
			hypo = 0
		}

		it.flexBasis = basis
		it.marginMain = marginMainStart + marginMainEnd
		it.hypotheticalMain = hypo + marginMainStart + marginMainEnd
		it.targetMain = hypo
	}

	// --- 4–5. Determine lines (single-line vs multi-line) ---
	var linesBuf [8]flexLine
	lines := linesBuf[:0]
	wrap := node.Style.FlexWrap

	isIndefiniteColWrap := !isRow && wrap != FlexWrapNoWrap && node.Style.Height.Kind == DimensionAuto && node.Style.MaxHeight.Kind == DimensionAuto && parentHeight <= 0

	if wrap == FlexWrapNoWrap || isIndefiniteColWrap {
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
		// Multi-line wrapping (W3C §9.3).
		wrapLimit := mainSize
		if wrapLimit <= 0 {
			var maxD Dimension
			if isRow {
				maxD = node.Style.MaxWidth
			} else {
				maxD = node.Style.MaxHeight
			}
			if maxD.Kind != DimensionAuto {
				maxV := t.resolveDimension(maxD, parentWidth)
				if !isRow {
					maxV = t.resolveDimension(maxD, parentHeight)
				}
				if maxV > 0 {
					if node.Style.BoxSizing == BoxSizingContentBox {
						wrapLimit = maxV
					} else {
						if isRow {
							wrapLimit = maxV - padLeft - padRight
						} else {
							wrapLimit = maxV - padTop - padBottom
						}
					}
				}
			}
		}

		lineStart := 0
		var lineMain float32
		for i := range items {
			itemMain := items[i].hypotheticalMain
			g := gap
			if i == lineStart {
				g = 0
			}
			if wrapLimit > 0 && i > lineStart && lineMain+g+itemMain > wrapLimit {
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

	// --- 6. Distribute free space per line (W3C §9.7) ---
	for li := range lines {
		line := &lines[li]
		lineItems := items[line.startIdx : line.startIdx+line.count]
		totalGaps := gap * float32(line.count-1)

		isUnconstrainedAuto := mainSize <= 0 && id == t.root && ((isRow && node.Style.Width.Kind == DimensionAuto && node.Style.MinWidth.Kind == DimensionAuto) || (!isRow && node.Style.Height.Kind == DimensionAuto && node.Style.MinHeight.Kind == DimensionAuto))
		if isUnconstrainedAuto {
			for i := range lineItems {
				cs := &t.nodes[lineItems[i].node].Style
				target := t.computeMinContentSize(lineItems[i].node, isRow, mainSize, crossSize)
				if isRow {
					var boxPad float32
					if cs.BoxSizing == BoxSizingContentBox {
						boxPad = t.resolveDimensionOrZero(cs.Padding.Left, crossSize) + t.resolveDimensionOrZero(cs.Padding.Right, crossSize)
					}
					if cs.Width.Kind == DimensionPx {
						target = cs.Width.Value + boxPad
					}
					target = t.clampNodeSize(target, cs.MinWidth, cs.MaxWidth, mainSize, boxPad)
				} else {
					var boxPad float32
					if cs.BoxSizing == BoxSizingContentBox {
						boxPad = t.resolveDimensionOrZero(cs.Padding.Top, crossSize) + t.resolveDimensionOrZero(cs.Padding.Bottom, crossSize)
					}
					if cs.FlexBasis.Kind == DimensionPx {
						target = cs.FlexBasis.Value + boxPad
					} else if cs.Height.Kind == DimensionPx {
						target = cs.Height.Value + boxPad
					}
					target = t.clampNodeSize(target, cs.MinHeight, cs.MaxHeight, mainSize, boxPad)
				}
				lineItems[i].targetMain = target
			}
		} else if mainSize <= 0 {
			for i := range lineItems {
				lineItems[i].targetMain = lineItems[i].hypotheticalMain - lineItems[i].marginMain
			}
		} else {
			for i := range lineItems {
				lineItems[i].targetMain = lineItems[i].hypotheticalMain - lineItems[i].marginMain
			}
			frozen := make([]bool, len(lineItems))
			for pass := 0; pass < len(lineItems); pass++ {
				var usedSpace float32
				var unfrozenGrow, unfrozenWeightedShrink float32
				unfrozenCount := 0
				for i := range lineItems {
					margin := lineItems[i].marginMain
					if frozen[i] {
						usedSpace += lineItems[i].targetMain + margin
					} else {
						unfrozenCount++
						usedSpace += lineItems[i].flexBasis + margin
						unfrozenGrow += lineItems[i].flexGrow
						unfrozenWeightedShrink += lineItems[i].flexShrink * lineItems[i].flexBasis
					}
				}
				if unfrozenCount == 0 {
					break
				}
				freeSpace := mainSize - totalGaps - usedSpace

				type flexViol uint8
				const (
					violNone flexViol = iota
					violMin
					violMax
				)
				itemViol := make([]flexViol, len(lineItems))
				var totalViolation float32
				anyViolation := false

				if freeSpace > 0 && unfrozenGrow > 0 {
					growDiv := unfrozenGrow
					if growDiv < 1.0 {
						growDiv = 1.0
					}
					for i := range lineItems {
						if frozen[i] {
							continue
						}
						var target float32
						if lineItems[i].flexGrow > 0 {
							target = lineItems[i].flexBasis + freeSpace*(lineItems[i].flexGrow/growDiv)
						} else {
							target = lineItems[i].hypotheticalMain - lineItems[i].marginMain
						}
						cs := &t.nodes[lineItems[i].node].Style
						var minMain float32
						var maxMain float32 = -1
						if isRow {
							minMain = t.resolveDimensionOrZero(cs.MinWidth, mainSize)
							if cs.MinWidth.Kind == DimensionAuto && cs.OverflowX == OverflowVisible {
								autoMin := t.computeMinContentSize(lineItems[i].node, true, mainSize, crossSize)
								if autoMin > minMain {
									minMain = autoMin
								}
							}
							if cs.MaxWidth.Kind != DimensionAuto {
								maxMain = t.resolveDimension(cs.MaxWidth, mainSize)
							}
						} else {
							minMain = t.resolveDimensionOrZero(cs.MinHeight, mainSize)
							if cs.MinHeight.Kind == DimensionAuto && cs.OverflowY == OverflowVisible {
								autoMin := t.computeMinContentSize(lineItems[i].node, false, mainSize, crossSize)
								if autoMin > minMain {
									minMain = autoMin
								}
							}
							if cs.MaxHeight.Kind != DimensionAuto {
								maxMain = t.resolveDimension(cs.MaxHeight, mainSize)
							}
						}
						clamped := target
						if clamped < minMain {
							clamped = minMain
							itemViol[i] = violMin
							totalViolation += (clamped - target)
							anyViolation = true
						} else if maxMain >= 0 && clamped > maxMain {
							clamped = maxMain
							itemViol[i] = violMax
							totalViolation += (clamped - target)
							anyViolation = true
						}
						lineItems[i].targetMain = clamped
					}
				} else if freeSpace < 0 && unfrozenWeightedShrink > 0 {
					var unfrozenShrinkSum float32
					for i := range lineItems {
						if !frozen[i] {
							unfrozenShrinkSum += lineItems[i].flexShrink
						}
					}
					overflow := -freeSpace
					if unfrozenShrinkSum < 1.0 {
						overflow *= unfrozenShrinkSum
					}
					for i := range lineItems {
						if frozen[i] {
							continue
						}
						ratio := (lineItems[i].flexShrink * lineItems[i].flexBasis) / unfrozenWeightedShrink
						target := lineItems[i].flexBasis - overflow*ratio
						cs := &t.nodes[lineItems[i].node].Style
						var minMain float32
						var maxMain float32 = -1
						if isRow {
							minMain = t.resolveDimensionOrZero(cs.MinWidth, mainSize)
							if cs.MinWidth.Kind == DimensionAuto && cs.OverflowX == OverflowVisible {
								autoMin := t.computeMinContentSize(lineItems[i].node, true, mainSize, crossSize)
								if autoMin > minMain {
									minMain = autoMin
								}
							}
							if cs.MaxWidth.Kind != DimensionAuto {
								maxMain = t.resolveDimension(cs.MaxWidth, mainSize)
							}
						} else {
							minMain = t.resolveDimensionOrZero(cs.MinHeight, mainSize)
							if cs.MinHeight.Kind == DimensionAuto && cs.OverflowY == OverflowVisible {
								autoMin := t.computeMinContentSize(lineItems[i].node, false, mainSize, crossSize)
								if autoMin > minMain {
									minMain = autoMin
								}
							}
							if cs.MaxHeight.Kind != DimensionAuto {
								maxMain = t.resolveDimension(cs.MaxHeight, mainSize)
							}
						}
						clamped := target
						if clamped < minMain {
							clamped = minMain
							itemViol[i] = violMin
							totalViolation += (clamped - target)
							anyViolation = true
						} else if maxMain >= 0 && clamped > maxMain {
							clamped = maxMain
							itemViol[i] = violMax
							totalViolation += (clamped - target)
							anyViolation = true
						}
						lineItems[i].targetMain = clamped
					}
				}
				if !anyViolation || totalViolation == 0 {
					break
				}
				for i := range lineItems {
					if frozen[i] {
						continue
					}
					if totalViolation > 0 && itemViol[i] == violMin {
						frozen[i] = true
					} else if totalViolation < 0 && itemViol[i] == violMax {
						frozen[i] = true
					}
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
			totalUsed += lineItems[i].hypotheticalMainWithTarget(isRow, &t.nodes[lineItems[i].node].Style, childMarginRef, parentHeight, t)
		}
		totalUsed += totalGaps
		line.mainUsed = totalUsed

		remaining := mainSize - totalUsed
		if mainSize <= 0 {
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

		effectiveJC := node.Style.JustifyContent
		if effectiveJC == JustifyStart {
			if isReverse {
				effectiveJC = JustifyFlexEnd
			} else {
				effectiveJC = JustifyFlexStart
			}
		} else if effectiveJC == JustifyEnd {
			if isReverse {
				effectiveJC = JustifyFlexStart
			} else {
				effectiveJC = JustifyFlexEnd
			}
		}

		if remaining < 0 {
			switch effectiveJC {
			case JustifyFlexEnd:
				offset = remaining
			case JustifyCenter:
				offset = remaining / 2
			default:
				offset = 0
			}
		} else {
			switch effectiveJC {
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
					marginMainStart = t.resolveDimensionOrZero(cs.Margin.Left, childMarginRef)
				}
				if isAutoMargin(cs.Margin.Right) {
					marginMainEnd = autoMarginShare
				} else {
					marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Right, childMarginRef)
				}
			} else {
				if isAutoMargin(cs.Margin.Top) {
					marginMainStart = autoMarginShare
				} else {
					marginMainStart = t.resolveDimensionOrZero(cs.Margin.Top, childMarginRef)
				}
				if isAutoMargin(cs.Margin.Bottom) {
					marginMainEnd = autoMarginShare
				} else {
					marginMainEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, childMarginRef)
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
			var boxPadCross float32
			if cs.BoxSizing == BoxSizingContentBox {
				cbW := crossSize
				if isRow {
					cbW = mainSize
				}
				if isRow {
					boxPadCross = t.resolveDimensionOrZero(cs.Padding.Top, cbW) + t.resolveDimensionOrZero(cs.Padding.Bottom, cbW)
				} else {
					boxPadCross = t.resolveDimensionOrZero(cs.Padding.Left, cbW) + t.resolveDimensionOrZero(cs.Padding.Right, cbW)
				}
			}

			var cross float32
			if isRow {
				isContainerCrossAuto := (id == t.root && node.Style.Height.Kind == DimensionAuto) || crossSize <= 0
				if isContainerCrossAuto && cs.Height.Kind == DimensionPercent {
					cross = 0
				} else {
					cross = t.resolveDimension(cs.Height, crossSize)
					if cross >= 0 && cs.BoxSizing == BoxSizingContentBox {
						cross += boxPadCross
					}
				}
				if cs.Height.Kind == DimensionAuto || (isContainerCrossAuto && cs.Height.Kind == DimensionPercent) {
					if cs.AspectRatio > 0 {
						cross = lineItems[i].targetMain / cs.AspectRatio
					} else if int(lineItems[i].node) < len(t.measureFuncs) && t.measureFuncs[lineItems[i].node] != nil {
						fn := t.measureFuncs[lineItems[i].node]
						_, mh := fn(t, lineItems[i].node, lineItems[i].targetMain, MeasureExact, crossSize, MeasureAtMost)
						cross = mh
					} else {
						cross = t.computeIntrinsicCross(lineItems[i].node, isRow, lineItems[i].targetMain)
					}
					if cross >= 0 && cs.BoxSizing == BoxSizingContentBox {
						cross += boxPadCross
					}
				}
				cross = t.clampNodeSize(cross, cs.MinHeight, cs.MaxHeight, crossSize, boxPadCross)
			} else {
				isContainerCrossAuto := (id == t.root && node.Style.Width.Kind == DimensionAuto) || crossSize <= 0
				if isContainerCrossAuto && cs.Width.Kind == DimensionPercent {
					cross = 0
				} else {
					cross = t.resolveDimension(cs.Width, crossSize)
					if cross >= 0 && cs.BoxSizing == BoxSizingContentBox {
						cross += boxPadCross
					}
				}
				if cs.Width.Kind == DimensionAuto || (isContainerCrossAuto && cs.Width.Kind == DimensionPercent) {
					if cs.AspectRatio > 0 {
						cross = lineItems[i].targetMain * cs.AspectRatio
					} else if int(lineItems[i].node) < len(t.measureFuncs) && t.measureFuncs[lineItems[i].node] != nil {
						fn := t.measureFuncs[lineItems[i].node]
						mw, _ := fn(t, lineItems[i].node, crossSize, MeasureAtMost, lineItems[i].targetMain, MeasureExact)
						cross = mw
					} else {
						cross = t.computeIntrinsicCross(lineItems[i].node, isRow, lineItems[i].targetMain)
					}
					if cross >= 0 && cs.BoxSizing == BoxSizingContentBox {
						cross += boxPadCross
					}
				}
				cross = t.clampNodeSize(cross, cs.MinWidth, cs.MaxWidth, crossSize, boxPadCross)
			}
			if cross < 0 {
				cross = 0
			}

			// Add cross-axis margins.
			var marginCrossStart, marginCrossEnd float32
			if isRow {
				marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Top, childMarginRef)
				marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, childMarginRef)
			} else {
				marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Left, childMarginRef)
				marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Right, childMarginRef)
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

	if len(lines) == 1 && (node.Style.FlexWrap == FlexWrapNoWrap || isIndefiniteColWrap) {
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
	if isAutoCross {
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

		if node.Style.FlexWrap == FlexWrapWrapReverse {
			containerCross := crossSize
			if containerCross <= 0 {
				containerCross = totalLineCross
			}
			for li := range lines {
				lines[li].lineOffset = containerCross - lines[li].lineOffset - lines[li].crossSize
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
					mStart = t.resolveDimensionOrZero(cs.Margin.Top, childMarginRef)
				} else {
					mStart = t.resolveDimensionOrZero(cs.Margin.Left, childMarginRef)
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
					marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Top, childMarginRef)
				}
				if isAutoMargin(cs.Margin.Bottom) {
					hasAutoCrossEnd = true
				} else {
					marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Bottom, childMarginRef)
				}
			} else {
				if isAutoMargin(cs.Margin.Left) {
					hasAutoCrossStart = true
				} else {
					marginCrossStart = t.resolveDimensionOrZero(cs.Margin.Left, childMarginRef)
				}
				if isAutoMargin(cs.Margin.Right) {
					hasAutoCrossEnd = true
				} else {
					marginCrossEnd = t.resolveDimensionOrZero(cs.Margin.Right, childMarginRef)
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
			} else if node.Style.FlexWrap == FlexWrapWrapReverse {
				switch align {
				case AlignStretch:
					if cs.AspectRatio == 0 && (lineItems[i].crossSize == 0 || (isRow && cs.Height.Kind == DimensionAuto && availCross > 0) || (!isRow && cs.Width.Kind == DimensionAuto && availCross > 0)) {
						var boxPadCross float32
						if cs.BoxSizing == BoxSizingContentBox {
							cbW := crossSize
							if isRow {
								cbW = mainSize
							}
							if isRow {
								boxPadCross = t.resolveDimensionOrZero(cs.Padding.Top, cbW) + t.resolveDimensionOrZero(cs.Padding.Bottom, cbW)
							} else {
								boxPadCross = t.resolveDimensionOrZero(cs.Padding.Left, cbW) + t.resolveDimensionOrZero(cs.Padding.Right, cbW)
							}
						}
						if isRow {
							lineItems[i].crossSize = t.clampNodeSize(availCross, cs.MinHeight, cs.MaxHeight, crossSize, boxPadCross)
						} else {
							lineItems[i].crossSize = t.clampNodeSize(availCross, cs.MinWidth, cs.MaxWidth, crossSize, boxPadCross)
						}
						lineItems[i].crossOffset = marginCrossStart
					} else {
						lineItems[i].crossOffset = line.crossSize - lineItems[i].crossSize - marginCrossStart
					}
				case AlignFlexStart:
					lineItems[i].crossOffset = line.crossSize - lineItems[i].crossSize - marginCrossStart
				case AlignFlexEnd:
					lineItems[i].crossOffset = marginCrossEnd
				case AlignCenter:
					lineItems[i].crossOffset = marginCrossStart + (availCross-lineItems[i].crossSize)/2
				case AlignBaseline:
					b := lineItems[i].crossSize
					if int(lineItems[i].node) < len(t.baselines) && t.baselines[lineItems[i].node] >= 0 {
						b = t.baselines[lineItems[i].node]
					}
					lineItems[i].crossOffset = line.crossSize - (marginCrossStart + (maxBaseline - b) + lineItems[i].crossSize)
				}
			} else {
				switch align {
				case AlignStretch:
					if cs.AspectRatio == 0 && (lineItems[i].crossSize == 0 || (isRow && cs.Height.Kind == DimensionAuto && availCross > 0) || (!isRow && cs.Width.Kind == DimensionAuto && availCross > 0)) {
						var boxPadCross float32
						if cs.BoxSizing == BoxSizingContentBox {
							cbW := crossSize
							if isRow {
								cbW = mainSize
							}
							if isRow {
								boxPadCross = t.resolveDimensionOrZero(cs.Padding.Top, cbW) + t.resolveDimensionOrZero(cs.Padding.Bottom, cbW)
							} else {
								boxPadCross = t.resolveDimensionOrZero(cs.Padding.Left, cbW) + t.resolveDimensionOrZero(cs.Padding.Right, cbW)
							}
						}
						if isRow {
							lineItems[i].crossSize = t.clampNodeSize(availCross, cs.MinHeight, cs.MaxHeight, crossSize, boxPadCross)
						} else {
							lineItems[i].crossSize = t.clampNodeSize(availCross, cs.MinWidth, cs.MaxWidth, crossSize, boxPadCross)
						}
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
				top := t.resolveDimensionOrZero(cn.Style.Inset.Top, contentHeight)
				bottom := t.resolveDimensionOrZero(cn.Style.Inset.Bottom, contentHeight)
				left := t.resolveDimensionOrZero(cn.Style.Inset.Left, contentWidth)
				right := t.resolveDimensionOrZero(cn.Style.Inset.Right, contentWidth)
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

			// --- 14. Recursive layout for container children ---
			if cn.FirstChild != InvalidNode {
				padRef := contentWidth
				if padRef <= 0 && parentWidth > 0 {
					padRef = parentWidth
				}
				if padRef <= 0 && isRow && node.Computed.Width > 0 {
					padRef = node.Computed.Width
				}
				padL := t.resolveDimensionOrZero(cn.Style.Padding.Left, padRef)
				padT := t.resolveDimensionOrZero(cn.Style.Padding.Top, padRef)
				padR := t.resolveDimensionOrZero(cn.Style.Padding.Right, padRef)
				padB := t.resolveDimensionOrZero(cn.Style.Padding.Bottom, padRef)
				cw := w - padL - padR
				ch := h - padT - padB
				if cw < 0 {
					cw = 0
				}
				if ch < 0 {
					ch = 0
				}
				recParentW := contentWidth
				if recParentW <= 0 && parentWidth > 0 {
					recParentW = parentWidth
				}
				recParentH := contentHeight
				if node.Style.Height.Kind == DimensionAuto && (id == t.root || parentHeight <= 0) {
					recParentH = 0
				} else if recParentH <= 0 && parentHeight > 0 {
					recParentH = parentHeight
				}
				if cn.Style.Display == DisplayFlex {
					t.layoutFlex(it.node, cw, ch, padL, padT, recParentW, recParentH)
				} else if cn.Style.Display == DisplayGrid {
					t.layoutGrid(it.node, cw, ch, padL, padT, recParentW, recParentH)
				} else {
					t.layoutBlock(it.node, cw, ch, padL, padT)
				}
				cn.Computed.X = x
				cn.Computed.Y = y
				hasExplicitBasis := cn.Style.FlexBasis.Kind != DimensionAuto && cn.Style.FlexBasis.Kind != DimensionContent && cn.Style.FlexBasis.Kind != DimensionMaxContent
				isStretch := cn.Style.AlignSelf == AlignSelfStretch || (cn.Style.AlignSelf == AlignSelfAuto && node.Style.AlignItems == AlignStretch)
				if isRow {
					if cn.Style.Width.Kind != DimensionAuto || hasExplicitBasis || it.flexGrow > 0 {
						cn.Computed.Width = w
					}
					if cn.Style.Height.Kind != DimensionAuto || isStretch {
						cn.Computed.Height = h
					}
				} else {
					if cn.Style.Height.Kind != DimensionAuto || hasExplicitBasis || it.flexGrow > 0 {
						cn.Computed.Height = h
					}
					if cn.Style.Width.Kind != DimensionAuto || isStretch {
						cn.Computed.Width = w
					}
				}
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

	padRight = t.resolveDimensionOrZero(node.Style.Padding.Right, parentWidth)
	padBottom = t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentWidth)

	if node.Style.Width.Kind == DimensionAuto {
		if node.Style.BoxSizing == BoxSizingContentBox {
			contentW := t.clampNodeSize(actualContentWidth, node.Style.MinWidth, node.Style.MaxWidth, parentWidth, 0)
			node.Computed.Width = contentW + padLeft + padRight
		} else {
			node.Computed.Width = t.clampSize(actualContentWidth+padLeft+padRight, node.Style.MinWidth, node.Style.MaxWidth, parentWidth)
		}
	}
	if node.Style.Height.Kind == DimensionAuto {
		if node.Style.BoxSizing == BoxSizingContentBox {
			contentH := t.clampNodeSize(actualContentHeight, node.Style.MinHeight, node.Style.MaxHeight, parentHeight, 0)
			node.Computed.Height = contentH + padTop + padBottom
		} else {
			node.Computed.Height = t.clampSize(actualContentHeight+padTop+padBottom, node.Style.MinHeight, node.Style.MaxHeight, parentHeight)
		}
	}

	// Re-run layout if auto container was clamped by Min/Max in main axis or cross axis
	clampedContentW := node.Computed.Width - padLeft - padRight
	clampedContentH := node.Computed.Height - padTop - padBottom
	reRunW := contentWidth
	reRunH := contentHeight
	needReRun := false

	if isRow {
		if node.Style.Width.Kind == DimensionAuto && clampedContentW > 0 && clampedContentW != actualContentWidth && clampedContentW != contentWidth {
			reRunW = clampedContentW
			needReRun = true
		}
		if node.Style.Height.Kind == DimensionAuto && clampedContentH > 0 && clampedContentH != actualContentHeight && clampedContentH != contentHeight {
			reRunH = clampedContentH
			needReRun = true
		}
	} else {
		if node.Style.Height.Kind == DimensionAuto && clampedContentH > 0 && clampedContentH != actualContentHeight && clampedContentH != contentHeight {
			reRunH = clampedContentH
			needReRun = true
		}
		if node.Style.Width.Kind == DimensionAuto && clampedContentW > 0 && clampedContentW != actualContentWidth && clampedContentW != contentWidth {
			reRunW = clampedContentW
			needReRun = true
		}
	}

	if needReRun {
		savedW := node.Computed.Width
		savedH := node.Computed.Height
		t.layoutFlex(id, reRunW, reRunH, padLeft, padTop, parentWidth, parentHeight)
		node.Computed.Width = savedW
		node.Computed.Height = savedH
		return
	}

	// Cyclic percentage gap in auto containers
	hasPercentGap := (isRow && (node.Style.ColumnGap.Kind == DimensionPercent || node.Style.Gap.Kind == DimensionPercent)) ||
		(!isRow && (node.Style.RowGap.Kind == DimensionPercent || node.Style.Gap.Kind == DimensionPercent))
	if hasPercentGap && ((isRow && contentWidth <= 0 && actualContentWidth > 0) || (!isRow && contentHeight <= 0 && actualContentHeight > 0)) {
		savedW := node.Computed.Width
		savedH := node.Computed.Height
		newCw := actualContentWidth
		newCh := contentHeight
		if !isRow {
			newCw = contentWidth
			newCh = actualContentHeight
		}
		t.layoutFlex(id, newCw, newCh, padLeft, padTop, parentWidth, parentHeight)
		node.Computed.Width = savedW
		node.Computed.Height = savedH
		return
	}

	// Resolve deferred percentage dimensions in auto containers
	if node.Style.Height.Kind == DimensionAuto && contentHeight <= 0 && actualContentHeight > 0 {
		for i := range items {
			cnode := &t.nodes[items[i].node]
			if cnode.Style.Height.Kind == DimensionPercent {
				hVal := actualContentHeight * cnode.Style.Height.Value / 100
				if cnode.Style.BoxSizing == BoxSizingContentBox {
					hVal += t.resolveDimensionOrZero(cnode.Style.Padding.Top, actualContentWidth) + t.resolveDimensionOrZero(cnode.Style.Padding.Bottom, actualContentWidth)
				}
				cnode.Computed.Height = hVal
			}
		}
	}
	if node.Style.Width.Kind == DimensionAuto && contentWidth <= 0 && actualContentWidth > 0 {
		for i := range items {
			cnode := &t.nodes[items[i].node]
			if cnode.Style.Width.Kind == DimensionPercent {
				wVal := actualContentWidth * cnode.Style.Width.Value / 100
				if cnode.Style.BoxSizing == BoxSizingContentBox {
					wVal += t.resolveDimensionOrZero(cnode.Style.Padding.Left, actualContentWidth) + t.resolveDimensionOrZero(cnode.Style.Padding.Right, actualContentWidth)
				}
				cnode.Computed.Width = wVal
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
	var minVal float32
	if isRow {
		if cs.MinWidth.Kind == DimensionPx || cs.MinWidth.Kind == DimensionPercent {
			minVal = t.resolveDimensionOrZero(cs.MinWidth, mainSize)
		}
		pad := t.resolveDimensionOrZero(cs.Padding.Left, mainSize) + t.resolveDimensionOrZero(cs.Padding.Right, mainSize)
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			mw, _ := fn(t, node, 0, MeasureAtMost, crossSize, MeasureAtMost)
			res := mw + pad
			if minVal > res {
				res = minVal
			}
			return res
		}
		cNode := &t.nodes[node]
		if cNode.FirstChild != InvalidNode {
			if cNode.Style.Display == DisplayGrid {
				colCount := len(cNode.Style.GridTemplateColumns)
				if colCount == 0 {
					colCount = 1
				}
				colWidths := make([]float32, colCount)
				for c := 0; c < colCount; c++ {
					if c < len(cNode.Style.GridTemplateColumns) && cNode.Style.GridTemplateColumns[c].Kind == DimensionPx {
						colWidths[c] = cNode.Style.GridTemplateColumns[c].Value
					}
				}
				chIdx := 0
				hasSub1Span := false
				var maxFrUnit float32
				for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
					chS := &t.nodes[ch].Style
					var cw float32
					if chS.Width.Kind == DimensionPx {
						cw = chS.Width.Value
					} else {
						cw = t.computeMinContentSize(ch, true, mainSize, crossSize)
					}
					mL := t.resolveDimensionOrZero(chS.Margin.Left, mainSize)
					mR := t.resolveDimensionOrZero(chS.Margin.Right, mainSize)
					itemW := cw + mL + mR

					cSpan := int(chS.GridColumnSpan)
					if cSpan < 1 {
						cSpan = 1
					}
					cStart := chIdx % colCount
					if chS.GridColumnStart > 0 {
						cStart = int(chS.GridColumnStart) - 1
					}
					if cStart+cSpan > colCount {
						cSpan = colCount - cStart
					}
					if cSpan == 1 {
						if itemW > colWidths[cStart] {
							colWidths[cStart] = itemW
						}
					} else {
						var spannedFr float32
						numFlex := 0
						for k := cStart; k < cStart+cSpan; k++ {
							if k < len(cNode.Style.GridTemplateColumns) && cNode.Style.GridTemplateColumns[k].Kind == DimensionFr {
								spannedFr += cNode.Style.GridTemplateColumns[k].Value
								numFlex++
							}
						}
						if numFlex > 0 && spannedFr > 0 {
							for k := cStart; k < cStart+cSpan; k++ {
								if k < len(cNode.Style.GridTemplateColumns) && cNode.Style.GridTemplateColumns[k].Kind == DimensionFr {
									share := itemW * (cNode.Style.GridTemplateColumns[k].Value / spannedFr)
									if share > colWidths[k] {
										colWidths[k] = share
									}
									if spannedFr < 1.0 {
										hasSub1Span = true
										subShare := share * spannedFr
										if subShare > maxFrUnit {
											maxFrUnit = subShare
										}
									}
								}
							}
						} else {
							share := itemW / float32(cSpan)
							for k := cStart; k < cStart+cSpan; k++ {
								if share > colWidths[k] {
									colWidths[k] = share
								}
							}
						}
					}
					chIdx += cSpan
				}
				if hasSub1Span && maxFrUnit > 0 {
					for k := 0; k < colCount; k++ {
						if k < len(cNode.Style.GridTemplateColumns) && cNode.Style.GridTemplateColumns[k].Kind == DimensionFr {
							if colWidths[k] == 0 {
								colWidths[k] = maxFrUnit
							}
						}
					}
				}
				var totalGridW float32
				for _, w := range colWidths {
					totalGridW += w
				}
				if colCount > 1 {
					totalGridW += float32(colCount-1) * t.resolveDimensionOrZero(cNode.Style.ColumnGap, mainSize)
				}
				res := totalGridW + pad
				if minVal > res {
					res = minVal
				}
				return res
			}

			isChildRow := cNode.Style.FlexDirection == FlexDirectionRow || cNode.Style.FlexDirection == FlexDirectionRowReverse
			isChildNowrap := cNode.Style.FlexWrap == FlexWrapNoWrap
			var totalOrMax float32
			for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
				chS := &t.nodes[ch].Style
				var cw float32
				if chS.Width.Kind == DimensionPx {
					cw = chS.Width.Value
				} else {
					cw = t.computeMinContentSize(ch, true, mainSize, crossSize)
				}
				mL := t.resolveDimensionOrZero(chS.Margin.Left, mainSize)
				mR := t.resolveDimensionOrZero(chS.Margin.Right, mainSize)
				itemW := cw + mL + mR
				if isChildRow && isChildNowrap {
					totalOrMax += itemW
				} else {
					if itemW > totalOrMax {
						totalOrMax = itemW
					}
				}
			}
			res := totalOrMax + pad
			if minVal > res {
				res = minVal
			}
			return res
		}
		res := pad
		if minVal > res {
			res = minVal
		}
		return res
	} else {
		inlineRef := crossSize
		if inlineRef <= 0 && cs.Width.Kind == DimensionPx {
			inlineRef = cs.Width.Value
		}
		if cs.MinHeight.Kind == DimensionPx || cs.MinHeight.Kind == DimensionPercent {
			minVal = t.resolveDimensionOrZero(cs.MinHeight, mainSize)
		}
		pad := t.resolveDimensionOrZero(cs.Padding.Top, inlineRef) + t.resolveDimensionOrZero(cs.Padding.Bottom, inlineRef)
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			_, mh := fn(t, node, crossSize, MeasureAtMost, 0, MeasureAtMost)
			res := mh + pad
			if minVal > res {
				res = minVal
			}
			return res
		}
		cNode := &t.nodes[node]
		if cNode.FirstChild != InvalidNode {
			chInlineRef := inlineRef
			if cNode.Style.Width.Kind == DimensionPx {
				chInlineRef = cNode.Style.Width.Value
			} else if cNode.Style.Width.Kind == DimensionPercent && inlineRef > 0 {
				chInlineRef = inlineRef * cNode.Style.Width.Value / 100
			} else if cNode.Style.Width.Kind == DimensionAuto && inlineRef > 0 {
				mL := t.resolveDimensionOrZero(cNode.Style.Margin.Left, inlineRef)
				mR := t.resolveDimensionOrZero(cNode.Style.Margin.Right, inlineRef)
				pL := t.resolveDimensionOrZero(cNode.Style.Padding.Left, inlineRef)
				pR := t.resolveDimensionOrZero(cNode.Style.Padding.Right, inlineRef)
				avail := inlineRef - mL - mR - pL - pR
				if avail > 0 {
					chInlineRef = avail
				}
			}

			if cNode.Style.Display == DisplayGrid {
				colCount := len(cNode.Style.GridTemplateColumns)
				if colCount == 0 {
					colCount = 1
				}
				var childHeights []float32
				for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
					chS := &t.nodes[ch].Style
					var chH float32
					if chS.Height.Kind == DimensionPx {
						chH = chS.Height.Value
					} else {
						chH = t.computeMinContentSize(ch, false, mainSize, chInlineRef)
					}
					mT := t.resolveDimensionOrZero(chS.Margin.Top, chInlineRef)
					mB := t.resolveDimensionOrZero(chS.Margin.Bottom, chInlineRef)
					childHeights = append(childHeights, chH+mT+mB)
				}
				var totalGridH float32
				rowCount := (len(childHeights) + colCount - 1) / colCount
				if len(cNode.Style.GridTemplateRows) > rowCount {
					rowCount = len(cNode.Style.GridTemplateRows)
				}
				for r := 0; r < rowCount; r++ {
					var maxInRow float32
					if r < len(cNode.Style.GridTemplateRows) && cNode.Style.GridTemplateRows[r].Kind == DimensionPx {
						maxInRow = cNode.Style.GridTemplateRows[r].Value
					}
					for c := 0; c < colCount; c++ {
						idx := r*colCount + c
						if idx < len(childHeights) && childHeights[idx] > maxInRow {
							maxInRow = childHeights[idx]
						}
					}
					totalGridH += maxInRow
				}
				if rowCount > 1 {
					totalGridH += float32(rowCount-1) * t.resolveDimensionOrZero(cNode.Style.RowGap, chInlineRef)
				}
				res := totalGridH + pad
				if minVal > res {
					res = minVal
				}
				return res
			}

			isChildCol := cNode.Style.FlexDirection == FlexDirectionColumn || cNode.Style.FlexDirection == FlexDirectionColumnReverse
			isChildNowrap := cNode.Style.FlexWrap == FlexWrapNoWrap || (isChildCol && cNode.Style.Height.Kind == DimensionAuto && cNode.Style.MaxHeight.Kind == DimensionAuto)
			var totalOrMax float32
			for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
				chS := &t.nodes[ch].Style
				var chH float32
				if chS.Height.Kind == DimensionPx {
					chH = chS.Height.Value
					if chS.BoxSizing == BoxSizingContentBox {
						chH += t.resolveDimensionOrZero(chS.Padding.Top, chInlineRef) + t.resolveDimensionOrZero(chS.Padding.Bottom, chInlineRef)
					}
				} else {
					chH = t.computeMinContentSize(ch, false, mainSize, chInlineRef)
				}
				mT := t.resolveDimensionOrZero(chS.Margin.Top, chInlineRef)
				mB := t.resolveDimensionOrZero(chS.Margin.Bottom, chInlineRef)
				itemH := chH + mT + mB
				if isChildCol && isChildNowrap {
					totalOrMax += itemH
				} else {
					if itemH > totalOrMax {
						totalOrMax = itemH
					}
				}
			}
			res := totalOrMax + pad
			if minVal > res {
				res = minVal
			}
			return res
		}
		res := pad
		if minVal > res {
			res = minVal
		}
		return res
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
func (t *Tree) resolveFlexBasis(node NodeID, s *Style, isRow bool, mainSize, crossSize float32) float32 {
	var boxPadMain float32
	if s.BoxSizing == BoxSizingContentBox {
		cbW := crossSize
		if isRow {
			cbW = mainSize
		}
		if isRow {
			boxPadMain = t.resolveDimensionOrZero(s.Padding.Left, cbW) + t.resolveDimensionOrZero(s.Padding.Right, cbW)
		} else {
			boxPadMain = t.resolveDimensionOrZero(s.Padding.Top, cbW) + t.resolveDimensionOrZero(s.Padding.Bottom, cbW)
		}
	}

	// 1. Explicit flex-basis takes precedence
	switch s.FlexBasis.Kind {
	case DimensionPx:
		return s.FlexBasis.Value + boxPadMain
	case DimensionPercent:
		if mainSize <= 0 {
			if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
				fn := t.measureFuncs[node]
				if isRow {
					mw, _ := fn(t, node, 0, MeasureUndefined, 0, MeasureUndefined)
					return mw + boxPadMain
				}
				itemW := crossSize
				wMode := MeasureExact
				if s.Width.Kind == DimensionPx {
					itemW = s.Width.Value
				} else if itemW <= 0 {
					wMode = MeasureUndefined
				}
				_, mh := fn(t, node, itemW, wMode, 0, MeasureUndefined)
				return mh + boxPadMain
			}
			return t.computeMaxContentSize(node, isRow, mainSize) + boxPadMain
		}
		return mainSize*s.FlexBasis.Value/100 + boxPadMain
	case DimensionContent, DimensionMaxContent:
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			if isRow {
				mw, _ := fn(t, node, 0, MeasureUndefined, 0, MeasureUndefined)
				return mw
			}
			itemW := crossSize
			wMode := MeasureExact
			if s.Width.Kind == DimensionPx {
				itemW = s.Width.Value
			} else if s.Width.Kind == DimensionPercent && crossSize >= 0 {
				itemW = crossSize * s.Width.Value / 100
			} else if itemW <= 0 {
				wMode = MeasureUndefined
			}
			_, mh := fn(t, node, itemW, wMode, 0, MeasureUndefined)
			return mh
		}
		return t.computeMaxContentSize(node, isRow, mainSize)
	case DimensionMinContent:
		if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
			fn := t.measureFuncs[node]
			if isRow {
				mw, _ := fn(t, node, 0, MeasureAtMost, 0, MeasureUndefined)
				return mw
			}
			itemW := crossSize
			wMode := MeasureAtMost
			if s.Width.Kind == DimensionPx {
				itemW = s.Width.Value
			} else if s.Width.Kind == DimensionPercent && crossSize >= 0 {
				itemW = crossSize * s.Width.Value / 100
			} else if itemW <= 0 {
				wMode = MeasureUndefined
			}
			_, mh := fn(t, node, itemW, wMode, 0, MeasureUndefined)
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
		return dim.Value + boxPadMain
	case DimensionPercent:
		return mainSize*dim.Value/100 + boxPadMain
	}

	// 3. AspectRatio resolution when main axis size is Auto but cross axis is definite
	if s.AspectRatio > 0 {
		if isRow && s.Width.Kind == DimensionAuto && s.Height.Kind == DimensionPx {
			return s.Height.Value*s.AspectRatio + boxPadMain
		} else if !isRow && s.Height.Kind == DimensionAuto && s.Width.Kind == DimensionPx {
			return s.Width.Value/s.AspectRatio + boxPadMain
		}
	}
	// 4. Intrinsic measure func if registered
	if int(node) < len(t.measureFuncs) && t.measureFuncs[node] != nil {
		fn := t.measureFuncs[node]
		if isRow {
			mw, _ := fn(t, node, mainSize, MeasureAtMost, 0, MeasureUndefined)
			return mw
		}
		itemW := crossSize
		wMode := MeasureExact
		if s.Width.Kind == DimensionPx {
			itemW = s.Width.Value
		} else if s.Width.Kind == DimensionPercent && crossSize >= 0 {
			itemW = crossSize * s.Width.Value / 100
		} else if itemW <= 0 {
			wMode = MeasureUndefined
		}
		_, mh := fn(t, node, itemW, wMode, 0, MeasureUndefined)
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
			if isChildRow {
				if chS.FlexBasis.Kind == DimensionPx {
					var boxPad float32
					if chS.BoxSizing == BoxSizingContentBox {
						boxPad = t.resolveDimensionOrZero(chS.Padding.Left, mainSize) + t.resolveDimensionOrZero(chS.Padding.Right, mainSize)
					}
					cw = chS.FlexBasis.Value + boxPad
				}
			} else {
				if chS.FlexBasis.Kind == DimensionPx {
					var boxPad float32
					if chS.BoxSizing == BoxSizingContentBox {
						boxPad = t.resolveDimensionOrZero(chS.Padding.Top, mainSize) + t.resolveDimensionOrZero(chS.Padding.Bottom, mainSize)
					}
					chH = chS.FlexBasis.Value + boxPad
				}
			}
			if chS.AspectRatio > 0 {
				if cw > 0 && chH == 0 {
					chH = cw / chS.AspectRatio
				} else if chH > 0 && cw == 0 {
					cw = chH * chS.AspectRatio
				}
			}
			inlineRef := crossSize
			if isRow {
				inlineRef = mainSize
			}
			if inlineRef <= 0 && cNode.Style.Width.Kind == DimensionPx {
				inlineRef = cNode.Style.Width.Value
			}
			if chS.BoxSizing == BoxSizingContentBox {
				padH := t.resolveDimensionOrZero(chS.Padding.Left, inlineRef) + t.resolveDimensionOrZero(chS.Padding.Right, inlineRef)
				padV := t.resolveDimensionOrZero(chS.Padding.Top, inlineRef) + t.resolveDimensionOrZero(chS.Padding.Bottom, inlineRef)
				if chS.Width.Kind == DimensionPx || chS.Width.Kind == DimensionPercent {
					cw += padH
				}
				if chS.Height.Kind == DimensionPx || chS.Height.Kind == DimensionPercent {
					chH += padV
				}
			}
			mLeft := t.resolveDimensionOrZero(chS.Margin.Left, inlineRef)
			mRight := t.resolveDimensionOrZero(chS.Margin.Right, inlineRef)
			mTop := t.resolveDimensionOrZero(chS.Margin.Top, inlineRef)
			mBottom := t.resolveDimensionOrZero(chS.Margin.Bottom, inlineRef)

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

	padTop := t.resolveDimensionOrZero(cNode.Style.Padding.Top, targetMain)
	padBottom := t.resolveDimensionOrZero(cNode.Style.Padding.Bottom, targetMain)
	padLeft := t.resolveDimensionOrZero(cNode.Style.Padding.Left, targetMain)
	padRight := t.resolveDimensionOrZero(cNode.Style.Padding.Right, targetMain)

	gap := t.resolveDimensionOrZero(cNode.Style.Gap, targetMain)

	if cNode.Style.FlexWrap != FlexWrapNoWrap && targetMain > 0 && isChildRow == isParentRow {
		var totalLinesCross float32
		var curLineMain float32
		var curLineCross float32
		lineCount := 0

		for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
			chS := &t.nodes[ch].Style
			cw := t.resolveDimensionOrZero(chS.Width, targetMain)
			chH := t.resolveDimensionOrZero(chS.Height, targetMain)
			if cw == 0 || chH == 0 {
				for gch := t.nodes[ch].FirstChild; gch != InvalidNode; gch = t.nodes[gch].NextSibling {
					gw := t.resolveDimensionOrZero(t.nodes[gch].Style.Width, targetMain)
					gh := t.resolveDimensionOrZero(t.nodes[gch].Style.Height, targetMain)
					if gw > cw {
						cw = gw
					}
					if gh > chH {
						chH = gh
					}
				}
			}
			mTop := t.resolveDimensionOrZero(chS.Margin.Top, targetMain)
			mBottom := t.resolveDimensionOrZero(chS.Margin.Bottom, targetMain)
			mLeft := t.resolveDimensionOrZero(chS.Margin.Left, targetMain)
			mRight := t.resolveDimensionOrZero(chS.Margin.Right, targetMain)

			var childMain, childCross float32
			if isChildRow {
				childMain = cw + mLeft + mRight
				childCross = chH + mTop + mBottom
			} else {
				childMain = chH + mTop + mBottom
				childCross = cw + mLeft + mRight
			}

			if curLineMain > 0 && curLineMain+childMain > targetMain {
				totalLinesCross += curLineCross
				lineCount++
				curLineMain = childMain
				curLineCross = childCross
			} else {
				curLineMain += childMain
				if childCross > curLineCross {
					curLineCross = childCross
				}
			}
		}
		if curLineCross > 0 {
			totalLinesCross += curLineCross
			lineCount++
		}
		if lineCount > 1 {
			totalLinesCross += float32(lineCount-1) * gap
		}
		if isParentRow {
			return totalLinesCross + padTop + padBottom
		}
		return totalLinesCross + padLeft + padRight
	}

	var maxVal, sumVal float32
	childCount := 0

	for ch := cNode.FirstChild; ch != InvalidNode; ch = t.nodes[ch].NextSibling {
		childCount++
		chS := &t.nodes[ch].Style
		cw := t.resolveDimensionOrZero(chS.Width, targetMain)
		chH := t.resolveDimensionOrZero(chS.Height, targetMain)
		if cw == 0 || chH == 0 {
			isGrandchildRow := t.nodes[ch].Style.FlexDirection == FlexDirectionRow || t.nodes[ch].Style.FlexDirection == FlexDirectionRowReverse
			isGrandchildNoWrap := t.nodes[ch].Style.FlexWrap == FlexWrapNoWrap
			var gSumW, gMaxW, gSumH, gMaxH float32
			for gch := t.nodes[ch].FirstChild; gch != InvalidNode; gch = t.nodes[gch].NextSibling {
				gw := t.resolveDimensionOrZero(t.nodes[gch].Style.Width, targetMain)
				gh := t.resolveDimensionOrZero(t.nodes[gch].Style.Height, targetMain)
				gSumW += gw
				if gw > gMaxW {
					gMaxW = gw
				}
				gSumH += gh
				if gh > gMaxH {
					gMaxH = gh
				}
			}
			if isGrandchildRow {
				if isGrandchildNoWrap && gSumW > 0 {
					cw = gSumW
				} else if gMaxW > 0 {
					cw = gMaxW
				}
				if gMaxH > 0 {
					chH = gMaxH
				}
			} else {
				if gMaxW > 0 {
					cw = gMaxW
				}
				if isGrandchildNoWrap && gSumH > 0 {
					chH = gSumH
				} else if gMaxH > 0 {
					chH = gMaxH
				}
			}
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

		var childCross float32
		if isParentRow {
			// Parent is row -> cross axis is Height
			childCross = chH + mTop + mBottom
		} else {
			// Parent is column -> cross axis is Width
			childCross = cw + mLeft + mRight
		}

		if childCross > maxVal {
			maxVal = childCross
		}
		sumVal += childCross
	}

	if childCount > 1 {
		sumVal += float32(childCount-1) * gap
	}

	if isParentRow {
		// Parent cross axis is Height
		if isChildRow {
			// Row container height is max child height
			return maxVal + padTop + padBottom
		}
		// Column container height is sum of child heights
		return sumVal + padTop + padBottom
	}
	// Parent cross axis is Width
	if isChildRow {
		// Row container width is sum of child widths
		return sumVal + padLeft + padRight
	}
	// Column container width is max child width
	return maxVal + padLeft + padRight
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

// clampNodeSize applies min/max constraints to a size value with box-sizing adjustment.
func (t *Tree) clampNodeSize(size float32, minD, maxD Dimension, parentRef, boxPad float32) float32 {
	if minD.Kind != DimensionAuto {
		minV := t.resolveDimension(minD, parentRef)
		if minV >= 0 {
			minV += boxPad
			if size < minV {
				size = minV
			}
		}
	}
	if maxD.Kind != DimensionAuto {
		maxV := t.resolveDimension(maxD, parentRef)
		if maxV >= 0 {
			maxV += boxPad
			if size > maxV {
				size = maxV
			}
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
		marginMainStart = t.resolveDimensionOrZero(s.Margin.Top, parentW)
		marginMainEnd = t.resolveDimensionOrZero(s.Margin.Bottom, parentW)
	}
	return fi.targetMain + marginMainStart + marginMainEnd
}
