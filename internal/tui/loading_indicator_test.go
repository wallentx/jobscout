package tui

import "testing"

func TestLoadingBeamFramePausesAfterSweep(t *testing.T) {
	travelFrames := loadingBeamTravelFrames(7)

	center, paused := loadingBeamFrame(0, 7)
	if paused {
		t.Fatal("loadingBeamFrame(0, 7) paused = true; want false")
	}
	if center != -loadingBeamRadius {
		t.Fatalf("loadingBeamFrame(0, 7) center = %d; want %d", center, -loadingBeamRadius)
	}

	_, paused = loadingBeamFrame(travelFrames, 7)
	if !paused {
		t.Fatal("loadingBeamFrame(travelFrames, 7) paused = false; want true")
	}

	center, paused = loadingBeamFrame(travelFrames+loadingBeamPauseFrames, 7)
	if paused {
		t.Fatal("loadingBeamFrame(travelFrames+pause, 7) paused = true; want false")
	}
	if center != -loadingBeamRadius {
		t.Fatalf("loadingBeamFrame(restart, 7) center = %d; want %d", center, -loadingBeamRadius)
	}
}

func TestRenderLoadingBannerPausesBeforeRestart(t *testing.T) {
	text := normalizeLoadingLabel("loading")
	travelFrames := loadingBeamTravelFrames(len([]rune(text)))

	pausedA := renderLoadingBanner(text, travelFrames, 20)
	pausedB := renderLoadingBanner(text, travelFrames+1, 20)
	restart := renderLoadingBanner(text, travelFrames+loadingBeamPauseFrames, 20)

	if pausedA != pausedB {
		t.Fatal("pause frames rendered differently; want stable paused output")
	}
	if restart != renderLoadingBanner(text, 0, 20) {
		t.Fatal("restart frame does not match initial frame")
	}
}
