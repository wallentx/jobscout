package tui

import (
	"fmt"
	"math"
	"strings"
)

const (
	DefaultStockGraphHeight = 7
	DefaultStockGraphWidth  = 30
)

// RenderStockGraph creates a Braille-based sparkline chart.
func RenderStockGraph(prices []float64, width, height int) string {
	if len(prices) == 0 {
		return ""
	}

	// Canvas dimensions (Braille dots)
	// Each Braille character is 2 dots wide by 4 dots high
	canvasWidth := width * 2
	canvasHeight := height * 4

	// Downsample prices to match canvas width
	sampled := samplePrices(prices, canvasWidth)
	if len(sampled) < 2 {
		return ""
	}

	minPrice, maxPrice := getMinMax(sampled)
	if maxPrice == minPrice {
		maxPrice = minPrice + 1
	}

	// Initialize canvas
	canvas := make([][]bool, canvasHeight)
	for i := range canvas {
		canvas[i] = make([]bool, canvasWidth)
	}

	// Plot lines
	for x := range len(sampled) - 1 {
		y1 := normalize(sampled[x], minPrice, maxPrice, canvasHeight)
		y2 := normalize(sampled[x+1], minPrice, maxPrice, canvasHeight)
		drawLine(canvas, x, y1, x+1, y2)
	}

	// Convert canvas to Braille string
	var result strings.Builder

	// Top Label
	fmt.Fprintf(&result, "%8.2f ┤", maxPrice)

	// First row of characters
	result.WriteString(renderRow(canvas, 0))
	result.WriteString("\n")

	// Middle rows
	for r := 1; r < height-1; r++ {
		result.WriteString("         │")
		result.WriteString(renderRow(canvas, r*4))
		result.WriteString("\n")
	}

	// Bottom Label
	fmt.Fprintf(&result, "%8.2f ┤", minPrice)
	result.WriteString(renderRow(canvas, (height-1)*4))
	result.WriteString("\n")

	// X-Axis
	change := ((sampled[len(sampled)-1] - sampled[0]) / sampled[0]) * 100
	fmt.Fprintf(&result, "         └%s", strings.Repeat("─", width))
	fmt.Fprintf(&result, " %+.1f%%", change)

	return result.String()
}

func samplePrices(prices []float64, targetCount int) []float64 {
	if len(prices) <= targetCount {
		return prices
	}
	step := float64(len(prices)-1) / float64(targetCount-1)
	sampled := make([]float64, targetCount)
	for i := range targetCount {
		idx := int(math.Round(float64(i) * step))
		if idx >= len(prices) {
			idx = len(prices) - 1
		}
		sampled[i] = prices[idx]
	}
	return sampled
}

func getMinMax(prices []float64) (float64, float64) {
	min := prices[0]
	max := prices[0]
	for _, p := range prices {
		if p < min {
			min = p
		}
		if p > max {
			max = p
		}
	}
	return min, max
}

func normalize(val, min, max float64, height int) int {
	ratio := (val - min) / (max - min)
	y := int(math.Round(ratio * float64(height-1)))
	// Invert Y (0 is top in array, but max value in graph)
	return (height - 1) - y
}

func drawLine(canvas [][]bool, x1, y1, x2, y2 int) {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	steps := math.Max(math.Abs(dx), math.Abs(dy))

	if steps == 0 {
		if y1 >= 0 && y1 < len(canvas) && x1 >= 0 && x1 < len(canvas[0]) {
			canvas[y1][x1] = true
		}
		return
	}

	xInc := dx / steps
	yInc := dy / steps

	x := float64(x1)
	y := float64(y1)

	for range int(steps) + 1 {
		ix := int(math.Round(x))
		iy := int(math.Round(y))
		if iy >= 0 && iy < len(canvas) && ix >= 0 && ix < len(canvas[0]) {
			canvas[iy][ix] = true
		}
		x += xInc
		y += yInc
	}
}

func renderRow(canvas [][]bool, startY int) string {
	var row strings.Builder
	width := len(canvas[0]) / 2 // 2 dots per char width

	for i := range width {
		// Calculate Braille rune for the 2x4 block
		// Block layout (dots):
		// 1 4
		// 2 5
		// 3 6
		// 7 8

		// Map to standard Braille unicode offset 0x2800
		// Bit mapping:
		// 0x1 (1)   0x8 (4)
		// 0x2 (2)   0x10 (5)
		// 0x4 (3)   0x20 (6)
		// 0x40 (7)  0x80 (8)

		var charCode rune = 0x2800
		baseX := i * 2

		// Col 1
		if safeGet(canvas, startY+0, baseX) {
			charCode |= 0x1
		}
		if safeGet(canvas, startY+1, baseX) {
			charCode |= 0x2
		}
		if safeGet(canvas, startY+2, baseX) {
			charCode |= 0x4
		}
		if safeGet(canvas, startY+3, baseX) {
			charCode |= 0x40
		}

		// Col 2
		if safeGet(canvas, startY+0, baseX+1) {
			charCode |= 0x8
		}
		if safeGet(canvas, startY+1, baseX+1) {
			charCode |= 0x10
		}
		if safeGet(canvas, startY+2, baseX+1) {
			charCode |= 0x20
		}
		if safeGet(canvas, startY+3, baseX+1) {
			charCode |= 0x80
		}

		row.WriteRune(charCode)
	}
	return row.String()
}

func safeGet(canvas [][]bool, y, x int) bool {
	if y < 0 || y >= len(canvas) {
		return false
	}
	if x < 0 || x >= len(canvas[0]) {
		return false
	}
	return canvas[y][x]
}
