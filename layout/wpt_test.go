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
	Children []xmlGenericNode `xml:",any"`
}

func parseWPTDimension(val string) Dimension {
	val = strings.TrimSpace(val)
	if val == "" || val == "auto" || val == "max-content" || val == "min-content" {
		return Auto()
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

func parseWPTTrackList(val string) []Dimension {
	val = strings.TrimSpace(val)
	if val == "" || val == "none" {
		return nil
	}
	parts := strings.Fields(val)
	var res []Dimension
	for _, p := range parts {
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
	case "flex-start", "start":
		s.JustifyContent = JustifyFlexStart
	case "flex-end", "end":
		s.JustifyContent = JustifyFlexEnd
	case "center":
		s.JustifyContent = JustifyCenter
	case "space-between":
		s.JustifyContent = JustifySpaceBetween
	case "space-around":
		s.JustifyContent = JustifySpaceAround
	case "space-evenly":
		s.JustifyContent = JustifySpaceEvenly
	}

	// Align Items
	ai := getAttr("align-items")
	switch ai {
	case "stretch":
		s.AlignItems = AlignStretch
	case "flex-start", "start":
		s.AlignItems = AlignFlexStart
	case "flex-end", "end":
		s.AlignItems = AlignFlexEnd
	case "center":
		s.AlignItems = AlignCenter
	case "baseline":
		s.AlignItems = AlignBaseline
	}

	// Align Self
	as := getAttr("align-self")
	switch as {
	case "auto":
		s.AlignSelf = AlignSelfAuto
	case "stretch":
		s.AlignSelf = AlignSelfStretch
	case "flex-start", "start":
		s.AlignSelf = AlignSelfFlexStart
	case "flex-end", "end":
		s.AlignSelf = AlignSelfFlexEnd
	case "center":
		s.AlignSelf = AlignSelfCenter
	case "baseline":
		s.AlignSelf = AlignSelfBaseline
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
		s.FlexBasis = parseWPTDimension(v)
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
		s.GridTemplateColumns = parseWPTTrackList(v)
	}
	if v := getAttr("grid-template-rows"); v != "" {
		s.GridTemplateRows = parseWPTTrackList(v)
	}
	if v := getAttr("grid-column-start"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.GridColumnStart = int16(n)
		}
	}
	if v := getAttr("grid-column-end"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.GridColumnEnd = int16(n)
		}
	}
	if v := getAttr("grid-row-start"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.GridRowStart = int16(n)
		}
	}
	if v := getAttr("grid-row-end"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.GridRowEnd = int16(n)
		}
	}

	// In content-box (CSS default), explicit width/height properties apply to the content area,
	// so the outer box dimensions equal content + padding + border.
	boxSizing := getAttr("box-sizing")
	if boxSizing == "" || boxSizing == "content-box" {
		padHoriz := s.Padding.Left.Value + s.Padding.Right.Value
		padVert := s.Padding.Top.Value + s.Padding.Bottom.Value
		if s.Width.Kind == DimensionPx {
			s.Width.Value += padHoriz
		}
		if s.Height.Kind == DimensionPx {
			s.Height.Value += padVert
		}
		if s.MinWidth.Kind == DimensionPx {
			s.MinWidth.Value += padHoriz
		}
		if s.MinHeight.Kind == DimensionPx {
			s.MinHeight.Value += padVert
		}
		if s.MaxWidth.Kind == DimensionPx {
			s.MaxWidth.Value += padHoriz
		}
		if s.MaxHeight.Kind == DimensionPx {
			s.MaxHeight.Value += padVert
		}
	}

	return s
}

func buildPlumeTreeFromXML(tree *Tree, xmlNode *xmlGenericNode) NodeID {
	style := parseWPTStyle(xmlNode)
	nodeID := tree.NewNode(style)
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

		for _, file := range files {
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
			} else {
				failed++
				if failed <= 5 {
					t.Logf("[%s FAIL] %s: %s", category, filepath.Base(file), diff)
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
