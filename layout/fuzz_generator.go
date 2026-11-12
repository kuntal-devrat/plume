package layout

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os/exec"
)

// FuzzTreeDef holds serializable tree definitions for fuzzing and differential verification.
type FuzzTreeDef struct {
	ViewportW float32       `json:"viewport_w"`
	ViewportH float32       `json:"viewport_h"`
	Nodes     []FuzzNodeDef `json:"nodes"`
}

type FuzzNodeDef struct {
	ID               int32   `json:"id"`
	Parent           int32   `json:"parent"`
	Display          string  `json:"display"`
	Direction        string  `json:"direction"`
	FlexWrap         string  `json:"flex_wrap"`
	JustifyContent   string  `json:"justify_content"`
	AlignItems       string  `json:"align_items"`
	AlignSelf        string  `json:"align_self"`
	FlexGrow         float32 `json:"flex_grow"`
	FlexShrink       float32 `json:"flex_shrink"`
	WidthKind        string  `json:"width_kind"`
	WidthVal         float32 `json:"width_val"`
	HeightKind       string  `json:"height_kind"`
	HeightVal        float32 `json:"height_val"`
	MinWidthKind     string  `json:"min_width_kind"`
	MinWidthVal      float32 `json:"min_width_val"`
	MinHeightKind    string  `json:"min_height_kind"`
	MinHeightVal     float32 `json:"min_height_val"`
	MaxWidthKind     string  `json:"max_width_kind"`
	MaxWidthVal      float32 `json:"max_width_val"`
	MaxHeightKind    string  `json:"max_height_kind"`
	MaxHeightVal     float32 `json:"max_height_val"`
	AspectRatio      float32 `json:"aspect_ratio"`
	MarginLeftKind   string  `json:"margin_left_kind"`
	MarginLeftVal    float32 `json:"margin_left_val"`
	MarginRightKind  string  `json:"margin_right_kind"`
	MarginRightVal   float32 `json:"margin_right_val"`
	MarginTopKind    string  `json:"margin_top_kind"`
	MarginTopVal     float32 `json:"margin_top_val"`
	MarginBottomKind string  `json:"margin_bottom_kind"`
	MarginBottomVal  float32 `json:"margin_bottom_val"`
	PaddingLeft      float32 `json:"padding_left"`
	PaddingRight     float32 `json:"padding_right"`
	PaddingTop       float32 `json:"padding_top"`
	PaddingBottom    float32 `json:"padding_bottom"`
	Gap              float32 `json:"gap"`
	RowGap           float32 `json:"row_gap"`
	ColumnGap        float32 `json:"column_gap"`
}

type FuzzTaffyOutput struct {
	ID int32   `json:"id"`
	X  float32 `json:"x"`
	Y  float32 `json:"y"`
	W  float32 `json:"w"`
	H  float32 `json:"h"`
}

