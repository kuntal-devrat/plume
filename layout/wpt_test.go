package layout

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Raw XML structures for W3C / Taffy XML test fixtures

type xmlTestDoc struct {
	XMLName      xml.Name        `xml:"test"`
	Name         string          `xml:"name,attr"`
	UseRounding  string          `xml:"use-rounding,attr"`
	Viewport     xmlViewport     `xml:"viewport"`
	Input        xmlGenericNode  `xml:"input"`
	Expectations xmlExpectations `xml:"expectations"`
}

type xmlViewport struct {
	Width  string `xml:"width,attr"`
	Height string `xml:"height,attr"`
}

type xmlExpectations struct {
	Nodes []xmlExpectNode `xml:"node"`
}

type xmlExpectNode struct {
	X        float32         `xml:"x,attr"`
	Y        float32         `xml:"y,attr"`
	Width    float32         `xml:"width,attr"`
	Height   float32         `xml:"height,attr"`
	Children []xmlExpectNode `xml:"node"`
}

type xmlGenericNode struct {
	XMLName  xml.Name         `xml:""`
	Attrs    []xml.Attr       `xml:",any,attr"`
	Content  string           `xml:",chardata"`
	Children []xmlGenericNode `xml:",any"`
}

func parseWPTDimension(val string) Dimension {
	val = strings.TrimSpace(val)
	if val == "" || val == "auto" {
		return Auto()
	}
	if val == "max-content" {
		return Dimension{Kind: DimensionMaxContent}
	}
	if val == "min-content" {
		return Dimension{Kind: DimensionMinContent}
	}
	if strings.HasPrefix(val, "fit-content(") && strings.HasSuffix(val, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(val, "fit-content("), ")")
		d := parseWPTDimension(inner)
		return FitContent(d)
	}
	if strings.HasPrefix(val, "minmax(") && strings.HasSuffix(val, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(val, "minmax("), ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 2 {
			d1 := parseWPTDimension(strings.TrimSpace(parts[0]))
			d2 := parseWPTDimension(strings.TrimSpace(parts[1]))
			return MinMax(d1, d2)
		}
	}
	if strings.HasSuffix(val, "px") {
		num := strings.TrimSuffix(val, "px")
		f, _ := strconv.ParseFloat(num, 32)
		return Px(float32(f))
	}
	if strings.HasSuffix(val, "%") {
		num := strings.TrimSuffix(val, "%")
		f, _ := strconv.ParseFloat(num, 32)
		return Percent(float32(f))
	}
	if strings.HasSuffix(val, "fr") {
		num := strings.TrimSuffix(val, "fr")
		f, _ := strconv.ParseFloat(num, 32)
		return Fr(float32(f))
	}
	// Try plain number
	if f, err := strconv.ParseFloat(val, 32); err == nil {
		return Px(float32(f))
	}
	return Auto()
}

