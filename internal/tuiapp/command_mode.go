package tuiapp

import (
	"fmt"
	"strings"
)

type operatorCommandResult struct {
	Title          string
	Message        string
	HealthIdentity CompanyHealthContext
	FetchOptions   *operatorFetchOptions
}

type operatorFetchOptions struct {
	Company string
	Aliases []string
	Website string
	All     bool
}

type operatorCommandSpec struct {
	Name        string
	Aliases     []string
	Execute     func([]string) (operatorCommandResult, error)
	Completions []operatorCommandCompletionSpec
}

type operatorCommandCompletionSpec struct {
	Label  string
	Insert string
}

type operatorCommandCompletion struct {
	Label string
	Value string
}

var operatorCommandSpecs = []operatorCommandSpec{
	{
		Name:    "debug",
		Execute: executeDebugCommand,
		Completions: []operatorCommandCompletionSpec{
			{Label: "status", Insert: "status"},
			{Label: "on", Insert: "on"},
			{Label: "off", Insert: "off"},
			{Label: "path <file>", Insert: "path "},
		},
	},
	{
		Name:    "health",
		Execute: executeHealthCommand,
	},
	{
		Name:    "fetch",
		Execute: executeFetchCommand,
	},
}

func executeOperatorCommand(input string) (operatorCommandResult, error) {
	args, err := parseOperatorCommandLine(input)
	if err != nil {
		return operatorCommandResult{}, err
	}
	if len(args) == 0 {
		return operatorCommandResult{}, fmt.Errorf("enter a command")
	}

	if spec, ok := findOperatorCommandSpec(args[0]); ok {
		return spec.Execute(args[1:])
	}
	return operatorCommandResult{}, fmt.Errorf("unknown command %q", args[0])
}

func findOperatorCommandSpec(name string) (operatorCommandSpec, bool) {
	name = strings.ToLower(name)
	for _, spec := range operatorCommandSpecs {
		if strings.EqualFold(spec.Name, name) {
			return spec, true
		}
		for _, alias := range spec.Aliases {
			if strings.EqualFold(alias, name) {
				return spec, true
			}
		}
	}
	return operatorCommandSpec{}, false
}

func operatorCommandCompletions(input string) []operatorCommandCompletion {
	input = strings.TrimLeft(input, " \t")
	command, rest, hasRest := splitOperatorCommandInput(input)
	if !hasRest {
		return operatorCommandNameCompletions(command)
	}

	spec, ok := findOperatorCommandSpec(command)
	if !ok {
		return nil
	}

	rest = strings.TrimLeft(rest, " \t")
	if spec.Name == "health" {
		return healthCommandCompletions(input)
	}
	if spec.Name == "fetch" {
		return fetchCommandCompletions(input)
	}

	var completions []operatorCommandCompletion
	for _, completion := range spec.Completions {
		insert := completion.Insert
		if matchOperatorCommandCompletion(rest, completion.Label, strings.TrimSpace(insert)) {
			completions = append(completions, operatorCommandCompletion{
				Label: completion.Label,
				Value: spec.Name + " " + insert,
			})
		}
	}
	return completions
}

func healthCommandCompletions(input string) []operatorCommandCompletion {
	input = strings.TrimLeft(input, " \t")
	command, rest, hasRest := splitOperatorCommandInput(input)
	if !hasRest || !strings.EqualFold(command, "health") {
		return nil
	}
	rest = strings.TrimLeft(rest, " \t")
	if strings.TrimSpace(rest) == "" || healthCommandAwaitingFlagValue(rest) || commandCurrentTokenRequiresValue(rest, healthFlagRequiresValue) {
		return nil
	}

	base, prefix, ok := commandFlagCompletionContext(rest)
	if !ok || !healthRestHasCompany(rest) {
		return nil
	}

	flags := []operatorCommandCompletionSpec{
		{Label: "--aka", Insert: "--aka"},
		{Label: "--website", Insert: "--website"},
	}
	usedFlags := healthCommandUsedFlags(rest)
	completions := make([]operatorCommandCompletion, 0, len(flags))
	for _, flag := range flags {
		if flag.Label == "--website" && usedFlags["--website"] {
			continue
		}
		if matchOperatorCommandCompletion(prefix, flag.Label, flag.Insert) {
			completions = append(completions, operatorCommandCompletion{
				Label: flag.Label,
				Value: "health " + base + flag.Insert,
			})
		}
	}
	return completions
}