// GenerateTreeFromBytes deterministic random layout tree generator from fuzz byte slice.
func GenerateTreeFromBytes(data []byte) (*FuzzTreeDef, *Tree, bool) {
	if len(data) < 16 {
		return nil, nil, false
	}

	r := bytes.NewReader(data)
	readU8 := func() uint8 {
		var b uint8
		_ = binary.Read(r, binary.LittleEndian, &b)
		return b
	}
	readU16 := func() uint16 {
		var v uint16
		_ = binary.Read(r, binary.LittleEndian, &v)
		return v
	}

	vw := float32(100 + (readU16() % 1800))
	vh := float32(100 + (readU16() % 1200))

	numNodes := int(1 + (readU8() % 8)) // 1 to 8 nodes

	treeDef := &FuzzTreeDef{
		ViewportW: vw,
		ViewportH: vh,
	}

	tree := NewTree()

	for i := 0; i < numNodes; i++ {
		var parent int32 = -1
		if i > 0 {
			parent = int32(readU8() % uint8(i))
		}

		dispIdx := readU8() % 3
		dispStr := "flex"
		display := DisplayFlex
		if dispIdx == 1 {
			dispStr = "grid"
			display = DisplayGrid
		} else if dispIdx == 2 {
			dispStr = "block"
			display = DisplayBlock
		}

		dirIdx := readU8() % 4
		dirStrs := []string{"row", "column", "row_reverse", "column_reverse"}
		flexDir := FlexDirection(dirIdx)

		wrapIdx := readU8() % 3
		wrapStrs := []string{"nowrap", "wrap", "wrap_reverse"}
		flexWrap := FlexWrap(wrapIdx)

		jcIdx := readU8() % 6
		jcStrs := []string{"start", "end", "center", "space_between", "space_around", "space_evenly"}
		jc := JustifyContent(jcIdx)

		aiIdx := readU8() % 4
		aiStrs := []string{"stretch", "start", "end", "center"}
		ai := AlignItems(aiIdx)

		asIdx := readU8() % 5
		asStrs := []string{"auto", "stretch", "start", "end", "center"}
		as := AlignSelf(asIdx)

		grow := float32(readU8()%4) * 0.5
		shrink := float32(readU8()%3) * 1.0

		// Sizing
		parseDimFromByte := func(b uint8) (string, Dimension) {
			mode := b % 3
			val := float32(20 + ((b >> 2) % 200))
			switch mode {
			case 0:
				return "auto", Auto()
			case 1:
				return "px", Px(val)
			case 2:
				pct := float32(10 + (val / 2))
				if pct > 100 {
					pct = 100
				}
				return "percent", Percent(pct)
			}
			return "auto", Auto()
		}

		wKind, wDim := parseDimFromByte(readU8())
		hKind, hDim := parseDimFromByte(readU8())
		minWKind, minWDim := parseDimFromByte(readU8())
		minHKind, minHDim := parseDimFromByte(readU8())
		maxWKind, maxWDim := parseDimFromByte(readU8())
		maxHKind, maxHDim := parseDimFromByte(readU8())

		// Aspect Ratio
		var ar float32 = 0
		if readU8()%4 == 0 {
			ar = 16.0 / 9.0
		}

		// Margins
		parseMarginFromByte := func(b uint8) (string, Dimension) {
			mode := b % 5
			if mode == 0 {
				return "auto", Auto()
			}
			val := float32((b >> 3) % 40)
			return "px", Px(val)
		}

		mlKind, mlDim := parseMarginFromByte(readU8())
		mrKind, mrDim := parseMarginFromByte(readU8())
		mtKind, mtDim := parseMarginFromByte(readU8())
		mbKind, mbDim := parseMarginFromByte(readU8())

		pL := float32(readU8() % 20)
		pR := float32(readU8() % 20)
		pT := float32(readU8() % 20)
		pB := float32(readU8() % 20)

		gap := float32(readU8() % 30)

		s := Style{
			Display:        display,
			FlexDirection:  flexDir,
			FlexWrap:       flexWrap,
			JustifyContent: jc,
			AlignItems:     ai,
			AlignSelf:      as,
			FlexGrow:       grow,
			FlexShrink:     shrink,
			Width:          wDim,
			Height:         hDim,
			MinWidth:       minWDim,
			MinHeight:      minHDim,
			MaxWidth:       maxWDim,
			MaxHeight:      maxHDim,
			AspectRatio:    ar,
			Margin: Edges{
				Left:   mlDim,
				Right:  mrDim,
				Top:    mtDim,
				Bottom: mbDim,
			},
			Padding: Edges{
				Left:   Px(pL),
				Right:  Px(pR),
				Top:    Px(pT),
				Bottom: Px(pB),
			},
			Gap: Px(gap),
		}

		nodeID := tree.NewNode(s)
		if parent >= 0 {
			tree.AddChild(NodeID(parent), nodeID)
		}

		treeDef.Nodes = append(treeDef.Nodes, FuzzNodeDef{
			ID:               int32(nodeID),
			Parent:           parent,
			Display:          dispStr,
			Direction:        dirStrs[dirIdx],
			FlexWrap:         wrapStrs[wrapIdx],
			JustifyContent:   jcStrs[jcIdx],
			AlignItems:       aiStrs[aiIdx],
			AlignSelf:        asStrs[asIdx],
			FlexGrow:         grow,
			FlexShrink:       shrink,
			WidthKind:        wKind,
			WidthVal:         wDim.Value,
			HeightKind:       hKind,
			HeightVal:        hDim.Value,
			MinWidthKind:     minWKind,
			MinWidthVal:      minWDim.Value,
			MinHeightKind:    minHKind,
			MinHeightVal:     minHDim.Value,
			MaxWidthKind:     maxWKind,
			MaxWidthVal:      maxWDim.Value,
			MaxHeightKind:    maxHKind,
			MaxHeightVal:     maxHDim.Value,
			AspectRatio:      ar,
			MarginLeftKind:   mlKind,
			MarginLeftVal:    mlDim.Value,
			MarginRightKind:  mrKind,
			MarginRightVal:   mrDim.Value,
			MarginTopKind:    mtKind,
			MarginTopVal:     mtDim.Value,
			MarginBottomKind: mbKind,
			MarginBottomVal:  mbDim.Value,
			PaddingLeft:      pL,
			PaddingRight:     pR,
			PaddingTop:       pT,
			PaddingBottom:    pB,
			Gap:              gap,
		})
	}

	return treeDef, tree, true
}

// DifferentialCheckWithTaffy pipes treeDef into Rust Taffy binary and verifies matching rects.
func DifferentialCheckWithTaffy(taffyBin string, treeDef *FuzzTreeDef, tree *Tree) (bool, string) {
	jsonData, err := json.Marshal(treeDef)
	if err != nil {
		return false, err.Error()
	}

	cmd := exec.Command(taffyBin)
	cmd.Stdin = bytes.NewReader(jsonData)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return false, "taffy execution error: " + err.Error() + " stderr: " + errBuf.String()
	}

	var taffyOutputs []FuzzTaffyOutput
	if err := json.Unmarshal(outBuf.Bytes(), &taffyOutputs); err != nil {
		return false, "json unmarshal error: " + err.Error() + " output: " + outBuf.String()
	}

	for _, to := range taffyOutputs {
		nodeID := NodeID(to.ID)
		rect := tree.ComputedRect(nodeID)

		dx := math.Abs(float64(rect.X - to.X))
		dy := math.Abs(float64(rect.Y - to.Y))
		dw := math.Abs(float64(rect.Width - to.W))
		dh := math.Abs(float64(rect.Height - to.H))

		// Check tolerance (allow 1.0px for subpixel float discrepancies)
		if dx > 1.0 || dy > 1.0 || dw > 1.0 || dh > 1.0 {
			return false, "discrepancy on node " + string(rune('0'+to.ID))
		}
	}

	return true, ""
}
