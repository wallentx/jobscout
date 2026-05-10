package tuiapp

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
)

type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayNotice
	overlayStatus
	overlayFilter
	overlayHealth
	overlayDetail
	overlaySetup
)

type noticeState struct {
	visible      bool
	busy         bool
	minimized    bool
	title        string
	message      string
	scrollOffset int
}

type filterOverlayState struct {
	idx    int
	values map[string]bool
	saved  bool
}

type healthOverlayState struct {
	report       *CompanyHealthResult
	loading      bool
	minimized    bool
	loadingText  string
	err          string
	scrollOffset int
}

type detailOverlayState struct {
	scrollOffset int
}

type singleHealthTaskState struct {
	job     Job
	company string
}

type backgroundHealthState struct {
	tasks     map[string]singleHealthTaskState
	selected  int
	updated   int
	skipped   int
	failed    int
	last      string
	expanded  bool
	animating bool
	progress  float64
}

type backgroundTaskState struct {
	active        bool
	expanded      bool
	animating     bool
	animProgress  float64 // 0.0: minimized (top-left), 1.0: expanded (centered)
	id            int
	title         string
	progress      string
	pendingFields map[string]map[string]bool
}

type overlayState struct {
	kind      overlayKind
	notice    noticeState
	statusIdx int
	filter    filterOverlayState
	health    healthOverlayState
	detail    detailOverlayState
}

type activeFetchState struct {
	expanded     bool
	animating    bool
	animProgress float64
	title        string
	progress     string
}

type model struct {
	taskCtx             context.Context
	cancelTasks         context.CancelFunc
	allJobs             []Job
	filteredJobs        []Job
	healthCache         HealthCache
	setupRequired       bool
	setupIssues         []string
	sessionLLMDisabled  bool
	quitting            bool
	sortBy              int  // 0: Health, 1: Company, 2: Title, 3: Status, 4: Date
	sortDesc            bool // toggle ascending/descending
	activeFilters       map[string]bool
	setup               setupState
	overlay             overlayState
	bulkHealthFetching  bool
	bulkHealthCompanies []string
	bulkHealthJobs      []Job
	bulkHealthIdx       int
	bulkHealthUpdated   int
	bulkHealthCompleted int
	bulkHealthFailed    int
	bulkHealthSkipped   int
	bulkHealthInFlight  int
	fetchingJobs        bool
	fetchProgress       <-chan string
	activeFetch         activeFetchState
	pendingFetch        *ReviewSession
	backgroundTask      backgroundTaskState
	backgroundHealth    backgroundHealthState
	nextBackgroundTask  int
	termWidth           int
	termHeight          int
	loading             loadingIndicatorState

	// Manual Table State
	cursor      int
	yOffset     int
	tableHeight int

	// Search/Filter State
	textInput   textinput.Model
	isFiltering bool
	searchQuery string
}