func fetchCommandCompletions(input string) []operatorCommandCompletion {
	input = strings.TrimLeft(input, " \t")
	command, rest, hasRest := splitOperatorCommandInput(input)
	if !hasRest || !strings.EqualFold(command, "fetch") {
		return nil
	}
	rest = strings.TrimLeft(rest, " \t")
	if strings.TrimSpace(rest) == "" || fetchCommandAwaitingFlagValue(rest) || commandCurrentTokenRequiresValue(rest, fetchFlagRequiresValue) {
		return nil
	}

	base, prefix, ok := commandFlagCompletionContext(rest)
	if !ok || !fetchRestHasCompany(rest) {
		return nil
	}

	flags := []operatorCommandCompletionSpec{
		{Label: "--aka", Insert: "--aka"},
		{Label: "--website", Insert: "--website"},
		{Label: "--all", Insert: "--all"},
	}
	usedFlags := fetchCommandUsedFlags(rest)
	completions := make([]operatorCommandCompletion, 0, len(flags))
	for _, flag := range flags {
		if flag.Label != "--aka" && usedFlags[flag.Label] {
			continue
		}
		if matchOperatorCommandCompletion(prefix, flag.Label, flag.Insert) {
			completions = append(completions, operatorCommandCompletion{
				Label: flag.Label,
				Value: "fetch " + base + flag.Insert,
			})
		}
	}
	return completions
}

func commandFlagCompletionContext(rest string) (base string, prefix string, ok bool) {
	if strings.HasSuffix(rest, " ") || strings.HasSuffix(rest, "\t") {
		return rest, "", true
	}
	trimmedRight := strings.TrimRight(rest, " \t")
	tokenStart := strings.LastIndexAny(trimmedRight, " \t")
	if tokenStart == -1 {
		if strings.HasPrefix(trimmedRight, "-") {
			return "", trimmedRight, true
		}
		return "", "", false
	}
	current := trimmedRight[tokenStart+1:]
	if !strings.HasPrefix(current, "-") {
		return "", "", false
	}
	return trimmedRight[:tokenStart+1], current, true
}

func commandCurrentTokenRequiresValue(rest string, requiresValue func(string) bool) bool {
	if strings.HasSuffix(rest, " ") || strings.HasSuffix(rest, "\t") {
		return false
	}
	args, err := parseOperatorCommandLine(rest)
	if err != nil || len(args) == 0 {
		return false
	}
	last := args[len(args)-1]
	if strings.Contains(last, "=") {
		return false
	}
	return requiresValue(last)
}

func healthCommandAwaitingFlagValue(rest string) bool {
	return commandAwaitingFlagValue(rest, healthFlagRequiresValue)
}

func fetchCommandAwaitingFlagValue(rest string) bool {
	return commandAwaitingFlagValue(rest, fetchFlagRequiresValue)
}

func commandAwaitingFlagValue(rest string, requiresValue func(string) bool) bool {
	if !strings.HasSuffix(rest, " ") && !strings.HasSuffix(rest, "\t") {
		return false
	}
	args, err := parseOperatorCommandLine(rest)
	if err != nil || len(args) == 0 {
		return false
	}
	flag, value, hasValue := strings.Cut(args[len(args)-1], "=")
	if hasValue && strings.TrimSpace(value) != "" {
		return false
	}
	return requiresValue(flag)
}

func healthRestHasCompany(rest string) bool {
	args, err := parseOperatorCommandLine(rest)
	if err != nil {
		return false
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return false
		}
		if strings.TrimSpace(arg) != "" {
			return true
		}
	}
	return false
}

func fetchRestHasCompany(rest string) bool {
	args, err := parseOperatorCommandLine(rest)
	if err != nil {
		return false
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			return false
		}
		if strings.TrimSpace(arg) != "" {
			return true
		}
	}
	return false
}

func healthFlagRequiresValue(flag string) bool {
	flag, _, _ = strings.Cut(strings.TrimSpace(flag), "=")
	switch flag {
	case "--aka", "--website":
		return true
	default:
		return false
	}
}

func fetchFlagRequiresValue(flag string) bool {
	flag, _, _ = strings.Cut(strings.TrimSpace(flag), "=")
	switch flag {
	case "--aka", "--website":
		return true
	default:
		return false
	}
}

func healthCommandUsedFlags(rest string) map[string]bool {
	used := make(map[string]bool)
	args, err := parseOperatorCommandLine(rest)
	if err != nil {
		return used
	}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		flag, _, _ := strings.Cut(arg, "=")
		used[flag] = true
	}
	return used
}

