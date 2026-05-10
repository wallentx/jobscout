package tuiapp

import (
	"errors"
	"fmt"
	"strings"
	"time"

	healthpkg "github.com/wallentx/jobscout/internal/health"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleHealthLoadedMsg(msg healthLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.taskKey != "" {
		m.finishSingleHealthTask(msg)
	}

	if msg.result != nil {
		cacheKey := healthLoadedCacheKey(msg)
		if cacheKey != "" {
			m.healthCache[cacheKey] = HealthCacheEntry{
				Result:    msg.result,
				Timestamp: healthLoadedFetchedAt(msg),
			}
		}
	}

	if msg.background || (m.overlay.kind == overlayHealth && m.overlay.health.minimized) {
		if !m.singleHealthTasksActive() && m.overlay.kind == overlayHealth && m.overlay.health.minimized {
			m.clearOverlay()
		}
		m.applyFilterAndSort()
		return m, nil
	}

	m.overlay.health.loading = false
	m.overlay.health.minimized = false
	m.overlay.health.loadingText = ""
	m.overlay.health.scrollOffset = 0
	if msg.err != nil {
		m.overlay.health.err = msg.err.Error()
		if strings.HasPrefix(strings.ToLower(m.overlay.health.err), "imported ") {
			// Reload
			if jobs, err := loadRuntimeJobs(); err == nil {
				m.allJobs = jobs
				m.applyFilterAndSort()
			}
			// Clear error after a moment? Or show it.
			// We'll show it in the health view if triggered, but for import
			// we might want a separate status message field.
			// For now, let's just print it to debug and rely on the list update.
			logDebug(m.overlay.health.err)
			// Clear it so it doesn't block health view
			m.overlay.health.err = ""
		} else {
			m.overlay.kind = overlayHealth
		}
	} else {
		m.overlay.kind = overlayHealth
		m.overlay.health.report = msg.result
		m.overlay.health.err = ""
	}
	return m, nil
}

func (m model) handleBulkHealthStepMsg(msg bulkHealthStepMsg) (tea.Model, tea.Cmd) {
	if !m.bulkHealthFetching || len(m.bulkHealthCompanies) == 0 {
		logBulkHealthDebug("step ignored company=%q fetching=%t companies=%d", msg.company, m.bulkHealthFetching, len(m.bulkHealthCompanies))
		return m, nil
	}
	if m.bulkHealthInFlight > 0 {
		m.bulkHealthInFlight--
	}
	m.bulkHealthCompleted++
	logBulkHealthDebug(
		"step received company=%q completed=%d/%d in_flight=%d elapsed=%s err=%v mem=%s",
		msg.company,
		m.bulkHealthCompleted,
		len(m.bulkHealthCompanies),
		m.bulkHealthInFlight,
		msg.elapsed.Round(time.Millisecond),
		msg.err,
		msg.mem,
	)

	if msg.err == nil && msg.result != nil {
		cacheKey := healthpkg.CacheKeyForIdentity(CompanyHealthContext{
			Company:  msg.result.Company,
			Website:  healthpkg.SourceStringFromMap(msg.result.Sources, "company_identity", "website"),
			Summary:  healthpkg.SourceStringFromMap(msg.result.Sources, "company_identity", "summary"),
			Industry: healthpkg.SourceStringFromMap(msg.result.Sources, "company_identity", "industry"),
		})
		if cacheKey == "" {
			cacheKey = msg.company
		}
		healthpkg.SetCachedHealth(m.healthCache, cacheKey, msg.result)
		_ = runtimeHealthStore.SetHealth(cacheKey, msg.result, time.Now())
		m.bulkHealthUpdated++
	} else if errors.Is(msg.err, errCompanyHealthIdentityUnresolved) {
		m.bulkHealthSkipped++
	} else {
		m.bulkHealthFailed++
	}

	total := len(m.bulkHealthCompanies)
	if m.bulkHealthCompleted >= total {
		failed := m.bulkHealthFailed
		skipped := m.bulkHealthSkipped
		updated := m.bulkHealthUpdated
		m.bulkHealthFetching = false
		m.bulkHealthCompanies = nil
		m.bulkHealthJobs = nil
		m.bulkHealthIdx = 0
		m.bulkHealthCompleted = 0
		m.bulkHealthFailed = 0
		m.bulkHealthSkipped = 0
		m.bulkHealthInFlight = 0
		m.applyFilterAndSort()
		m.backgroundTask.active = false
		m.backgroundTask.expanded = false
		m.backgroundTask.animating = false
		m.backgroundTask.progress = ""
		m.showNotice("Health Refresh Complete", fmt.Sprintf("Updated health data for %d companies. Skipped: %d. Failed: %d.", updated, skipped, failed), false)
		m.bulkHealthUpdated = 0
		logBulkHealthDebug("complete total=%d updated=%d skipped=%d failed=%d mem=%s", total, updated, skipped, failed, currentBulkHealthMemStats())
		return m, nil
	}

	cmd := m.scheduleBulkHealthCommands()
	m.backgroundTask.active = true
	m.backgroundTask.title = "Refreshing Health Data"
	m.backgroundTask.progress = bulkHealthProgressMessage(m.bulkHealthCompleted, total, m.bulkHealthUpdated, m.bulkHealthSkipped, m.bulkHealthFailed, m.bulkHealthInFlight, msg.company)
	return m, cmd
}
