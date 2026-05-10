package tui

import (
	"strings"
	"testing"
)

func TestRenderStockGraphEmptyPrices(t *testing.T) {
	if got := RenderStockGraph(nil, DefaultStockGraphWidth, DefaultStockGraphHeight); got != "" {
		t.Fatalf("RenderStockGraph(nil, width, height) = %q, want empty string", got)
	}
}

func TestRenderStockGraphIncludesAxisAndChange(t *testing.T) {
	graph := RenderStockGraph([]float64{100, 102, 101, 105}, 12, 4)

	for _, want := range []string{"105.00", "100.00", "+5.0%"} {
		if !strings.Contains(graph, want) {
			t.Fatalf("RenderStockGraph(...) = %q, want to contain %q", graph, want)
		}
	}
}
