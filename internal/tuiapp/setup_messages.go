package tuiapp

type setupPromptEditedMsg struct {
	content string
	err     error
}

type setupPreviewMsg struct {
	jobs     []Job
	notices  []string
	rejected map[string][]string
	err      error
}

type setupModelsFetchedMsg struct {
	provider string
	models   []string
	err      error
}

type setupResumeCriteriaMsg struct {
	path     string
	criteria *CriteriaConfig
	err      error
}

type setupCriteriaChoiceOption struct {
	Value string
	Label string
}