func fetchCommandUsedFlags(rest string) map[string]bool {
	used := make(map[string]bool)
	args, err := parseOperatorCommandLine(rest)
	if err != nil {
		return used
	}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		flag, _, _ := strings.Cut(arg, "=")
		switch flag {
		case "--aka", "--website", "--all":
			used[flag] = true
		}
	}
	return used
}

func splitOperatorCommandInput(input string) (string, string, bool) {
	for i, r := range input {
		if isOperatorCommandSpace(r) {
			return input[:i], input[i+1:], true
		}
	}
	return input, "", false
}

func operatorCommandNameCompletions(prefix string) []operatorCommandCompletion {
	var completions []operatorCommandCompletion
	for _, spec := range operatorCommandSpecs {
		if fuzzyMatchOperatorCommand(prefix, spec.Name) {
			completions = append(completions, operatorCommandCompletion{
				Label: spec.Name,
				Value: spec.Name,
			})
		}
	}
	return completions
}

func fuzzyMatchOperatorCommand(pattern string, candidate string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	candidate = strings.ToLower(candidate)
	if pattern == "" {
		return true
	}
	next := 0
	for _, r := range candidate {
		if next >= len(pattern) {
			return true
		}
		if r == rune(pattern[next]) {
			next++
		}
	}
	return next >= len(pattern)
}

func matchOperatorCommandCompletion(pattern string, label string, insert string) bool {
	pattern = strings.TrimSpace(pattern)
	if strings.HasPrefix(pattern, "-") {
		needle := strings.ToLower(pattern)
		return strings.HasPrefix(strings.ToLower(label), needle) || strings.HasPrefix(strings.ToLower(insert), needle)
	}
	return fuzzyMatchOperatorCommand(pattern, label) || fuzzyMatchOperatorCommand(pattern, insert)
}

func (m *model) applyOperatorCommandCompletion() {
	query := m.commandTypedInput
	if query == "" && !m.commandCompletionActive {
		query = m.commandInput.Value()
		m.commandTypedInput = query
	}

	if !m.commandCompletionActive {
		m.commandCompletions = operatorCommandCompletions(query)
		m.commandCompletionIdx = 0
	} else if len(m.commandCompletions) > 0 {
		m.commandCompletionIdx = (m.commandCompletionIdx + 1) % len(m.commandCompletions)
	}

	if len(m.commandCompletions) == 0 {
		m.commandCompletionActive = false
		return
	}
	m.commandCompletionActive = true
	m.commandInput.SetValue(m.commandCompletions[m.commandCompletionIdx].Value)
	m.commandInput.CursorEnd()
}

func (m *model) confirmOperatorCommandCompletion() bool {
	if !m.commandCompletionActive || len(m.commandCompletions) == 0 {
		return false
	}
	selected := m.commandCompletions[m.commandCompletionIdx]
	value := selected.Value
	if !strings.HasSuffix(value, " ") {
		value += " "
	}
	m.commandInput.SetValue(value)
	m.commandInput.CursorEnd()
	m.commandTypedInput = value
	m.resetOperatorCommandCompletion()
	return true
}

func (m *model) resetOperatorCommandCompletion() {
	m.commandCompletions = nil
	m.commandCompletionIdx = 0
	m.commandCompletionActive = false
}

func (m *model) resetOperatorCommandPrompt() {
	m.commandInput.SetValue("")
	m.commandTypedInput = ""
	m.resetOperatorCommandCompletion()
}

func (m *model) clearOperatorCommandResult() {
	m.commandResultTitle = ""
	m.commandResultMessage = ""
	m.commandResultError = false
}

func (m *model) prepareOperatorCommandTyping() {
	if m.commandCompletionActive {
		m.commandTypedInput = m.commandInput.Value()
	}
	m.resetOperatorCommandCompletion()
}

func (m *model) updateOperatorCommandTypedInput() {
	m.commandTypedInput = m.commandInput.Value()
}

func (m model) operatorCommandQuery() string {
	if m.commandCompletionActive || m.commandTypedInput != "" || m.commandInput.Value() == "" {
		return m.commandTypedInput
	}
	return m.commandInput.Value()
}

func (m model) operatorCommandPool() []operatorCommandCompletion {
	if m.commandCompletionActive && len(m.commandCompletions) > 0 {
		return m.commandCompletions
	}
	return operatorCommandCompletions(m.operatorCommandQuery())
}

func (m model) operatorCommandGhostHint() string {
	return operatorCommandGhostHint(m.commandInput.Value())
}

