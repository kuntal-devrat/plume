package layout

// layoutGrid implements the W3C CSS Grid Layout Module Level 1 algorithm.
func (t *Tree) layoutGrid(id NodeID, contentWidth, contentHeight, padLeft, padTop, parentWidth, parentHeight float32) {
	node := &t.nodes[id]

	// 1. Resolve Gaps
	colGap := t.resolveDimensionOrZero(node.Style.ColumnGap, contentWidth)
	if node.Style.ColumnGap.Kind == DimensionAuto {
		colGap = t.resolveDimensionOrZero(node.Style.Gap, contentWidth)
	}

	rowGap := t.resolveDimensionOrZero(node.Style.RowGap, contentHeight)
	if node.Style.RowGap.Kind == DimensionAuto {
		rowGap = t.resolveDimensionOrZero(node.Style.Gap, contentHeight)
	}

	// 2. Collect Grid Items
	numCols := len(node.Style.GridTemplateColumns)
	if numCols == 0 {
		numCols = 1
	}
	numRows := len(node.Style.GridTemplateRows)
	if numRows == 0 {
		numRows = 1
	}

	t.gridItems = t.gridItems[:0]
	child := node.FirstChild
	for child != InvalidNode {
		c := &t.nodes[child]
		if c.Style.Display != DisplayNone && c.Style.Position != PositionAbsolute {
			cs := &c.Style

			// Columns
			colStart := -1
			if cs.GridColumnStart > 0 {
				colStart = int(cs.GridColumnStart) - 1
			} else if cs.GridColumnStart < 0 {
				colStart = numCols + 1 + int(cs.GridColumnStart)
			}

			colEnd := -1
			if cs.GridColumnEnd > 0 {
				colEnd = int(cs.GridColumnEnd) - 1
			} else if cs.GridColumnEnd < 0 {
				colEnd = numCols + 1 + int(cs.GridColumnEnd)
			}

			colSpan := int(cs.GridColumnSpan)
			if colSpan <= 0 {
				if colEnd > colStart && colStart >= 0 {
					colSpan = colEnd - colStart
				} else {
					colSpan = 1
				}
			}

			// Rows
			rowStart := -1
			if cs.GridRowStart > 0 {
				rowStart = int(cs.GridRowStart) - 1
			} else if cs.GridRowStart < 0 {
				rowStart = numRows + 1 + int(cs.GridRowStart)
			}

			rowEnd := -1
			if cs.GridRowEnd > 0 {
				rowEnd = int(cs.GridRowEnd) - 1
			} else if cs.GridRowEnd < 0 {
				rowEnd = numRows + 1 + int(cs.GridRowEnd)
			}

			rowSpan := int(cs.GridRowSpan)
			if rowSpan <= 0 {
				if rowEnd > rowStart && rowStart >= 0 {
					rowSpan = rowEnd - rowStart
				} else {
					rowSpan = 1
				}
			}

			definite := colStart >= 0 && rowStart >= 0

			t.gridItems = append(t.gridItems, gridItemScratch{
				node:     child,
				colStart: colStart,
				colSpan:  colSpan,
				rowStart: rowStart,
				rowSpan:  rowSpan,
				definite: definite,
			})
		}
		child = c.NextSibling
	}

	if len(t.gridItems) == 0 {
		return
	}

	// 3. Determine Grid Dimensions (Columns & Rows count)
	explicitCols := len(node.Style.GridTemplateColumns)
	if explicitCols == 0 {
		explicitCols = 1
	}

	explicitRows := len(node.Style.GridTemplateRows)
	if explicitRows == 0 {
		explicitRows = 1
	}

	totalCols := explicitCols
	totalRows := explicitRows

	// Check if any placed item extends beyond explicit bounds
	for _, it := range t.gridItems {
		if it.colStart >= 0 && it.colStart+it.colSpan > totalCols {
			totalCols = it.colStart + it.colSpan
		}
		if it.rowStart >= 0 && it.rowStart+it.rowSpan > totalRows {
			totalRows = it.rowStart + it.rowSpan
		}
	}

	// 4. Auto-placement
	// Prepare occupancy map
	maxExpectedCells := (totalCols + len(t.gridItems)) * (totalRows + len(t.gridItems))
	if cap(t.occupied) < maxExpectedCells {
		t.occupied = make([]bool, maxExpectedCells)
	} else {
		t.occupied = t.occupied[:maxExpectedCells]
		for i := range t.occupied {
			t.occupied[i] = false
		}
	}

	isOccupied := func(r, c, stride int) bool {
		idx := r*stride + c
		if idx >= len(t.occupied) {
			return false
		}
		return t.occupied[idx]
	}

	setOccupied := func(r, c, rSpan, cSpan, stride int) {
		for i := 0; i < rSpan; i++ {
			for j := 0; j < cSpan; j++ {
				idx := (r+i)*stride + (c + j)
				if idx < len(t.occupied) {
					t.occupied[idx] = true
				}
			}
		}
	}

	// Stride for the 2D occupancy grid
	stride := totalCols + len(t.gridItems) + 8

	// Place definite items first
	for i := range t.gridItems {
		it := &t.gridItems[i]
		if it.definite {
			setOccupied(it.rowStart, it.colStart, it.rowSpan, it.colSpan, stride)
		}
	}

	// Place remaining items
	cursorRow := 0
	cursorCol := 0
	isColFlow := node.Style.GridAutoFlow == GridAutoFlowColumn || node.Style.GridAutoFlow == GridAutoFlowColumnDense

	for i := range t.gridItems {
		it := &t.gridItems[i]
		if it.definite {
			continue
		}

		if !isColFlow {
			// Row-major auto-placement
			r := cursorRow
			c := cursorCol

			if it.colStart >= 0 {
				c = it.colStart
			}

			for {
				// Check if item fits in row bounds
				if it.colStart < 0 && c+it.colSpan > totalCols {
					r++
					c = 0
					continue
				}

				// Check overlap
				fits := true
				for dr := 0; dr < it.rowSpan; dr++ {
					for dc := 0; dc < it.colSpan; dc++ {
						if isOccupied(r+dr, c+dc, stride) {
							fits = false
							break
						}
					}
					if !fits {
						break
					}
				}

				if fits {
					it.rowStart = r
					it.colStart = c
					setOccupied(r, c, it.rowSpan, it.colSpan, stride)
					if it.colStart+it.colSpan > totalCols {
						totalCols = it.colStart + it.colSpan
					}
					if it.rowStart+it.rowSpan > totalRows {
						totalRows = it.rowStart + it.rowSpan
					}

					if node.Style.GridAutoFlow != GridAutoFlowRowDense {
						cursorRow = r
						cursorCol = c + it.colSpan
					}
					break
				}

				c++
			}
		} else {
			// Column-major auto-placement
			r := cursorRow
			c := cursorCol

			if it.rowStart >= 0 {
				r = it.rowStart
			}

			for {
				if it.rowStart < 0 && r+it.rowSpan > totalRows {
					c++
					r = 0
					continue
				}

				fits := true
				for dr := 0; dr < it.rowSpan; dr++ {
					for dc := 0; dc < it.colSpan; dc++ {
						if isOccupied(r+dr, c+dc, stride) {
							fits = false
							break
						}
					}
					if !fits {
						break
					}
				}

				if fits {
					it.rowStart = r
					it.colStart = c
					setOccupied(r, c, it.rowSpan, it.colSpan, stride)
					if it.colStart+it.colSpan > totalCols {
						totalCols = it.colStart + it.colSpan
					}
					if it.rowStart+it.rowSpan > totalRows {
						totalRows = it.rowStart + it.rowSpan
					}

					if node.Style.GridAutoFlow != GridAutoFlowColumnDense {
						cursorCol = c
						cursorRow = r + it.rowSpan
					}
					break
				}

				r++
			}
		}
	}

	// 5. Track Sizing Algorithm (Columns & Rows)
	t.colSizes = t.resolveTracks(node.Style.GridTemplateColumns, totalCols, contentWidth, colGap, true, t.colSizes)
	t.rowSizes = t.resolveTracks(node.Style.GridTemplateRows, totalRows, contentHeight, rowGap, false, t.rowSizes)

	// Update container height if Auto
	if node.Style.Height.Kind == DimensionAuto && contentHeight <= 0 {
		var sumH float32
		for _, h := range t.rowSizes {
			sumH += h
		}
		if totalRows > 1 {
			sumH += float32(totalRows-1) * rowGap
		}
		node.Computed.Height = sumH + padTop + t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentHeight)
	}

	// 5b. Align-Content & Justify-Content for the Grid Tracks (CSS Grid §10.5)
	var totalColWidth, totalRowHeight float32
	for _, w := range t.colSizes {
		totalColWidth += w
	}
	if totalCols > 1 {
		totalColWidth += float32(totalCols-1) * colGap
	}
	for _, h := range t.rowSizes {
		totalRowHeight += h
	}
	if totalRows > 1 {
		totalRowHeight += float32(totalRows-1) * rowGap
	}

	var gridAlignOffsetY, gridJustifyOffsetX float32
	if contentHeight > totalRowHeight {
		freeY := contentHeight - totalRowHeight
		switch node.Style.AlignContent {
		case AlignContentCenter:
			gridAlignOffsetY = freeY * 0.5
		case AlignContentFlexEnd:
			gridAlignOffsetY = freeY
		case AlignContentSpaceBetween:
			if totalRows > 1 {
				rowGap += freeY / float32(totalRows-1)
			}
		case AlignContentSpaceAround:
			if totalRows > 0 {
				space := freeY / float32(totalRows)
				gridAlignOffsetY = space * 0.5
				rowGap += space
			}
		}
	}
	if contentWidth > totalColWidth {
		freeX := contentWidth - totalColWidth
		switch node.Style.JustifyContent {
		case JustifyCenter:
			gridJustifyOffsetX = freeX * 0.5
		case JustifyFlexEnd:
			gridJustifyOffsetX = freeX
		case JustifySpaceBetween:
			if totalCols > 1 {
				colGap += freeX / float32(totalCols-1)
			}
		case JustifySpaceAround:
			if totalCols > 0 {
				space := freeX / float32(totalCols)
				gridJustifyOffsetX = space * 0.5
				colGap += space
			}
		}
	}

	// 6. Item Positioning & Sizing
	for _, it := range t.gridItems {
		c := &t.nodes[it.node]

		// Cell left X & width
		var cellX float32 = padLeft + gridJustifyOffsetX
		for i := 0; i < it.colStart; i++ {
			if i < len(t.colSizes) {
				cellX += t.colSizes[i] + colGap
			}
		}

		var cellW float32
		for i := it.colStart; i < it.colStart+it.colSpan; i++ {
			if i < len(t.colSizes) {
				cellW += t.colSizes[i]
			}
		}
		if it.colSpan > 1 {
			cellW += float32(it.colSpan-1) * colGap
		}

		// Cell top Y & height
		var cellY float32 = padTop + gridAlignOffsetY
		for i := 0; i < it.rowStart; i++ {
			if i < len(t.rowSizes) {
				cellY += t.rowSizes[i] + rowGap
			}
		}

		var cellH float32
		for i := it.rowStart; i < it.rowStart+it.rowSpan; i++ {
			if i < len(t.rowSizes) {
				cellH += t.rowSizes[i]
			}
		}
		if it.rowSpan > 1 {
			cellH += float32(it.rowSpan-1) * rowGap
		}

		// Alignment within grid cell
		justify := node.Style.AlignItems
		if node.Style.JustifyItems != 0 {
			justify = node.Style.JustifyItems
		}
		if c.Style.JustifySelf != AlignSelfAuto {
			justify = AlignItems(c.Style.JustifySelf)
		}

		align := node.Style.AlignItems
		if c.Style.AlignSelf != AlignSelfAuto {
			align = AlignItems(c.Style.AlignSelf)
		}

		// Resolve margins
		var mLeft, mRight, mTop, mBottom float32
		isAutoMLeft := isAutoMargin(c.Style.Margin.Left)
		isAutoMRight := isAutoMargin(c.Style.Margin.Right)
		isAutoMTop := isAutoMargin(c.Style.Margin.Top)
		isAutoMBottom := isAutoMargin(c.Style.Margin.Bottom)

		if !isAutoMLeft {
			mLeft = t.resolveDimensionOrZero(c.Style.Margin.Left, cellW)
		}
		if !isAutoMRight {
			mRight = t.resolveDimensionOrZero(c.Style.Margin.Right, cellW)
		}
		if !isAutoMTop {
			mTop = t.resolveDimensionOrZero(c.Style.Margin.Top, cellH)
		}
		if !isAutoMBottom {
			mBottom = t.resolveDimensionOrZero(c.Style.Margin.Bottom, cellH)
		}

		availW := cellW - mLeft - mRight
		if availW < 0 {
			availW = 0
		}
		availH := cellH - mTop - mBottom
		if availH < 0 {
			availH = 0
		}

		// Item resolved size
		itemW := availW
		if c.Style.Width.Kind != DimensionAuto {
			itemW = t.resolveDimensionOrZero(c.Style.Width, availW)
		}
		itemW = t.clampSize(itemW, c.Style.MinWidth, c.Style.MaxWidth, availW)

		itemH := availH
		if c.Style.Height.Kind != DimensionAuto {
			itemH = t.resolveDimensionOrZero(c.Style.Height, availH)
		}
		itemH = t.clampSize(itemH, c.Style.MinHeight, c.Style.MaxHeight, availH)

		// AspectRatio handling
		if c.Style.AspectRatio > 0 {
			if c.Style.Width.Kind != DimensionAuto && c.Style.Height.Kind == DimensionAuto {
				itemH = itemW / c.Style.AspectRatio
				itemH = t.clampSize(itemH, c.Style.MinHeight, c.Style.MaxHeight, availH)
			} else if c.Style.Height.Kind != DimensionAuto && c.Style.Width.Kind == DimensionAuto {
				itemW = itemH * c.Style.AspectRatio
				itemW = t.clampSize(itemW, c.Style.MinWidth, c.Style.MaxWidth, availW)
			} else if c.Style.Width.Kind == DimensionAuto && c.Style.Height.Kind == DimensionAuto {
				if availW > 0 && availH > 0 {
					if availW/availH > c.Style.AspectRatio {
						itemH = availH
						itemW = itemH * c.Style.AspectRatio
					} else {
						itemW = availW
						itemH = itemW / c.Style.AspectRatio
					}
				}
			}
		}

		// Stretch sizing adjustment when width/height is Auto and not auto margin
		if justify == AlignStretch && c.Style.Width.Kind == DimensionAuto && !isAutoMLeft && !isAutoMRight && c.Style.AspectRatio == 0 {
			itemW = availW
		}
		if align == AlignStretch && c.Style.Height.Kind == DimensionAuto && !isAutoMTop && !isAutoMBottom && c.Style.AspectRatio == 0 {
			itemH = availH
		}

		// Item offsets (Horizontal)
		itemX := cellX
		if isAutoMLeft && isAutoMRight {
			itemX = cellX + (cellW-itemW)*0.5
		} else if isAutoMLeft {
			itemX = cellX + cellW - itemW - mRight
		} else if isAutoMRight {
			itemX = cellX + mLeft
		} else {
			switch justify {
			case AlignStretch, AlignFlexStart:
				itemX = cellX + mLeft
			case AlignFlexEnd:
				itemX = cellX + cellW - itemW - mRight
			case AlignCenter:
				itemX = cellX + mLeft + (availW-itemW)*0.5
			}
		}

		// Item offsets (Vertical)
		itemY := cellY
		if isAutoMTop && isAutoMBottom {
			itemY = cellY + (cellH-itemH)*0.5
		} else if isAutoMTop {
			itemY = cellY + cellH - itemH - mBottom
		} else if isAutoMBottom {
			itemY = cellY + mTop
		} else {
			switch align {
			case AlignStretch, AlignFlexStart:
				itemY = cellY + mTop
			case AlignFlexEnd:
				itemY = cellY + cellH - itemH - mBottom
			case AlignCenter:
				itemY = cellY + mTop + (availH-itemH)*0.5
			}
		}

		// Relative positioning offsets
		if c.Style.Position == PositionRelative {
			top := t.resolveDimensionOrZero(c.Style.Inset.Top, parentHeight)
			bottom := t.resolveDimensionOrZero(c.Style.Inset.Bottom, parentHeight)
			left := t.resolveDimensionOrZero(c.Style.Inset.Left, parentWidth)
			right := t.resolveDimensionOrZero(c.Style.Inset.Right, parentWidth)
			if c.Style.Inset.Left.Kind != DimensionAuto {
				itemX += left
			} else if c.Style.Inset.Right.Kind != DimensionAuto {
				itemX -= right
			}
			if c.Style.Inset.Top.Kind != DimensionAuto {
				itemY += top
			} else if c.Style.Inset.Bottom.Kind != DimensionAuto {
				itemY -= bottom
			}
		}

		c.Computed.X = itemX
		c.Computed.Y = itemY
		c.Computed.Width = itemW
		c.Computed.Height = itemH

		// Recurse for nested containers
		if (c.Style.Display == DisplayFlex || c.Style.Display == DisplayGrid) && c.FirstChild != InvalidNode {
			t.computeNode(it.node, itemW, itemH)
			c.Computed.X = itemX
			c.Computed.Y = itemY
			c.Computed.Width = itemW
			c.Computed.Height = itemH
		}
	}

	// Position Absolute Children
	t.layoutAbsoluteChildren(id, contentWidth, contentHeight, padLeft, padTop)
}