func parseWPTTrackList(val string, getAttr func(string) string, isCol bool) []Dimension {
	val = strings.TrimSpace(val)
	if val == "" || val == "none" {
		return nil
	}
	var parts []string
	var cur strings.Builder
	depth := 0
	for _, r := range val {
		if r == '(' {
			depth++
			cur.WriteRune(r)
		} else if r == ')' {
			depth--
			cur.WriteRune(r)
		} else if (r == ' ' || r == '\t' || r == '\n') && depth == 0 {
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		} else {
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	var res []Dimension
	for _, p := range parts {
		if strings.HasPrefix(p, "repeat(") && strings.HasSuffix(p, ")") {
			inner := strings.TrimSuffix(strings.TrimPrefix(p, "repeat("), ")")
			var args []string
			var aCur strings.Builder
			aDepth := 0
			for _, r := range inner {
				if r == '(' {
					aDepth++
					aCur.WriteRune(r)
				} else if r == ')' {
					aDepth--
					aCur.WriteRune(r)
				} else if r == ',' && aDepth == 0 {
					args = append(args, strings.TrimSpace(aCur.String()))
					aCur.Reset()
				} else {
					aCur.WriteRune(r)
				}
			}
			if aCur.Len() > 0 {
				args = append(args, strings.TrimSpace(aCur.String()))
			}
			if len(args) >= 2 {
				countStr := args[0]
				trackSpec := args[1]
				trackDim := parseWPTDimension(trackSpec)
				var count int
				if n, err := strconv.Atoi(countStr); err == nil {
					count = n
				} else if countStr == "auto-fill" || countStr == "auto-fit" {
					var avail float32
					if isCol {
						wStr := getAttr("width")
						if strings.HasSuffix(wStr, "px") {
							f, _ := strconv.ParseFloat(strings.TrimSuffix(wStr, "px"), 32)
							avail = float32(f)
						} else {
							avail = 710
						}
						for _, prev := range res {
							if prev.Kind == DimensionPx {
								avail -= prev.Value
							}
						}
					}
					trackMin := float32(40)
					if trackDim.Kind == DimensionPx {
						trackMin = trackDim.Value
					} else if trackDim.Kind == DimensionMinMax && trackDim.Min != nil && trackDim.Min.Kind == DimensionPx {
						trackMin = trackDim.Min.Value
					}
					var gapVal float32
					gapStr := getAttr("gap")
					if isCol && getAttr("column-gap") != "" {
						gapStr = getAttr("column-gap")
					}
					if strings.HasSuffix(gapStr, "px") {
						f, _ := strconv.ParseFloat(strings.TrimSuffix(gapStr, "px"), 32)
						gapVal = float32(f)
					}
					count = int((avail + gapVal) / (trackMin + gapVal))
					if count < 1 {
						count = 1
					}
				}
				for k := 0; k < count; k++ {
					res = append(res, trackDim)
				}
				continue
			}
		}
		res = append(res, parseWPTDimension(p))
	}
	return res
}

func parseWPTStyle(node *xmlGenericNode) Style {
	s := DefaultStyle()

	getAttr := func(name string) string {
		for _, a := range node.Attrs {
			if a.Name.Local == name {
				return a.Value
			}
		}
		return ""
	}

	// Display
	disp := getAttr("display")
	switch disp {
	case "flex":
		s.Display = DisplayFlex
	case "grid":
		s.Display = DisplayGrid
	case "block":
		s.Display = DisplayBlock
	case "none":
		s.Display = DisplayNone
	default:
		if getAttr("grid-template-columns") != "" || getAttr("grid-template-rows") != "" {
			s.Display = DisplayGrid
		} else {
			for _, ch := range node.Children {
				for _, a := range ch.Attrs {
					if a.Name.Local == "display" && a.Value == "grid" {
						s.Display = DisplayBlock
						break
					}
				}
			}
		}
	}

	// Direction
	dirAttr := getAttr("direction")
	if dirAttr == "rtl" {
		s.Direction = DirectionRTL
	} else if dirAttr == "ltr" {
		s.Direction = DirectionLTR
	}

	// Position
	pos := getAttr("position")
	switch pos {
	case "absolute":
		s.Position = PositionAbsolute
	case "relative":
		s.Position = PositionRelative
	}

	// Flex Direction
	fd := getAttr("flex-direction")
	switch fd {
	case "row":
		s.FlexDirection = FlexDirectionRow
	case "column":
		s.FlexDirection = FlexDirectionColumn
	case "row-reverse":
		s.FlexDirection = FlexDirectionRowReverse
	case "column-reverse":
		s.FlexDirection = FlexDirectionColumnReverse
	}

	// Flex Wrap
	fw := getAttr("flex-wrap")
	switch fw {
	case "nowrap":
		s.FlexWrap = FlexWrapNoWrap
	case "wrap":
		s.FlexWrap = FlexWrapWrap
	case "wrap-reverse":
		s.FlexWrap = FlexWrapWrapReverse
	}

	// Justify Content
	jc := getAttr("justify-content")
	switch jc {
	case "flex-start":
		s.JustifyContent = JustifyFlexStart
	case "start":
		s.JustifyContent = JustifyStart
	case "flex-end":
		s.JustifyContent = JustifyFlexEnd
	case "end":
		s.JustifyContent = JustifyEnd
	case "center":
		s.JustifyContent = JustifyCenter
	case "space-between":
		s.JustifyContent = JustifySpaceBetween
	case "space-around":
		s.JustifyContent = JustifySpaceAround
	case "space-evenly":
		s.JustifyContent = JustifySpaceEvenly
	case "stretch":
		s.JustifyContent = JustifyStretch
	default:
		if s.Display == DisplayGrid {
			s.JustifyContent = JustifyStretch
		}
	}

	// Align Items
	ai := getAttr("align-items")
	switch ai {
	case "stretch":
		s.AlignItems = AlignStretch
	case "flex-start", "start", "self-start":
		s.AlignItems = AlignFlexStart
	case "flex-end", "end", "self-end":
		s.AlignItems = AlignFlexEnd
	case "center":
		s.AlignItems = AlignCenter
	case "baseline":
		s.AlignItems = AlignBaseline
	}

	// Justify Items
	ji := getAttr("justify-items")
	switch ji {
	case "stretch":
		s.JustifyItems = AlignStretch
	case "flex-start", "start", "self-start":
		s.JustifyItems = AlignFlexStart
	case "flex-end", "end", "self-end":
		s.JustifyItems = AlignFlexEnd
	case "center":
		s.JustifyItems = AlignCenter
	}

	// Align Self
	as := getAttr("align-self")
	switch as {
	case "auto":
		s.AlignSelf = AlignSelfAuto
	case "stretch":
		s.AlignSelf = AlignSelfStretch
	case "flex-start", "start", "self-start":
		s.AlignSelf = AlignSelfFlexStart
	case "flex-end", "end", "self-end":
		s.AlignSelf = AlignSelfFlexEnd
	case "center":
		s.AlignSelf = AlignSelfCenter
	case "baseline":
		s.AlignSelf = AlignSelfBaseline
	}

	// Justify Self
	js := getAttr("justify-self")
	switch js {
	case "auto":
		s.JustifySelf = AlignSelfAuto
	case "stretch":
		s.JustifySelf = AlignSelfStretch
	case "flex-start", "start", "self-start":
		s.JustifySelf = AlignSelfFlexStart
	case "flex-end", "end", "self-end":
		s.JustifySelf = AlignSelfFlexEnd
	case "center":
		s.JustifySelf = AlignSelfCenter
	}

	// Align Content
	ac := getAttr("align-content")
	switch ac {
	case "stretch":
		s.AlignContent = AlignContentStretch
	case "flex-start", "start":
		s.AlignContent = AlignContentFlexStart
	case "flex-end", "end":
		s.AlignContent = AlignContentFlexEnd
	case "center":
		s.AlignContent = AlignContentCenter
	case "space-between":
		s.AlignContent = AlignContentSpaceBetween
	case "space-around":
		s.AlignContent = AlignContentSpaceAround
	case "space-evenly":
		s.AlignContent = AlignContentSpaceEvenly
	default:
		if s.Display == DisplayGrid {
			s.AlignContent = AlignContentStretch
		}
	}

	// Overflow
	if v := getAttr("overflow-x"); v != "" {
		switch v {
		case "hidden":
			s.OverflowX = OverflowHidden
		case "scroll":
			s.OverflowX = OverflowScroll
		case "auto":
			s.OverflowX = OverflowAuto
		}
	}
	if v := getAttr("overflow-y"); v != "" {
		switch v {
		case "hidden":
			s.OverflowY = OverflowHidden
		case "scroll":
			s.OverflowY = OverflowScroll
		case "auto":
			s.OverflowY = OverflowAuto
		}
	}
	if v := getAttr("overflow"); v != "" {
		switch v {
		case "hidden":
			s.OverflowX = OverflowHidden
			s.OverflowY = OverflowHidden
		case "scroll":
			s.OverflowX = OverflowScroll
			s.OverflowY = OverflowScroll
		case "auto":
			s.OverflowX = OverflowAuto
			s.OverflowY = OverflowAuto
		}
	}

	// Flex factors
	if v := getAttr("flex-grow"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			s.FlexGrow = float32(f)
		}
	}
	if v := getAttr("flex-shrink"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			s.FlexShrink = float32(f)
		}
	}
	if v := getAttr("flex-basis"); v != "" {
		switch v {
		case "stretch":
			s.FlexBasis = Dimension{Kind: DimensionPercent, Value: 100}
		case "content":
			s.FlexBasis = Dimension{Kind: DimensionContent}
		case "min-content":
			s.FlexBasis = Dimension{Kind: DimensionMinContent}
		case "max-content":
			s.FlexBasis = Dimension{Kind: DimensionMaxContent}
		case "fit-content":
			s.FlexBasis = Dimension{Kind: DimensionFitContent}
		default:
			s.FlexBasis = parseWPTDimension(v)
		}
	}

	// Dimensions
	if v := getAttr("width"); v != "" {
		s.Width = parseWPTDimension(v)
	}
	if v := getAttr("height"); v != "" {
		s.Height = parseWPTDimension(v)
	}
	if v := getAttr("min-width"); v != "" {
		s.MinWidth = parseWPTDimension(v)
	}
	if v := getAttr("min-height"); v != "" {
		s.MinHeight = parseWPTDimension(v)
	}
	if v := getAttr("max-width"); v != "" {
		s.MaxWidth = parseWPTDimension(v)
	}
	if v := getAttr("max-height"); v != "" {
		s.MaxHeight = parseWPTDimension(v)
	}
	if v := getAttr("aspect-ratio"); v != "" {
		if strings.Contains(v, "/") {
			parts := strings.Split(v, "/")
			if len(parts) == 2 {
				n, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 32)
				d, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 32)
				if d > 0 {
					s.AspectRatio = float32(n / d)
				}
			}
		} else if f, err := strconv.ParseFloat(v, 32); err == nil {
			s.AspectRatio = float32(f)
		}
	}

	// Insets
	if v := getAttr("top"); v != "" {
		s.Inset.Top = parseWPTDimension(v)
	}
	if v := getAttr("right"); v != "" {
		s.Inset.Right = parseWPTDimension(v)
	}
	if v := getAttr("bottom"); v != "" {
		s.Inset.Bottom = parseWPTDimension(v)
	}
	if v := getAttr("left"); v != "" {
		s.Inset.Left = parseWPTDimension(v)
	}

	// Margins
	if v := getAttr("margin-top"); v != "" {
		s.Margin.Top = parseWPTDimension(v)
	}
	if v := getAttr("margin-right"); v != "" {
		s.Margin.Right = parseWPTDimension(v)
	}
	if v := getAttr("margin-bottom"); v != "" {
		s.Margin.Bottom = parseWPTDimension(v)
	}
	if v := getAttr("margin-left"); v != "" {
		s.Margin.Left = parseWPTDimension(v)
	}

	// Padding
	if v := getAttr("padding-top"); v != "" {
		s.Padding.Top = parseWPTDimension(v)
	}
	if v := getAttr("padding-right"); v != "" {
		s.Padding.Right = parseWPTDimension(v)
	}
	if v := getAttr("padding-bottom"); v != "" {
		s.Padding.Bottom = parseWPTDimension(v)
	}
	if v := getAttr("padding-left"); v != "" {
		s.Padding.Left = parseWPTDimension(v)
	}

	// Border (adds to padding for layout calculation)
	addBorder := func(attr string, target *Dimension) {
		if v := getAttr(attr); v != "" {
			bd := parseWPTDimension(v)
			if bd.Kind == DimensionPx {
				target.Value += bd.Value
				target.Kind = DimensionPx
			}
		}
	}
	addBorder("border-top", &s.Padding.Top)
	addBorder("border-right", &s.Padding.Right)
	addBorder("border-bottom", &s.Padding.Bottom)
	addBorder("border-left", &s.Padding.Left)

	// Gaps
	if v := getAttr("gap"); v != "" {
		s.Gap = parseWPTDimension(v)
	}
	if v := getAttr("row-gap"); v != "" {
		s.RowGap = parseWPTDimension(v)
	}
	if v := getAttr("column-gap"); v != "" {
		s.ColumnGap = parseWPTDimension(v)
	}

	// Grid
	if v := getAttr("grid-template-columns"); v != "" {
		s.GridTemplateColumns = parseWPTTrackList(v, getAttr, true)
		if strings.Contains(v, "auto-fit") {
			s.AutoFitCols = true
		}
	}
	if v := getAttr("grid-template-rows"); v != "" {
		s.GridTemplateRows = parseWPTTrackList(v, getAttr, false)
		if strings.Contains(v, "auto-fit") {
			s.AutoFitRows = true
		}
	}
	if v := getAttr("grid-auto-columns"); v != "" {
		s.GridAutoColumns = parseWPTTrackList(v, getAttr, true)
	}
	if v := getAttr("grid-auto-rows"); v != "" {
		s.GridAutoRows = parseWPTTrackList(v, getAttr, false)
	}
	if v := getAttr("grid-auto-flow"); v != "" {
		switch v {
		case "row":
			s.GridAutoFlow = GridAutoFlowRow
		case "column":
			s.GridAutoFlow = GridAutoFlowColumn
		case "row dense":
			s.GridAutoFlow = GridAutoFlowRowDense
		case "column dense":
			s.GridAutoFlow = GridAutoFlowColumnDense
		}
	}
	parseSpan := func(v string) (val int16, isSpan bool) {
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "span") {
			parts := strings.Fields(v)
			if len(parts) >= 2 {
				if n, err := strconv.Atoi(parts[1]); err == nil {
					return int16(n), true
				}
			}
			return 1, true
		}
		if n, err := strconv.Atoi(v); err == nil {
			return int16(n), false
		}
		return 0, false
	}

	if v := getAttr("grid-column-start"); v != "" {
		if val, isSpan := parseSpan(v); isSpan {
			s.GridColumnSpan = uint16(val)
		} else if val != 0 {
			s.GridColumnStart = val
		}
	}
	if v := getAttr("grid-column-end"); v != "" {
		if val, isSpan := parseSpan(v); isSpan {
			s.GridColumnSpan = uint16(val)
		} else if val != 0 {
			s.GridColumnEnd = val
		}
	}
	if v := getAttr("grid-row-start"); v != "" {
		if val, isSpan := parseSpan(v); isSpan {
			s.GridRowSpan = uint16(val)
		} else if val != 0 {
			s.GridRowStart = val
		}
	}
	if v := getAttr("grid-row-end"); v != "" {
		if val, isSpan := parseSpan(v); isSpan {
			s.GridRowSpan = uint16(val)
		} else if val != 0 {
			s.GridRowEnd = val
		}
	}

	boxSizing := getAttr("box-sizing")
	if boxSizing == "content-box" || boxSizing == "" {
		s.BoxSizing = BoxSizingContentBox
	} else if boxSizing == "border-box" {
		s.BoxSizing = BoxSizingBorderBox
	}

	return s
}

