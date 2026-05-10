package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderPopupMenuViewportKeepsSelectedItemVisible(t *testing.T) {
	items := make([]PopupMenuItem, 0, 12)
	for i := 0; i < 12; i++ {
		items = append(items, PopupMenuItem{
			Label:    "model-" + string(rune('a'+i)),
			Selected: i == 11,
		})
	}

	viewport := RenderPopupMenuViewport(items, 32, 4, 11)
	stripped := ansi.Strip(viewport.Content)

	if !strings.Contains(stripped, "model-l") {
		t.Fatalf("viewport.Content = %q; want selected model-l visible", stripped)
	}
	if strings.Contains(stripped, "model-a") {
		t.Fatalf("viewport.Content = %q; did not want first item visible when selected item is near end", stripped)
	}
	if !strings.Contains(viewport.Content, "█") {
		t.Fatalf("viewport.Content = %q; want scrollbar thumb", viewport.Content)
	}
}

func TestRenderPopupDropdownStaysBoundedAndScrollable(t *testing.T) {
	items := []string{
		"gemini-flash-lite-latest",
		"gemini-pro-latest",
		"gemini-2.5-flash-lite",
		"gemini-2.5-flash",
		"gemini-2.0-flash",
		"gemini-3.1-pro-preview",
	}

	spec := PopupDropdownSpec{
		Label:       "LLM features:",
		Value:       "gemini-flash-lite-latest",
		Items:       items,
		LabelWidth:  22,
		Width:       36,
		MaxOpenRows: 4,
		SelectedIdx: 5,
		Open:        true,
		Focused:     true,
	}
	rendered := RenderPopupDropdown(spec)
	stripped := ansi.Strip(rendered)

	if !strings.Contains(stripped, "LLM features:") {
		t.Fatalf("RenderPopupDropdown(...) = %q; want label", stripped)
	}
	if strings.Contains(stripped, "gemini-3.1-pro-preview") {
		t.Fatalf("RenderPopupDropdown(...) = %q; did not want open panel inline", stripped)
	}

	overlay := RenderPopupDropdownOverlay(spec)
	overlayStripped := ansi.Strip(overlay.Content)
	if !strings.Contains(overlayStripped, "gemini-3.1-pro-preview") {
		t.Fatalf("RenderPopupDropdownOverlay(...) = %q; want selected item visible", overlayStripped)
	}
	if strings.Contains(overlayStripped, "gemini-pro-latest") {
		t.Fatalf("RenderPopupDropdownOverlay(...) = %q; did not want early item visible after scroll", overlayStripped)
	}
	if !strings.Contains(overlay.Content, "█") {
		t.Fatalf("RenderPopupDropdownOverlay(...) = %q; want scrollbar thumb", overlay.Content)
	}
	if !strings.Contains(overlayStripped, "↑") {
		t.Fatalf("RenderPopupDropdownOverlay(...) = %q; want scroll marker", overlayStripped)
	}
}