func operatorCommandGhostHint(input string) string {
	if len(operatorCommandCompletions(input)) > 0 {
		return ""
	}
	input = strings.TrimLeft(input, " \t")
	command, rest, hasRest := splitOperatorCommandInput(input)
	if !hasRest {
		return ""
	}
	rest = strings.TrimLeft(rest, " \t")
	if strings.EqualFold(command, "fetch") {
		if strings.TrimSpace(rest) == "" {
			return "<company>"
		}
		if hint := fetchCommandValueHint(rest); hint != "" {
			return hint
		}
		return ""
	}
	if !strings.EqualFold(command, "health") {
		return ""
	}
	if strings.TrimSpace(rest) == "" {
		return "<company>"
	}
	if hint := healthCommandValueHint(rest); hint != "" {
		return hint
	}
	return ""
}

func healthCommandValueHint(rest string) string {
	return commandFlagValueHint(rest, healthFlagRequiresValue, map[string]string{
		"--aka":     "<name>",
		"--website": "<url>",
	})
}

func fetchCommandValueHint(rest string) string {
	return commandFlagValueHint(rest, fetchFlagRequiresValue, map[string]string{
		"--aka":     "<name>",
		"--website": "<url>",
	})
}

func commandFlagValueHint(rest string, requiresValue func(string) bool, hints map[string]string) string {
	args, err := parseOperatorCommandLine(rest)
	if err != nil || len(args) == 0 {
		return ""
	}
	last := args[len(args)-1]
	flag, value, hasValue := strings.Cut(last, "=")
	if hasValue && strings.TrimSpace(value) != "" {
		return ""
	}
	if !requiresValue(flag) {
		return ""
	}
	hint := hints[flag]
	if hint == "" {
		return ""
	}
	return commandValueHintText(rest, hint)
}

func commandValueHintText(rest string, hint string) string {
	if strings.HasSuffix(rest, " ") || strings.HasSuffix(rest, "\t") || strings.Contains(lastOperatorCommandToken(rest), "=") {
		return hint
	}
	return " " + hint
}

func lastOperatorCommandToken(input string) string {
	input = strings.TrimRight(input, " \t")
	idx := strings.LastIndexAny(input, " \t")
	if idx == -1 {
		return input
	}
	return input[idx+1:]
}

func parseOperatorCommandLine(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	var args []string
	var current strings.Builder
	inQuote := false
	escaped := false
	tokenStarted := false

	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			tokenStarted = true
			escaped = false
		case r == '\\' && inQuote:
			escaped = true
		case r == '"':
			inQuote = !inQuote
			tokenStarted = true
		case isOperatorCommandSpace(r) && !inQuote:
			if tokenStarted {
				args = append(args, current.String())
				current.Reset()
				tokenStarted = false
			}
		default:
			current.WriteRune(r)
			tokenStarted = true
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if tokenStarted {
		args = append(args, current.String())
	}
	return args, nil
}

func isOperatorCommandSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func executeFetchCommand(args []string) (operatorCommandResult, error) {
	options, err := parseFetchCommandOptions(args)
	if err != nil {
		return operatorCommandResult{}, err
	}
	return operatorCommandResult{FetchOptions: &options}, nil
}

func parseFetchCommandOptions(args []string) (operatorFetchOptions, error) {
	var options operatorFetchOptions
	var companyParts []string
	seenOption := false
	websiteSet := false
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			if seenOption {
				return operatorFetchOptions{}, fmt.Errorf("unexpected fetch argument %q after options", arg)
			}
			companyParts = append(companyParts, arg)
			continue
		}

		seenOption = true
		flag, value, hasValue := strings.Cut(arg, "=")
		switch flag {
		case "--aka":
			if !hasValue {
				i++
				if i >= len(args) {
					return operatorFetchOptions{}, fmt.Errorf("usage: fetch <company> [--aka <name>] [--website <url>] [--all]")
				}
				value = args[i]
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return operatorFetchOptions{}, fmt.Errorf("--aka cannot be empty")
			}
			options.Aliases = appendUniqueFetchAlias(options.Aliases, value)
		case "--website":
			if websiteSet {
				return operatorFetchOptions{}, fmt.Errorf("--website can only be provided once")
			}
			if !hasValue {
				i++
				if i >= len(args) {
					return operatorFetchOptions{}, fmt.Errorf("usage: fetch <company> [--aka <name>] [--website <url>] [--all]")
				}
				value = args[i]
			}
			options.Website = strings.TrimSpace(value)
			if options.Website == "" {
				return operatorFetchOptions{}, fmt.Errorf("--website cannot be empty")
			}
			websiteSet = true
		case "--all":
			if hasValue {
				return operatorFetchOptions{}, fmt.Errorf("--all does not accept a value")
			}
			options.All = true
		default:
			return operatorFetchOptions{}, fmt.Errorf("unknown fetch option %q", flag)
		}
	}
	options.Company = strings.TrimSpace(strings.Join(companyParts, " "))
	if options.Company == "" {
		return operatorFetchOptions{}, fmt.Errorf("usage: fetch <company> [--aka <name>] [--website <url>] [--all]")
	}
	return options, nil
}

