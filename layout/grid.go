package layout

// layoutGrid implements the W3C CSS Grid Layout Module Level 1 algorithm.
func (t *Tree) layoutGrid(id NodeID, contentWidth, contentHeight, padLeft, padTop, parentWidth, parentHeight float32) {
	node := &t.nodes[id]

	// 1. Resolve Gaps
	colGap := t.resolveDimensionOrZero(node.Style.ColumnGap, contentWidth)
	if node.Style.ColumnGap.Kind == DimensionAuto {
		colGap = t.resolveDimensionOrZero(node.Style.Gap, contentWidth)
	}
	t.curColGap = colGap

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
	stride := totalCols + len(t.gridItems) + 16
	maxExpectedCells := (totalRows + len(t.gridItems) + 16) * stride
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
				if idx >= len(t.occupied) {
					extra := make([]bool, (idx+1)*2)
					copy(extra, t.occupied)
					t.occupied = extra
				}
				t.occupied[idx] = true
			}
		}
	}

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
				if c < cursorCol {
					r = cursorRow + 1
				}
			}

			for {
				if it.colStart >= 0 {
					c = it.colStart
				} else if c+it.colSpan > totalCols {
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

				if it.colStart >= 0 {
					r++
				} else {
					c++
				}
			}
		} else {
			// Column-major auto-placement
			r := cursorRow
			c := cursorCol

			if it.rowStart >= 0 {
				r = it.rowStart
				if r < cursorRow {
					c = cursorCol + 1
				}
			}

			for {
				if it.rowStart >= 0 {
					r = it.rowStart
				} else if r+it.rowSpan > totalRows {
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

				if it.rowStart >= 0 {
					c++
				} else {
					r++
				}
			}
		}
	}

	// 5. Track Sizing Algorithm (Columns & Rows)
	shouldStretchCols := node.Style.JustifyContent == JustifyStretch
	shouldStretchRows := node.Style.AlignContent == AlignContentStretch
	isColIndefinite := (id == t.root && (node.Style.Width.Kind == DimensionAuto || node.Style.Width.Kind == DimensionMinContent || node.Style.Width.Kind == DimensionMaxContent)) || (node.Style.Position == PositionAbsolute && node.Style.Width.Kind == DimensionAuto)
	isRowIndefinite := (id == t.root && (node.Style.Height.Kind == DimensionAuto || node.Style.Height.Kind == DimensionMinContent || node.Style.Height.Kind == DimensionMaxContent)) || (node.Style.Position == PositionAbsolute && node.Style.Height.Kind == DimensionAuto)
	t.colSizes = t.resolveTracks(node.Style.GridTemplateColumns, totalCols, contentWidth, colGap, true, t.colSizes, shouldStretchCols, isColIndefinite)
	t.rowSizes = t.resolveTracks(node.Style.GridTemplateRows, totalRows, contentHeight, rowGap, false, t.rowSizes, shouldStretchRows, isRowIndefinite)

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

	// Update container width/height if Auto
	if node.Style.Width.Kind == DimensionAuto {
		node.Computed.Width = totalColWidth + padLeft + t.resolveDimensionOrZero(node.Style.Padding.Right, parentWidth)
	}
	if node.Style.Height.Kind == DimensionAuto {
		node.Computed.Height = totalRowHeight + padTop + t.resolveDimensionOrZero(node.Style.Padding.Bottom, parentHeight)
	}

	var gridAlignOffsetY, gridJustifyOffsetX float32
	freeY := contentHeight - totalRowHeight
	if freeY < 0 {
		switch node.Style.AlignContent {
		case AlignContentCenter:
			gridAlignOffsetY = freeY * 0.5
		case AlignContentFlexEnd:
			gridAlignOffsetY = freeY
		}
	} else if freeY > 0 {
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
		case AlignContentSpaceEvenly:
			if totalRows > 0 {
				space := freeY / float32(totalRows+1)
				gridAlignOffsetY = space
				rowGap += space
			}
		}
	}

	freeX := contentWidth - totalColWidth
	if freeX < 0 {
		switch node.Style.JustifyContent {
		case JustifyCenter:
			gridJustifyOffsetX = freeX * 0.5
		case JustifyFlexEnd:
			gridJustifyOffsetX = freeX
		}
	} else if freeX > 0 {
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
		case JustifySpaceEvenly:
			if totalCols > 0 {
				space := freeX / float32(totalCols+1)
				gridJustifyOffsetX = space
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
			ref := cellH
			if c.Style.Margin.Top.Kind == DimensionPercent {
				ref = cellW
			}
			mTop = t.resolveDimensionOrZero(c.Style.Margin.Top, ref)
		}
		if !isAutoMBottom {
			ref := cellH
			if c.Style.Margin.Bottom.Kind == DimensionPercent {
				ref = cellW
			}
			mBottom = t.resolveDimensionOrZero(c.Style.Margin.Bottom, ref)
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
			case AlignFlexEnd:
				itemX = cellX + cellW - itemW - mRight
			case AlignCenter:
				itemX = cellX + mLeft + (availW-itemW)*0.5
			default: // AlignStretch, AlignFlexStart, AlignBaseline
				itemX = cellX + mLeft
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
			case AlignFlexEnd:
				itemY = cellY + cellH - itemH - mBottom
			case AlignCenter:
				itemY = cellY + mTop + (availH-itemH)*0.5
			default: // AlignStretch, AlignFlexStart, AlignBaseline
				itemY = cellY + mTop
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

type gridTrackSizing struct {
	minFn       Dimension
	maxFn       Dimension
	fitLimit    float32
	baseSize    float32
	growthLimit float32
}

func (ts *gridTrackSizing) initBaseAndGrowth(containerSize float32, isIndefinite bool) {
	switch ts.minFn.Kind {
	case DimensionPx:
		ts.baseSize = ts.minFn.Value
	case DimensionPercent:
		if !isIndefinite && containerSize > 0 {
			ts.baseSize = containerSize * ts.minFn.Value / 100.0
		} else {
			ts.baseSize = 0
		}
	default:
		ts.baseSize = 0
	}

	switch ts.maxFn.Kind {
	case DimensionPx:
		ts.growthLimit = ts.maxFn.Value
	case DimensionPercent:
		if !isIndefinite && containerSize > 0 {
			ts.growthLimit = containerSize * ts.maxFn.Value / 100.0
		} else {
			ts.growthLimit = 1e9
		}
	case DimensionMinContent:
		ts.growthLimit = ts.baseSize
	case DimensionMaxContent, DimensionAuto, DimensionFr:
		ts.growthLimit = 1e9
	default:
		ts.growthLimit = 1e9
	}

	if ts.fitLimit > 0 && ts.fitLimit < ts.growthLimit {
		ts.growthLimit = ts.fitLimit
	}
	if ts.growthLimit < ts.baseSize {
		ts.growthLimit = ts.baseSize
	}
}

func getTrackSizing(dim Dimension, containerSize float32, isIndefinite bool) gridTrackSizing {
	switch dim.Kind {
	case DimensionMinMax:
		minD := Auto()
		maxD := Auto()
		if dim.Min != nil {
			minD = *dim.Min
		}
		if dim.Max != nil {
			maxD = *dim.Max
		}
		ts := gridTrackSizing{minFn: minD, maxFn: maxD}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	case DimensionFitContent:
		ts := gridTrackSizing{
			minFn:    Dimension{Kind: DimensionAuto},
			maxFn:    Dimension{Kind: DimensionMaxContent},
			fitLimit: dim.Value,
		}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	case DimensionPx:
		return gridTrackSizing{minFn: dim, maxFn: dim, baseSize: dim.Value, growthLimit: dim.Value}
	case DimensionPercent:
		val := float32(0)
		if !isIndefinite && containerSize > 0 {
			val = containerSize * dim.Value / 100.0
		}
		ts := gridTrackSizing{minFn: dim, maxFn: dim, baseSize: val, growthLimit: val}
		if isIndefinite {
			ts.growthLimit = 1e9
		}
		return ts
	case DimensionFr:
		ts := gridTrackSizing{minFn: Auto(), maxFn: dim}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	case DimensionMinContent:
		ts := gridTrackSizing{minFn: dim, maxFn: dim}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	case DimensionMaxContent:
		ts := gridTrackSizing{minFn: dim, maxFn: dim}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	case DimensionAuto:
		ts := gridTrackSizing{minFn: Dimension{Kind: DimensionAuto}, maxFn: Dimension{Kind: DimensionAuto}}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	default:
		ts := gridTrackSizing{minFn: Dimension{Kind: DimensionAuto}, maxFn: Dimension{Kind: DimensionAuto}}
		ts.initBaseAndGrowth(containerSize, isIndefinite)
		return ts
	}
}

func isIntrinsic(k DimensionKind) bool {
	return k == DimensionAuto || k == DimensionMinContent || k == DimensionMaxContent || k == DimensionFitContent
}

func distributeSpaceUpToLimits(
	space float32,
	tracks []gridTrackSizing,
	start, end int,
	isAffected func(*gridTrackSizing) bool,
	getLimit func(*gridTrackSizing) float32,
) float32 {
	remaining := space
	for remaining > 0.0001 {
		countEligible := 0
		for k := start; k < end; k++ {
			if isAffected(&tracks[k]) {
				limit := getLimit(&tracks[k])
				if limit <= 0 || tracks[k].baseSize < limit {
					countEligible++
				}
			}
		}
		if countEligible == 0 {
			break
		}
		share := remaining / float32(countEligible)
		var distributed float32
		for k := start; k < end; k++ {
			if isAffected(&tracks[k]) {
				limit := getLimit(&tracks[k])
				if limit <= 0 {
					tracks[k].baseSize += share
					if tracks[k].growthLimit < tracks[k].baseSize {
						tracks[k].growthLimit = tracks[k].baseSize
					}
					distributed += share
				} else if tracks[k].baseSize < limit {
					add := share
					if tracks[k].baseSize+add > limit {
						add = limit - tracks[k].baseSize
					}
					tracks[k].baseSize += add
					if tracks[k].growthLimit < tracks[k].baseSize {
						tracks[k].growthLimit = tracks[k].baseSize
					}
					distributed += add
				}
			}
		}
		remaining -= distributed
		if distributed <= 0.0001 {
			break
		}
	}
	return remaining
}

// resolveTracks computes the pixel sizes of grid tracks.
func (t *Tree) resolveTracks(templates []Dimension, totalTracks int, containerSize, gap float32, isCol bool, dst []float32, shouldStretch bool, isIndefinite bool) []float32 {
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

	getDim := func(idx int) Dimension {
		if idx < len(templates) {
			return templates[idx]
		}
		return Auto()
	}

	tracks := make([]gridTrackSizing, totalTracks)
	for i := 0; i < totalTracks; i++ {
		tracks[i] = getTrackSizing(getDim(i), containerSize, isIndefinite)
	}

	if isIndefinite {
		for i := 0; i < totalTracks; i++ {
			if tracks[i].minFn.Kind == DimensionPercent || tracks[i].maxFn.Kind == DimensionPercent {
				pct := tracks[i].minFn.Value
				if pct == 0 {
					pct = tracks[i].maxFn.Value
				}
				var maxSpanMC float32
				for _, it := range t.gridItems {
					span := it.colSpan
					start := it.colStart
					if !isCol {
						span = it.rowSpan
						start = it.rowStart
					}
					if start <= i && i < start+span {
						mc := t.computeGridItemMaxContent(&it, isCol, 0)
						if mc > maxSpanMC {
							maxSpanMC = mc
						}
					}
				}
				if maxSpanMC > 0 {
					tracks[i].baseSize = maxSpanMC * pct / 100.0
					tracks[i].growthLimit = tracks[i].baseSize
				}
			}
		}
	}

	// Pass 1: 1-span items
	for i := 0; i < totalTracks; i++ {
		tr := &tracks[i]
		for _, it := range t.gridItems {
			span := it.colSpan
			start := it.colStart
			if !isCol {
				span = it.rowSpan
				start = it.rowStart
			}
			if span == 1 && start == i && tr.maxFn.Kind != DimensionFr {
				minC := t.computeGridItemMinContent(&it, isCol, containerSize)
				maxC := t.computeGridItemMaxContent(&it, isCol, containerSize)

				if isIntrinsic(tr.minFn.Kind) {
					if tr.minFn.Kind == DimensionMaxContent {
						if maxC > tr.baseSize {
							tr.baseSize = maxC
						}
					} else {
						if minC > tr.baseSize {
							tr.baseSize = minC
						}
					}
				}

				if isIntrinsic(tr.maxFn.Kind) {
					if tr.maxFn.Kind == DimensionMinContent {
						if minC > tr.growthLimit {
							tr.growthLimit = minC
						}
					} else {
						limit := maxC
						if tr.fitLimit > 0 && limit > tr.fitLimit {
							limit = tr.fitLimit
						}
						if limit > tr.growthLimit {
							tr.growthLimit = limit
						}
					}
				}
				if tr.growthLimit < tr.baseSize {
					tr.growthLimit = tr.baseSize
				}
			}
		}
	}

	// Pass 2: Multi-span items (W3C Grid §11.5)
	for spanLen := 2; spanLen <= totalTracks; spanLen++ {
		for _, it := range t.gridItems {
			span := it.colSpan
			start := it.colStart
			if !isCol {
				span = it.rowSpan
				start = it.rowStart
			}
			if span == spanLen {
				end := start + span
				if end > totalTracks {
					end = totalTracks
				}
				minC := t.computeGridItemMinContent(&it, isCol, containerSize)
				maxC := t.computeGridItemMaxContent(&it, isCol, containerSize)

				// Check if item spans any flexible tracks
				isFlex := false
				var flexSum float32
				for k := start; k < end; k++ {
					if tracks[k].maxFn.Kind == DimensionFr {
						isFlex = true
						flexSum += tracks[k].maxFn.Value
					}
				}

				// Step 2A: Distribute min-content deficit to baseSizes of intrinsic tracks
				if isFlex {
					var currentBaseSum float32
					for k := start; k < end; k++ {
						currentBaseSum += tracks[k].baseSize
					}
					if minC > currentBaseSum && flexSum > 0 {
						deficit := minC - currentBaseSum
						for k := start; k < end; k++ {
							if tracks[k].maxFn.Kind == DimensionFr {
								share := deficit * (tracks[k].maxFn.Value / flexSum)
								tracks[k].baseSize += share
								if tracks[k].growthLimit < tracks[k].baseSize {
									tracks[k].growthLimit = tracks[k].baseSize
								}
							}
						}
					}
				} else {
					c := &t.nodes[it.node]
					isScroll := (isCol && c.Style.OverflowX != OverflowVisible) || (!isCol && c.Style.OverflowY != OverflowVisible)
					isEligibleMin := func(k DimensionKind) bool {
						if isScroll {
							return k == DimensionMinContent || k == DimensionMaxContent
						}
						return isIntrinsic(k)
					}

					var currentBaseSum float32
					countMinTracks := 0
					for k := start; k < end; k++ {
						currentBaseSum += tracks[k].baseSize
						if isEligibleMin(tracks[k].minFn.Kind) {
							countMinTracks++
						}
					}
					if minC > currentBaseSum {
						deficit := minC - currentBaseSum
						if countMinTracks > 0 {
							share := deficit / float32(countMinTracks)
							for k := start; k < end; k++ {
								if isEligibleMin(tracks[k].minFn.Kind) {
									tracks[k].baseSize += share
									if tracks[k].growthLimit < tracks[k].baseSize {
										tracks[k].growthLimit = tracks[k].baseSize
									}
								}
							}
						} else if end > start {
							share := deficit / float32(end - start)
							for k := start; k < end; k++ {
								tracks[k].baseSize += share
								if tracks[k].growthLimit < tracks[k].baseSize {
									tracks[k].growthLimit = tracks[k].baseSize
								}
							}
						}
					}
				}

				// Step 2B: Distribute max-content deficit
				if isFlex {
					if isIndefinite {
						var currentMaxSum float32
						for k := start; k < end; k++ {
							currentMaxSum += tracks[k].baseSize
						}
						if maxC > currentMaxSum && flexSum > 0 {
							deficit := maxC - currentMaxSum
							for k := start; k < end; k++ {
								if tracks[k].maxFn.Kind == DimensionFr {
									share := deficit * (tracks[k].maxFn.Value / flexSum)
									tracks[k].baseSize += share
									if tracks[k].growthLimit < tracks[k].baseSize {
										tracks[k].growthLimit = tracks[k].baseSize
									}
								}
							}
						}
					}
				} else {
					var currentMaxSum float32
					for k := start; k < end; k++ {
						currentMaxSum += tracks[k].baseSize
					}
					if maxC > currentMaxSum {
						deficit := maxC - currentMaxSum
						countMaxContent := 0
						for k := start; k < end; k++ {
							if tracks[k].minFn.Kind == DimensionMaxContent {
								countMaxContent++
							}
						}
						if countMaxContent > 0 {
							share := deficit / float32(countMaxContent)
							for k := start; k < end; k++ {
								if tracks[k].minFn.Kind == DimensionMaxContent {
									tracks[k].baseSize += share
									tracks[k].growthLimit = tracks[k].baseSize
								}
							}
						} else if isIndefinite {
							// Priority 2: auto or fit-content (only under max-content constraint)
							for deficit > 0.001 {
								countEligible := 0
								for k := start; k < end; k++ {
									if tracks[k].maxFn.Kind == DimensionAuto || (tracks[k].maxFn.Kind == DimensionMaxContent && tracks[k].fitLimit > 0) {
										if tracks[k].fitLimit == 0 || tracks[k].baseSize < tracks[k].fitLimit {
											countEligible++
										}
									}
								}
								if countEligible == 0 {
									break
								}
								share := deficit / float32(countEligible)
								var distributed float32
								for k := start; k < end; k++ {
									if tracks[k].maxFn.Kind == DimensionAuto || (tracks[k].maxFn.Kind == DimensionMaxContent && tracks[k].fitLimit > 0) {
										if tracks[k].fitLimit == 0 {
											tracks[k].baseSize += share
											tracks[k].growthLimit = tracks[k].baseSize
											distributed += share
										} else if tracks[k].baseSize < tracks[k].fitLimit {
											add := share
											if tracks[k].baseSize+add > tracks[k].fitLimit {
												add = tracks[k].fitLimit - tracks[k].baseSize
											}
											tracks[k].baseSize += add
											tracks[k].growthLimit = tracks[k].baseSize
											distributed += add
										}
									}
								}
								deficit -= distributed
								if distributed <= 0.001 {
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// CSS Grid §11.6: Maximise Tracks
	// Expand tracks with a definite maximum size up to their maximum
	if isIndefinite {
		for i := 0; i < totalTracks; i++ {
			if tracks[i].maxFn.Kind == DimensionPx && tracks[i].maxFn.Value > tracks[i].baseSize {
				tracks[i].baseSize = tracks[i].maxFn.Value
			}
		}
	}



	// Set initial sizes
	var usedSpace float32
	for i := 0; i < totalTracks; i++ {
		dst[i] = tracks[i].baseSize
		usedSpace += dst[i]
	}

	// Pass 3: Free space distribution (only if container size is definite)
	if !isIndefinite && containerSize > 0 {
		remainingSpace := containerSize - totalGaps - usedSpace
		if remainingSpace > 0 {
			var totalFr float32
			var autoCount int
			for i := 0; i < totalTracks; i++ {
				if tracks[i].maxFn.Kind == DimensionFr {
					totalFr += tracks[i].maxFn.Value
				} else if tracks[i].maxFn.Kind == DimensionAuto || tracks[i].minFn.Kind == DimensionAuto {
					if dst[i] == 0 {
						totalFr += 1.0
					} else {
						autoCount++
					}
				}
			}

			if totalFr > 0 {
				frUnit := remainingSpace / totalFr
				for i := 0; i < totalTracks; i++ {
					if tracks[i].maxFn.Kind == DimensionFr {
						dst[i] = tracks[i].maxFn.Value * frUnit
					} else if (tracks[i].maxFn.Kind == DimensionAuto || tracks[i].minFn.Kind == DimensionAuto) && dst[i] == 0 {
						dst[i] = 1.0 * frUnit
					}
				}
			} else if autoCount > 0 && shouldStretch {
				share := remainingSpace / float32(autoCount)
				for i := 0; i < totalTracks; i++ {
					if tracks[i].maxFn.Kind == DimensionAuto || tracks[i].minFn.Kind == DimensionAuto {
						dst[i] += share
					}
				}
			}
		}
	}

	return dst
}

func (t *Tree) getSpannedColWidth(it *gridItemScratch) float32 {
	if len(t.colSizes) == 0 {
		return 0
	}
	var total float32
	start := it.colStart
	end := start + it.colSpan
	if start < 0 {
		start = 0
	}
	if end > len(t.colSizes) {
		end = len(t.colSizes)
	}
	for c := start; c < end; c++ {
		total += t.colSizes[c]
	}
	if end-start > 1 {
		total += float32(end-start-1) * t.curColGap
	}
	return total
}

func (t *Tree) computeGridItemMinContent(it *gridItemScratch, isCol bool, containerSize float32) float32 {
	c := &t.nodes[it.node]
	if isCol {
		if c.Style.Width.Kind == DimensionPx {
			return c.Style.Width.Value
		} else if c.Style.Width.Kind == DimensionPercent {
			return containerSize * c.Style.Width.Value / 100.0
		}
		if int(it.node) < len(t.measureFuncs) && t.measureFuncs[it.node] != nil {
			fn := t.measureFuncs[it.node]
			mw, _ := fn(t, it.node, 0, MeasureAtMost, 0, MeasureUndefined)
			return mw
		}
		return t.computeMinContentSize(it.node, true, containerSize, 0)
	} else {
		if c.Style.Height.Kind == DimensionPx {
			return c.Style.Height.Value
		} else if c.Style.Height.Kind == DimensionPercent {
			return containerSize * c.Style.Height.Value / 100.0
		}
		colW := t.getSpannedColWidth(it)
		if int(it.node) < len(t.measureFuncs) && t.measureFuncs[it.node] != nil {
			fn := t.measureFuncs[it.node]
			if colW > 0 {
				_, mh := fn(t, it.node, colW, MeasureExact, 0, MeasureUndefined)
				return mh
			}
			_, mh := fn(t, it.node, 0, MeasureUndefined, 0, MeasureAtMost)
			return mh
		}
		return t.computeMinContentSize(it.node, false, colW, containerSize)
	}
}

func (t *Tree) computeGridItemMaxContent(it *gridItemScratch, isCol bool, containerSize float32) float32 {
	c := &t.nodes[it.node]
	if isCol {
		if c.Style.Width.Kind == DimensionPx {
			return c.Style.Width.Value
		} else if c.Style.Width.Kind == DimensionPercent {
			return containerSize * c.Style.Width.Value / 100.0
		}
		if int(it.node) < len(t.measureFuncs) && t.measureFuncs[it.node] != nil {
			fn := t.measureFuncs[it.node]
			mw, _ := fn(t, it.node, 0, MeasureUndefined, 0, MeasureUndefined)
			return mw
		}
		return t.computeMaxContentSize(it.node, true, containerSize)
	} else {
		if c.Style.Height.Kind == DimensionPx {
			return c.Style.Height.Value
		} else if c.Style.Height.Kind == DimensionPercent {
			return containerSize * c.Style.Height.Value / 100.0
		}
		colW := t.getSpannedColWidth(it)
		if int(it.node) < len(t.measureFuncs) && t.measureFuncs[it.node] != nil {
			fn := t.measureFuncs[it.node]
			if colW > 0 {
				_, mh := fn(t, it.node, colW, MeasureExact, 0, MeasureUndefined)
				return mh
			}
			_, mh := fn(t, it.node, 0, MeasureUndefined, 0, MeasureUndefined)
			return mh
		}
		return t.computeMaxContentSize(it.node, false, containerSize)
	}
}