// resolveTracks computes the pixel sizes of grid tracks.
func (t *Tree) resolveTracks(templates []Dimension, totalTracks int, containerSize, gap float32, isCol bool, dst []float32) []float32 {
	if cap(dst) < totalTracks {
		dst = make([]float32, totalTracks)
	} else {
		dst = dst[:totalTracks]
	}

	for i := range dst {
		dst[i] = 0
	}

	totalGaps := float32(0)
	if totalTracks > 1 {
		totalGaps = float32(totalTracks-1) * gap
	}

	availSpace := containerSize - totalGaps
	if availSpace < 0 {
		availSpace = 0
	}

	remainingSpace := availSpace
	totalFr := float32(0)

	// Pass 1: Resolve fixed, percentage, and auto-content tracks
	for i := 0; i < totalTracks; i++ {
		var dim Dimension
		if i < len(templates) {
			dim = templates[i]
		} else {
			// Implicit tracks default to 1fr
			dim = Fr(1)
		}

		switch dim.Kind {
		case DimensionPx:
			dst[i] = dim.Value
			remainingSpace -= dim.Value
		case DimensionPercent:
			val := containerSize * dim.Value / 100.0
			dst[i] = val
			remainingSpace -= val
		case DimensionFr:
			totalFr += dim.Value
		case DimensionAuto, DimensionMinContent, DimensionMaxContent:
			// Measure intrinsic content contribution of items in this track
			var maxItemContent float32
			for _, it := range t.gridItems {
				span := it.colSpan
				start := it.colStart
				if !isCol {
					span = it.rowSpan
					start = it.rowStart
				}
				if span == 1 && start == i {
					c := &t.nodes[it.node]
					var sz float32
					if isCol {
						if c.Style.Width.Kind == DimensionPx {
							sz = c.Style.Width.Value
						} else if c.Style.Width.Kind == DimensionPercent {
							sz = containerSize * c.Style.Width.Value / 100.0
						} else {
							sz = t.computeMinContentSize(it.node, true, containerSize, 0)
						}
					} else {
						if c.Style.Height.Kind == DimensionPx {
							sz = c.Style.Height.Value
						} else if c.Style.Height.Kind == DimensionPercent {
							sz = containerSize * c.Style.Height.Value / 100.0
						} else {
							sz = t.computeMinContentSize(it.node, false, 0, containerSize)
						}
					}
					if sz > maxItemContent {
						maxItemContent = sz
					}
				}
			}

			if maxItemContent > 0 {
				dst[i] = maxItemContent
				remainingSpace -= maxItemContent
			} else {
				totalFr += 1.0
			}
		}
	}

	// Pass 2: Distribute remaining space to flexible (fr) tracks
	if remainingSpace > 0 && totalFr > 0 {
		frUnit := remainingSpace / totalFr
		for i := 0; i < totalTracks; i++ {
			var dim Dimension
			if i < len(templates) {
				dim = templates[i]
			} else {
				dim = Fr(1)
			}

			switch dim.Kind {
			case DimensionFr:
				dst[i] = dim.Value * frUnit
			case DimensionAuto, DimensionMinContent, DimensionMaxContent:
				if dst[i] == 0 {
					dst[i] = 1.0 * frUnit
				}
			}
		}
	}

	return dst
}
