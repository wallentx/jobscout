package tuiapp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/updatecheck"
)

func TestHandleUpdateCheckMsgShowsNoticeForAvailableUpdate(t *testing.T) {
	m := model{}

	updated, _ := m.Update(updateCheckMsg{
		result: updatecheck.Result{
			CurrentVersion: "v1.2.3-abcdef0",
			LatestVersion:  "v1.3.0",
			ReleaseURL:     "https://github.com/wallentx/jobscout/releases/tag/v1.3.0",
			Available:      true,
		},
	})
	got := updated.(model)

	if got.overlay.kind != overlayNotice {
		t.Fatalf("overlay.kind = %v; want overlayNotice", got.overlay.kind)
	}
	if !strings.Contains(got.overlay.notice.message, "v1.3.0") {
		t.Fatalf("notice message = %q; want latest version", got.overlay.notice.message)
	}
}

func TestHandleUpdateCheckMsgDoesNotInterruptExistingOverlay(t *testing.T) {
	m := model{overlay: overlayState{kind: overlaySetup}}

	updated, _ := m.Update(updateCheckMsg{
		result: updatecheck.Result{
			CurrentVersion: "v1.2.3-abcdef0",
			LatestVersion:  "v1.3.0",
			ReleaseURL:     "https://github.com/wallentx/jobscout/releases/tag/v1.3.0",
			Available:      true,
		},
	})
	got := updated.(model)

	if got.overlay.kind != overlaySetup {
		t.Fatalf("overlay.kind = %v; want overlaySetup", got.overlay.kind)
	}
}

func TestCheckForUpdateCmdReturnsMessage(t *testing.T) {
	cmd := checkForUpdateCmd(context.Background(), "v1.2.3-abcdef0", func(ctx context.Context, version string) (updatecheck.Result, error) {
		if version != "v1.2.3-abcdef0" {
			t.Fatalf("version = %q; want v1.2.3-abcdef0", version)
		}
		return updatecheck.Result{CurrentVersion: version, LatestVersion: "v1.3.0", Available: true}, nil
	})
	if cmd == nil {
		t.Fatal("checkForUpdateCmd() = nil; want command")
	}

	msg := cmd()
	if _, ok := msg.(updateCheckMsg); !ok {
		t.Fatalf("command message = %T; want updateCheckMsg", msg)
	}
}

func TestCheckForUpdateCmdPropagatesErrorInMessage(t *testing.T) {
	wantErr := errors.New("boom")
	cmd := checkForUpdateCmd(context.Background(), "v1.2.3", func(ctx context.Context, version string) (updatecheck.Result, error) {
		return updatecheck.Result{}, wantErr
	})

	msg := cmd().(updateCheckMsg)
	if !errors.Is(msg.err, wantErr) {
		t.Fatalf("message error = %v; want %v", msg.err, wantErr)
	}
}
