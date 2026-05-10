package tuiapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/wallentx/jobscout/internal/updatecheck"

	tea "github.com/charmbracelet/bubbletea"
)

const updateCheckDisabledEnv = "JOBSCOUT_DISABLE_UPDATE_CHECK"

type updateCheckMsg struct {
	result updatecheck.Result
	err    error
}

func maybeCheckForUpdateCmd(ctx context.Context, currentVersion string) tea.Cmd {
	if updateCheckDisabled() {
		return nil
	}
	checker := updateCheckerFunc(runtimeUpdateChecker)
	return checkForUpdateCmd(ctx, currentVersion, checker)
}

func checkForUpdateCmd(ctx context.Context, currentVersion string, checker updateCheckerFunc) tea.Cmd {
	currentVersion = strings.TrimSpace(currentVersion)
	if currentVersion == "" {
		return nil
	}
	if checker == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return func() tea.Msg {
		result, err := checker(ctx, currentVersion)
		return updateCheckMsg{result: result, err: err}
	}
}

func (m model) handleUpdateCheckMsg(msg updateCheckMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		if !errors.Is(msg.err, updatecheck.ErrVersionNotComparable) {
			logDebug("Update check failed: %v", msg.err)
		}
		return m, nil
	}
	if !msg.result.Available || m.quitting || m.overlay.kind != overlayNone {
		return m, nil
	}

	m.showNotice("Update Available", updateNoticeMessage(msg.result), false)
	return m, nil
}

func updateNoticeMessage(result updatecheck.Result) string {
	return fmt.Sprintf(
		"A newer jobscout release is available.\n\nCurrent: %s\nLatest:  %s\n\n%s",
		strings.TrimSpace(result.CurrentVersion),
		strings.TrimSpace(result.LatestVersion),
		strings.TrimSpace(result.ReleaseURL),
	)
}

func updateCheckDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(updateCheckDisabledEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