func setupAhemTextMeasure(tree *Tree, nodeID NodeID, text string) {
	text = strings.TrimSpace(text)
	tree.SetBaseline(nodeID, 8.0) // 8px ascent for 10px Ahem font
	tree.SetMeasureFunc(nodeID, func(t *Tree, node NodeID, w float32, wMode MeasureMode, h float32, hMode MeasureMode) (float32, float32) {
		var words []float32
		var totalChars float32
		var curWordChars float32

		for _, r := range text {
			if r == '\u200B' {
				if curWordChars > 0 {
					words = append(words, curWordChars*10.0)
					curWordChars = 0
				}
			} else if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
				if curWordChars > 0 {
					words = append(words, curWordChars*10.0)
					curWordChars = 0
				}
				totalChars += 1.0
			} else {
				curWordChars += 1.0
				totalChars += 1.0
			}
		}
		if curWordChars > 0 {
			words = append(words, curWordChars*10.0)
		}

		maxWordWidth := float32(0)
		for _, wVal := range words {
			if wVal > maxWordWidth {
				maxWordWidth = wVal
			}
		}
		maxContentWidth := totalChars * 10.0
		if maxContentWidth == 0 {
			maxContentWidth = maxWordWidth
		}

		switch wMode {
		case MeasureExact:
			if w <= 0 {
				return 0, 0
			}
			if w >= maxContentWidth {
				return w, 10.0
			}
			lines := 1
			lineW := float32(0)
			for _, ww := range words {
				if lineW > 0 && lineW+ww > w {
					lines++
					lineW = ww
				} else {
					lineW += ww
				}
			}
			return w, float32(lines) * 10.0
		case MeasureAtMost:
			if w <= 0 {
				return maxWordWidth, float32(len(words)) * 10.0
			}
			if w >= maxContentWidth {
				return maxContentWidth, 10.0
			}
			lines := 1
			lineW := float32(0)
			for _, ww := range words {
				if lineW > 0 && lineW+ww > w {
					lines++
					lineW = ww
				} else {
					lineW += ww
				}
			}
			return w, float32(lines) * 10.0
		default: // MeasureUndefined
			return maxContentWidth, 10.0
		}
	})
}

