package tuiapp

type fetchJobsMsg struct {
	jobs    []Job
	summary FetchSummary
	err     error
}

type fetchJobsProgressMsg struct {
	text string
	ch   <-chan string
	done bool
}

type acceptedFetchSavedMsg struct {
	jobs  []Job
	saved int
	err   error
}

type acceptedFetchEnrichedMsg struct {
	taskID int
	jobs   []Job
	err    error
}

type backgroundTaskProgressMsg struct {
	taskID int
	text   string
	ch     <-chan string
	jobCh  <-chan Job
	done   bool
}

type backgroundJobEnrichedMsg struct {
	taskID int
	job    Job
	ch     <-chan string
	jobCh  <-chan Job
}

type backgroundTaskAnimMsg struct {
	target float64
}
