package layout

import (
	"testing"
)

func TestPositionAbsolute(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(400),
		Height:        Px(300),
		Padding:       EdgesPx(10, 10, 10, 10),
	})

	inFlow := tree.NewNode(Style{
		Width:  Px(100),
		Height: Px(100),
	})
	tree.AddChild(root, inFlow)

	// Absolute child pinned to top-right
	absTopRight := tree.NewNode(Style{
		Position: PositionAbsolute,
		Inset: Edges{
			Top:   Px(5),
			Right: Px(15),
		},
		Width:  Px(50),
		Height: Px(40),
	})
	tree.AddChild(root, absTopRight)

	// Absolute child stretched across bottom
	absBottomStretch := tree.NewNode(Style{
		Position: PositionAbsolute,
		Inset: Edges{
			Left:   Px(20),
			Right:  Px(20),
			Bottom: Px(5),
		},
		Height: Px(30),
	})
	tree.AddChild(root, absBottomStretch)

	tree.Compute(400, 300)

	// inFlow is at padding left=10, top=10
	if r := tree.ComputedRect(inFlow); r.X != 10 || r.Y != 10 || r.Width != 100 || r.Height != 100 {
		t.Fatalf("inFlow rect unexpected: %+v", r)
	}

	// Content box of root is 380 x 280 (padding 10 each side)
	// absTopRight: X = padLeft + contentWidth - right - width = 10 + 380 - 15 - 50 = 325
	// Y = padTop + top = 10 + 5 = 15
	if r := tree.ComputedRect(absTopRight); r.X != 325 || r.Y != 15 || r.Width != 50 || r.Height != 40 {
		t.Fatalf("absTopRight rect unexpected: %+v", r)
	}

	// absBottomStretch:
	// X = padLeft + left = 10 + 20 = 30
	// Width = contentWidth - left - right = 380 - 20 - 20 = 340
	// Y = padTop + contentHeight - bottom - height = 10 + 280 - 5 - 30 = 255
	if r := tree.ComputedRect(absBottomStretch); r.X != 30 || r.Y != 255 || r.Width != 340 || r.Height != 30 {
		t.Fatalf("absBottomStretch rect unexpected: %+v", r)
	}
}

func TestPositionRelative(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(300),
		Height:        Px(100),
	})

	item1 := tree.NewNode(Style{
		Width:  Px(100),
		Height: Px(50),
	})
	tree.AddChild(root, item1)

	// item2 is relative, shifted by top: 10, left: 15
	item2 := tree.NewNode(Style{
		Position: PositionRelative,
		Inset: Edges{
			Top:  Px(10),
			Left: Px(15),
		},
		Width:  Px(100),
		Height: Px(50),
	})
	tree.AddChild(root, item2)

	tree.Compute(300, 100)

	if r := tree.ComputedRect(item1); r.X != 0 || r.Y != 0 {
		t.Fatalf("item1 unexpected: %+v", r)
	}
	// Normal X would be 100, shifted by 15 -> 115; Y shifted by 10 -> 10
	if r := tree.ComputedRect(item2); r.X != 115 || r.Y != 10 || r.Width != 100 || r.Height != 50 {
		t.Fatalf("item2 unexpected: %+v", r)
	}
}

func TestBaselineAlignment(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		AlignItems:    AlignBaseline,
		Width:         Px(400),
		Height:        Px(200),
	})

	// item1: height 60, baseline at 40
	item1 := tree.NewNode(Style{
		Width:  Px(80),
		Height: Px(60),
	})
	tree.SetBaseline(item1, 40)
	tree.AddChild(root, item1)

	// item2: height 100, baseline at 80
	item2 := tree.NewNode(Style{
		Width:  Px(80),
		Height: Px(100),
	})
	tree.SetBaseline(item2, 80)
	tree.AddChild(root, item2)

	tree.Compute(400, 200)

	// maxBaseline is 80 (from item2)
	// item1 crossOffset = maxBaseline - baseline = 80 - 40 = 40
	// item2 crossOffset = maxBaseline - baseline = 80 - 80 = 0
	r1 := tree.ComputedRect(item1)
	r2 := tree.ComputedRect(item2)

	if r1.Y != 40 {
		t.Fatalf("item1 Y expected 40, got %v", r1.Y)
	}
	if r2.Y != 0 {
		t.Fatalf("item2 Y expected 0, got %v", r2.Y)
	}
	// Their baselines: r1.Y + 40 = 80; r2.Y + 80 = 80! Perfect alignment!
}

func TestMeasureFunc(t *testing.T) {
	tree := NewTree()

	root := tree.NewNode(Style{
		Display:       DisplayFlex,
		FlexDirection: FlexDirectionRow,
		Width:         Px(500),
		Height:        Px(200),
	})

	// Text block node with Auto width and height and AlignSelfFlexStart
	textNode := tree.NewNode(Style{
		FlexGrow:  0,
		AlignSelf: AlignSelfFlexStart,
	})
	tree.SetMeasureFunc(textNode, func(tr *Tree, n NodeID, w float32, wm MeasureMode, h float32, hm MeasureMode) (float32, float32) {
		// Simulates text that is 142px wide and 24px high
		return 142, 24
	})
	tree.AddChild(root, textNode)

	tree.Compute(500, 200)

	r := tree.ComputedRect(textNode)
	if r.Width != 142 || r.Height != 24 {
		t.Fatalf("measured textNode expected 142x24, got %vx%v", r.Width, r.Height)
	}
}