func buildPlumeTreeFromXML(tree *Tree, xmlNode *xmlGenericNode) NodeID {
	style := parseWPTStyle(xmlNode)
	nodeID := tree.NewNode(style)
	content := strings.TrimSpace(xmlNode.Content)
	if xmlNode.XMLName.Local == "text" || content != "" {
		setupAhemTextMeasure(tree, nodeID, xmlNode.Content)
	}
	for i := range xmlNode.Children {
		childID := buildPlumeTreeFromXML(tree, &xmlNode.Children[i])
		tree.AddChild(nodeID, childID)
	}
	return nodeID
}

func verifyExpectations(t *testing.T, tree *Tree, nodeID NodeID, expected *xmlExpectNode, tolerance float32, path string) (bool, string) {
	rect := tree.ComputedRect(nodeID)

	dx := math.Abs(float64(rect.X - expected.X))
	dy := math.Abs(float64(rect.Y - expected.Y))
	dw := math.Abs(float64(rect.Width - expected.Width))
	dh := math.Abs(float64(rect.Height - expected.Height))

	tol := float64(tolerance)
	if dx > tol || dy > tol || dw > tol || dh > tol {
		diff := fmt.Sprintf("at %s: expected [x:%.1f y:%.1f w:%.1f h:%.1f] but got [x:%.1f y:%.1f w:%.1f h:%.1f] (diff: dx=%.1f dy=%.1f dw=%.1f dh=%.1f)",
			path, expected.X, expected.Y, expected.Width, expected.Height,
			rect.X, rect.Y, rect.Width, rect.Height, dx, dy, dw, dh)
		return false, diff
	}

	children := tree.Children(nodeID)
	if len(children) != len(expected.Children) {
		diff := fmt.Sprintf("at %s: expected %d children but got %d", path, len(expected.Children), len(children))
		return false, diff
	}

	for i := range children {
		childPath := fmt.Sprintf("%s/%d", path, i)
		if ok, diff := verifyExpectations(t, tree, children[i], &expected.Children[i], tolerance, childPath); !ok {
			return false, diff
		}
	}

	return true, ""
}