func appendUniqueFetchAlias(aliases []string, alias string) []string {
	for _, existing := range aliases {
		if strings.EqualFold(existing, alias) {
			return aliases
		}
	}
	return append(aliases, alias)
}

func executeHealthCommand(args []string) (operatorCommandResult, error) {
	identity, err := parseHealthCommandIdentity(args)
	if err != nil {
		return operatorCommandResult{}, err
	}
	return operatorCommandResult{HealthIdentity: identity}, nil
}

func parseHealthCommandIdentity(args []string) (CompanyHealthContext, error) {
	var companyParts []string
	var aliases []string
	var website string
	seenOption := false
	websiteSet := false

	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			if seenOption {
				return CompanyHealthContext{}, fmt.Errorf("unexpected health argument %q after options", arg)
			}
			companyParts = append(companyParts, arg)
			continue
		}

		seenOption = true
		flag, value, hasValue := strings.Cut(arg, "=")
		switch flag {
		case "--aka":
			if !hasValue {
				i++
				if i >= len(args) {
					return CompanyHealthContext{}, fmt.Errorf("usage: health <company> [--aka <name>] [--website <url>]")
				}
				value = args[i]
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return CompanyHealthContext{}, fmt.Errorf("--aka cannot be empty")
			}
			aliases = appendUniqueHealthAlias(aliases, value)
		case "--website":
			if websiteSet {
				return CompanyHealthContext{}, fmt.Errorf("--website can only be provided once")
			}
			if !hasValue {
				i++
				if i >= len(args) {
					return CompanyHealthContext{}, fmt.Errorf("usage: health <company> [--aka <name>] [--website <url>]")
				}
				value = args[i]
			}
			website = strings.TrimSpace(value)
			if website == "" {
				return CompanyHealthContext{}, fmt.Errorf("--website cannot be empty")
			}
			websiteSet = true
		default:
			return CompanyHealthContext{}, fmt.Errorf("unknown health option %q", flag)
		}
	}

	company := strings.TrimSpace(strings.Join(companyParts, " "))
	if company == "" {
		return CompanyHealthContext{}, fmt.Errorf("usage: health <company> [--aka <name>] [--website <url>]")
	}
	return CompanyHealthContext{
		Company: company,
		Aliases: aliases,
		Website: website,
	}, nil
}

func appendUniqueHealthAlias(aliases []string, alias string) []string {
	for _, existing := range aliases {
		if strings.EqualFold(existing, alias) {
			return aliases
		}
	}
	return append(aliases, alias)
}

func executeDebugCommand(args []string) (operatorCommandResult, error) {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch strings.ToLower(args[0]) {
	case "status":
		return debugStatusResult(), nil
	case "on":
		if len(args) > 2 {
			return operatorCommandResult{}, fmt.Errorf("usage: debug on [path]")
		}
		path := runtimeDebugPath
		if len(args) == 2 {
			path = strings.TrimSpace(args[1])
			if path == "" {
				return operatorCommandResult{}, fmt.Errorf("debug path cannot be empty")
			}
		}
		setRuntimeDebug(true, path)
		return debugStatusResult(), nil
	case "off":
		if len(args) != 1 {
			return operatorCommandResult{}, fmt.Errorf("usage: debug off")
		}
		setRuntimeDebug(false, runtimeDebugPath)
		return debugStatusResult(), nil
	case "path":
		if len(args) != 2 {
			return operatorCommandResult{}, fmt.Errorf("usage: debug path \"./debug.log\"")
		}
		path := strings.TrimSpace(args[1])
		if path == "" {
			return operatorCommandResult{}, fmt.Errorf("debug path cannot be empty")
		}
		setRuntimeDebug(runtimeDebugEnabled, path)
		return debugStatusResult(), nil
	default:
		return operatorCommandResult{}, fmt.Errorf("unknown debug command %q", args[0])
	}
}

func debugStatusResult() operatorCommandResult {
	state := "off"
	if runtimeDebugEnabled {
		state = "on"
	}
	return operatorCommandResult{
		Title:   "Debug",
		Message: fmt.Sprintf("Debug is %s.\nLog path: %s", state, runtimeDebugPath),
	}
}
