package tuiapp

import (
	"fmt"
	"strings"
)

type operatorCommandResult struct {
	Title          string
	Message        string
	HealthIdentity CompanyHealthContext
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
		Name:    "help",
		Aliases: []string{"?"},
		Execute: operatorCommandHelp,
		Completions: []operatorCommandCompletionSpec{
			{Label: "debug", Insert: "debug"},
			{Label: "health", Insert: "health"},
		},
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

	var completions []operatorCommandCompletion
	for _, completion := range spec.Completions {
		insert := completion.Insert
		if fuzzyMatchOperatorCommand(rest, completion.Label) || fuzzyMatchOperatorCommand(rest, strings.TrimSpace(insert)) {
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
	if strings.TrimSpace(rest) == "" || healthCommandAwaitingFlagValue(rest) {
		return nil
	}

	base, prefix, ok := healthCommandFlagCompletionContext(rest)
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
		if fuzzyMatchOperatorCommand(prefix, flag.Label) {
			completions = append(completions, operatorCommandCompletion{
				Label: flag.Label,
				Value: "health " + base + flag.Insert,
			})
		}
	}
	return completions
}

func healthCommandFlagCompletionContext(rest string) (base string, prefix string, ok bool) {
	if strings.HasSuffix(rest, " ") || strings.HasSuffix(rest, "\t") {
		return rest, "", true
	}
	trimmedRight := strings.TrimRight(rest, " \t")
	tokenStart := strings.LastIndexAny(trimmedRight, " \t")
	if tokenStart == -1 {
		return "", "", false
	}
	current := trimmedRight[tokenStart+1:]
	if !strings.HasPrefix(current, "--") {
		return "", "", false
	}
	return trimmedRight[:tokenStart+1], current, true
}

func healthCommandAwaitingFlagValue(rest string) bool {
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
	return healthFlagRequiresValue(flag)
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

func healthFlagRequiresValue(flag string) bool {
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
	input = strings.TrimLeft(input, " \t")
	command, rest, hasRest := splitOperatorCommandInput(input)
	if !strings.EqualFold(command, "health") {
		return ""
	}
	if !hasRest {
		return ""
	}
	rest = strings.TrimLeft(rest, " \t")
	if strings.TrimSpace(rest) == "" {
		return "<company>"
	}
	if hint := healthCommandValueHint(rest); hint != "" {
		return hint
	}
	return ""
}

func healthCommandValueHint(rest string) string {
	args, err := parseOperatorCommandLine(rest)
	if err != nil || len(args) == 0 {
		return ""
	}
	last := args[len(args)-1]
	flag, value, hasValue := strings.Cut(last, "=")
	if hasValue && strings.TrimSpace(value) != "" {
		return ""
	}
	if !healthFlagRequiresValue(flag) {
		return ""
	}
	switch flag {
	case "--aka":
		if strings.HasSuffix(rest, " ") || strings.HasSuffix(rest, "\t") || hasValue {
			return "<name>"
		}
		return " <name>"
	case "--website":
		if strings.HasSuffix(rest, " ") || strings.HasSuffix(rest, "\t") || hasValue {
			return "<url>"
		}
		return " <url>"
	default:
		return ""
	}
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

func operatorCommandHelp(args []string) (operatorCommandResult, error) {
	if len(args) > 0 && strings.EqualFold(args[0], "debug") {
		return operatorCommandResult{
			Title: "Command Help",
			Message: strings.Join([]string{
				"debug status",
				"debug on",
				"debug off",
				"debug path \"./debug.log\"",
			}, "\n"),
		}, nil
	}
	if len(args) > 0 && strings.EqualFold(args[0], "health") {
		return operatorCommandResult{
			Title: "Command Help",
			Message: strings.Join([]string{
				"health <company> [--aka <name>] [--website <url>]",
				"Fetch company health info for any company name.",
				"Use repeated --aka values for alternate company names.",
			}, "\n"),
		}, nil
	}
	return operatorCommandResult{
		Title: "Commands",
		Message: strings.Join([]string{
			"debug status  Show debug state",
			"debug on      Enable debug output",
			"debug off     Disable debug output",
			"debug path    Set debug log path",
			"health        Fetch company health info",
		}, "\n"),
	}, nil
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