func TestWPTAutomatedFlexAndGrid(t *testing.T) {
	testDir := "testdata/wpt"
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Skip("testdata/wpt directory not found, skipping WPT suite")
	}

	var flexFiles, gridFiles []string
	filepath.WalkDir(testDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".xml") {
			if strings.Contains(path, "/flex/") {
				flexFiles = append(flexFiles, path)
			} else if strings.Contains(path, "/grid/") {
				gridFiles = append(gridFiles, path)
			}
		}
		return nil
	})

	t.Logf("Found %d Flexbox WPT test fixtures, %d CSS Grid WPT test fixtures", len(flexFiles), len(gridFiles))

	runSuite := func(category string, files []string) {
		passed := 0
		failed := 0
		skipped := 0

		filter := os.Getenv("WPT_FILTER")
		for _, file := range files {
			if filter != "" && !strings.Contains(filepath.Base(file), filter) {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			var doc xmlTestDoc
			if err := xml.Unmarshal(data, &doc); err != nil {
				skipped++
				continue
			}

			if len(doc.Input.Children) == 0 || len(doc.Expectations.Nodes) == 0 {
				skipped++
				continue
			}

			// Viewport sizing
			var availW, availH float32 = 800, 600
			if doc.Viewport.Width != "" && doc.Viewport.Width != "max-content" && doc.Viewport.Width != "auto" {
				if d := parseWPTDimension(doc.Viewport.Width); d.Kind == DimensionPx {
					availW = d.Value
				}
			}
			if doc.Viewport.Height != "" && doc.Viewport.Height != "max-content" && doc.Viewport.Height != "auto" {
				if d := parseWPTDimension(doc.Viewport.Height); d.Kind == DimensionPx {
					availH = d.Value
				}
			}

			tree := NewTree()
			rootInput := &doc.Input.Children[0]
			rootID := buildPlumeTreeFromXML(tree, rootInput)

			tree.Compute(availW, availH)

			expectedRoot := &doc.Expectations.Nodes[0]
			if ok, diff := verifyExpectations(t, tree, rootID, expectedRoot, 1.0, doc.Name); ok {
				passed++
				if filter != "" {
					t.Logf("[%s PASS] %s", category, filepath.Base(file))
				}
			} else {
				failed++
				t.Logf("[%s FAIL] %s: %s", category, filepath.Base(file), diff)
				if filter != "" {
					for n := 0; n < len(tree.nodes); n++ {
						t.Logf("  node %d: computed=%+v style.Display=%v MaxHeight=%+v Height=%+v Padding=%+v",
							n, tree.nodes[n].Computed, tree.nodes[n].Style.Display, tree.nodes[n].Style.MaxHeight, tree.nodes[n].Style.Height, tree.nodes[n].Style.Padding)
					}
				}
			}
		}

		t.Logf("[%s WPT Suite] Passed: %d, Failed: %d, Skipped/Incompatible: %d, Conformance: %.1f%%",
			category, passed, failed, skipped, float32(passed)/float32(passed+failed)*100.0)
	}

	t.Run("W3C_Flexbox", func(t *testing.T) {
		runSuite("Flexbox", flexFiles)
	})

	t.Run("W3C_CSS_Grid", func(t *testing.T) {
		runSuite("CSS Grid", gridFiles)
	})
}
