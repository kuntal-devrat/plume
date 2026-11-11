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
	t.colSizes = t.resolveTracks(node.Style.GridTemplateColumns, totalCols, contentWidth, colGap, t.colSizes)
	t.rowSizes = t.resolveTracks(node.Style.GridTemplateRows, totalRows, contentHeight, rowGap, t.rowSizes)

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

	// 6. Item Positioning & Sizing
	for _, it := range t.gridItems {
		c := &t.nodes[it.node]

		// Cell left X & width
		var cellX float32 = padLeft
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
		var cellY float32 = padTop
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
		if c.Style.JustifySelf != AlignSelfAuto {
			justify = AlignItems(c.Style.JustifySelf)
		}

		align := node.Style.AlignItems
		if c.Style.AlignSelf != AlignSelfAuto {
			align = AlignItems(c.Style.AlignSelf)
		}

		// Item resolved size
		itemW := cellW
		if c.Style.Width.Kind != DimensionAuto {
			itemW = t.resolveDimensionOrZero(c.Style.Width, cellW)
		}
		itemW = t.clampSize(itemW, c.Style.MinWidth, c.Style.MaxWidth, cellW)

		itemH := cellH
		if c.Style.Height.Kind != DimensionAuto {
			itemH = t.resolveDimensionOrZero(c.Style.Height, cellH)
		}
		itemH = t.clampSize(itemH, c.Style.MinHeight, c.Style.MaxHeight, cellH)

		// Item offsets
		itemX := cellX
		switch justify {
		case AlignStretch:
			if c.Style.Width.Kind == DimensionAuto {
				itemW = cellW
			}
		case AlignFlexStart:
			itemX = cellX
		case AlignFlexEnd:
			itemX = cellX + cellW - itemW
		case AlignCenter:
			itemX = cellX + (cellW-itemW)*0.5
		}

		itemY := cellY
		switch align {
		case AlignStretch:
			if c.Style.Height.Kind == DimensionAuto {
				itemH = cellH
			}
		case AlignFlexStart:
			itemY = cellY
		case AlignFlexEnd:
			itemY = cellY + cellH - itemH
		case AlignCenter:
			itemY = cellY + (cellH-itemH)*0.5
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
func (t *Tree) resolveTracks(templates []Dimension, totalTracks int, containerSize, gap float32, dst []float32) []float32 {
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

	// Pass 1: Resolve fixed and percentage tracks
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
			// Auto tracks without explicit fr or size default to flexible share
			totalFr += 1.0
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
				dst[i] = 1.0 * frUnit
			}
		}
	}

	return dst
}
