package commandsearch

import (
	"path/filepath"
	"strings"
	"unicode"

	"wut/internal/performance"
)

// Profile captures normalized command intent for matching and ranking.
type Profile struct {
	Raw        string
	Normalized string
	Executable string
	Subcommand string
	Intent     string
	Tokens     []string
	SearchText string
}

// Query is a parsed search query.
type Query struct {
	Raw        string
	Normalized string
	Executable string
	Subcommand string
	Intent     string
	Tokens     []string
}

// ParseQuery normalizes a user search query into an intent-aware structure.
func ParseQuery(query string) Query {
	profile := BuildProfile(query)
	return Query{
		Raw:        profile.Raw,
		Normalized: profile.Normalized,
		Executable: profile.Executable,
		Subcommand: profile.Subcommand,
		Intent:     profile.Intent,
		Tokens:     append([]string(nil), profile.Tokens...),
	}
}

// BuildProfile extracts executable, subcommand, aliases and tokenized search
// text from a shell command.
func BuildProfile(command string) Profile {
	command = normalizeWhitespace(command)
	command = unwrapShellWrapper(command, 0)
	command = normalizeWhitespace(command)
	if command == "" {
		return Profile{}
	}

	rawTokens := strings.Fields(command)
	if len(rawTokens) == 0 {
		return Profile{}
	}

	executable, remainder, aliases := canonicalExecutable(rawTokens[0], rawTokens[1:])
	subcommand, intent, intentAliases := detectIntent(executable, remainder)
	aliases = append(aliases, intentAliases...)

	searchTerms := []string{
		strings.ToLower(command),
		executable,
		subcommand,
		intent,
	}
	searchTerms = append(searchTerms, aliases...)
	searchTerms = uniqueStrings(searchTerms...)

	tokenTerms := append([]string(nil), searchTerms...)
	tokenTerms = append(tokenTerms, rawTokens...)
	tokens := uniqueTokens(tokenTerms...)

	return Profile{
		Raw:        command,
		Normalized: strings.ToLower(command),
		Executable: executable,
		Subcommand: subcommand,
		Intent:     intent,
		Tokens:     tokens,
		SearchText: strings.Join(searchTerms, " "),
	}
}

// Score measures how well a query matches a command profile.
func Score(query Query, profile Profile, matcher *performance.FastMatcher) (float64, bool) {
	if query.Normalized == "" || profile.Normalized == "" {
		return 0, false
	}

	score := 0.0

	if query.Normalized == profile.Normalized {
		score += 1600
	}
	if query.Intent != "" && query.Intent == profile.Intent && profile.Intent != "" {
		score += 1100
	}
	if query.Executable != "" && query.Executable == profile.Executable && profile.Executable != "" {
		score += 180
	}
	if query.Subcommand != "" && query.Subcommand == profile.Subcommand && profile.Subcommand != "" {
		score += 240
	}

	switch {
	case strings.HasPrefix(profile.Normalized, query.Normalized):
		score += 700
	case strings.Contains(profile.Normalized, query.Normalized):
		score += 440
	}
	switch {
	case strings.HasPrefix(profile.SearchText, query.Normalized):
		score += 260
	case strings.Contains(profile.SearchText, query.Normalized):
		score += 180
	}

	if coverage, ordered := tokenCoverage(query.Tokens, profile.Tokens); coverage > 0 {
		score += coverage * 460
		if ordered {
			score += 120
		}
		if coverage == 1 {
			score += 140
		}
	}

	if matcher != nil {
		if match := matcher.Match(query.Normalized, profile.Normalized); match.Matched {
			score += match.Score * 220
		}
		if profile.SearchText != "" && profile.SearchText != profile.Normalized {
			if match := matcher.Match(query.Normalized, profile.SearchText); match.Matched {
				score += match.Score * 150
			}
		}
		if query.Intent != "" && profile.Intent != "" {
			if match := matcher.Match(query.Intent, profile.Intent); match.Matched {
				score += match.Score * 240
			}
		}
	}

	return score, score > 0
}

// HasAnchor reports whether a query has a meaningful command-level anchor in a
// target profile. This keeps history search from surfacing unrelated long log
// lines just because a loose fuzzy matcher found a weak character-order match.
func HasAnchor(query Query, profile Profile, matcher *performance.FastMatcher) bool {
	if query.Normalized == "" || profile.Normalized == "" {
		return false
	}

	switch {
	case query.Normalized == profile.Normalized:
		return true
	case strings.HasPrefix(profile.Normalized, query.Normalized):
		return true
	case strings.Contains(profile.Normalized, query.Normalized):
		return true
	case profile.SearchText != "" && strings.HasPrefix(profile.SearchText, query.Normalized):
		return true
	case profile.SearchText != "" && strings.Contains(profile.SearchText, query.Normalized):
		return true
	}

	if query.Intent != "" && profile.Intent != "" {
		switch {
		case query.Intent == profile.Intent:
			return true
		case strings.HasPrefix(profile.Intent, query.Intent):
			return true
		case strings.Contains(profile.Intent, query.Intent):
			return true
		case strongFuzzyEquivalent(query.Intent, profile.Intent, matcher):
			return true
		}
	}

	if query.Executable != "" && profile.Executable != "" {
		if query.Executable == profile.Executable || strongFuzzyEquivalent(query.Executable, profile.Executable, matcher) {
			return true
		}
	}

	if query.Subcommand != "" && profile.Subcommand != "" {
		if tokenMatches(query.Subcommand, profile.Subcommand) || strongFuzzyEquivalent(query.Subcommand, profile.Subcommand, matcher) {
			return true
		}
	}

	if coverage, _ := tokenCoverage(query.Tokens, profile.Tokens); coverage >= 0.5 {
		return true
	}

	return hasStrongTokenFuzzy(query.Tokens, profile.Tokens, matcher)
}

func hasStrongTokenFuzzy(queryTokens, targetTokens []string, matcher *performance.FastMatcher) bool {
	if matcher == nil || len(queryTokens) == 0 || len(targetTokens) == 0 {
		return false
	}

	for _, queryToken := range queryTokens {
		if len(queryToken) < 3 {
			continue
		}
		for _, targetToken := range targetTokens {
			if strongFuzzyEquivalent(queryToken, targetToken, matcher) {
				return true
			}
		}
	}

	return false
}

func strongFuzzyEquivalent(query, target string, matcher *performance.FastMatcher) bool {
	query = normalizeCommandToken(query)
	target = normalizeCommandToken(target)
	if query == "" || target == "" {
		return false
	}
	if query == target {
		return true
	}
	if strings.HasPrefix(target, query) {
		return true
	}
	if matcher == nil {
		return false
	}

	match := matcher.Match(query, target)
	if !match.Matched {
		return false
	}

	minLen := len(query)
	if len(target) < minLen {
		minLen = len(target)
	}

	threshold := 0.82
	switch {
	case minLen <= 3:
		threshold = 0.93
	case minLen <= 4:
		threshold = 0.88
	}

	return match.Score >= threshold
}

func unwrapShellWrapper(command string, depth int) string {
	if depth >= 2 {
		return command
	}

	tokens := strings.Fields(command)
	if len(tokens) == 0 {
		return ""
	}

	start := 0
	for start < len(tokens) {
		token := normalizeCommandToken(tokens[start])
		switch token {
		case "sudo", "doas", "command", "builtin", "env", "noglob", "nohup", "time":
			start++
			continue
		}
		if isEnvAssignment(token) {
			start++
			continue
		}
		break
	}
	if start >= len(tokens) {
		return ""
	}

	tokens = tokens[start:]
	if len(tokens) < 3 {
		return strings.Join(tokens, " ")
	}

	execToken := normalizeCommandToken(tokens[0])
	switch execToken {
	case "sh", "bash", "zsh", "fish", "pwsh", "powershell", "cmd":
		if isShellCommandFlag(tokens[1]) {
			nested := normalizeWhitespace(strings.Join(tokens[2:], " "))
			nested = trimOuterQuotes(nested)
			if nested != "" {
				return unwrapShellWrapper(nested, depth+1)
			}
		}
	}

	return strings.Join(tokens, " ")
}

func canonicalExecutable(token string, remainder []string) (string, []string, []string) {
	token = normalizeCommandToken(token)
	if token == "" {
		return "", remainder, nil
	}

	aliases := make([]string, 0, 4)
	switch token {
	case "docker-compose":
		aliases = append(aliases, "docker compose")
		return "docker", append([]string{"compose"}, remainder...), aliases
	case "python3", "py":
		aliases = append(aliases, token)
		return "python", remainder, aliases
	case "pip3":
		aliases = append(aliases, token)
		return "pip", remainder, aliases
	case "kubectl":
		return "kubectl", remainder, aliases
	default:
		return token, remainder, aliases
	}
}

func detectIntent(executable string, remainder []string) (string, string, []string) {
	if executable == "" {
		return "", "", nil
	}
	if len(remainder) == 0 {
		return "", executable, nil
	}

	first := normalizeCommandToken(remainder[0])
	intentParts := []string{executable}
	aliases := make([]string, 0, 3)
	subcommand := ""

	switch executable {
	case "docker":
		if !isSubcommandToken(first) {
			return "", executable, nil
		}
		subcommand = first
		if first == "compose" {
			intentParts = append(intentParts, "compose")
			if len(remainder) > 1 {
				next := normalizeCommandToken(remainder[1])
				if isSubcommandToken(next) {
					intentParts = append(intentParts, next)
				}
			}
			aliases = append(aliases, strings.Join(intentParts, "-"))
			return "compose", strings.Join(intentParts, " "), aliases
		}
	case "npm", "pnpm", "yarn":
		if !isSubcommandToken(first) {
			return "", executable, nil
		}
		subcommand = first
		intentParts = append(intentParts, first)
		if first == "run" && len(remainder) > 1 {
			next := normalizeCommandToken(remainder[1])
			if isSubcommandToken(next) {
				intentParts = append(intentParts, next)
			}
		}
		return first, strings.Join(intentParts, " "), nil
	case "python":
		if first == "-m" && len(remainder) > 1 {
			module := normalizeCommandToken(remainder[1])
			if isSubcommandToken(module) {
				intentParts = append(intentParts, "-m", module)
				aliases = append(aliases, module)
				return "-m", strings.Join(intentParts, " "), aliases
			}
		}
		if !isSubcommandToken(first) {
			return "", executable, nil
		}
		subcommand = first
	default:
		if !isSubcommandToken(first) {
			return "", executable, nil
		}
		subcommand = first
	}

	intentParts = append(intentParts, first)
	return subcommand, strings.Join(intentParts, " "), nil
}

func tokenCoverage(queryTokens, targetTokens []string) (float64, bool) {
	if len(queryTokens) == 0 || len(targetTokens) == 0 {
		return 0, false
	}

	matched := 0
	lastIdx := -1
	ordered := true

	for _, queryToken := range queryTokens {
		foundIdx := -1
		for i, targetToken := range targetTokens {
			if !tokenMatches(queryToken, targetToken) {
				continue
			}
			foundIdx = i
			break
		}
		if foundIdx == -1 {
			continue
		}
		matched++
		if lastIdx > foundIdx {
			ordered = false
		}
		if foundIdx > lastIdx {
			lastIdx = foundIdx
		}
	}

	return float64(matched) / float64(len(queryTokens)), ordered && matched > 1
}

func tokenMatches(queryToken, targetToken string) bool {
	if queryToken == "" || targetToken == "" {
		return false
	}
	return queryToken == targetToken ||
		strings.HasPrefix(targetToken, queryToken) ||
		strings.Contains(targetToken, queryToken)
}

func uniqueTokens(values ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values)*2)

	for _, value := range values {
		for _, token := range tokenize(value) {
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			result = append(result, token)
		}
	}
	return result
}

func uniqueStrings(values ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeWhitespace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func tokenize(value string) []string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return nil
	}
	return strings.FieldsFunc(value, func(r rune) bool {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return false
		case r == '+', r == '#':
			return false
		default:
			return true
		}
	})
}

func normalizeCommandToken(token string) string {
	token = strings.TrimSpace(trimOuterQuotes(token))
	if token == "" {
		return ""
	}
	token = filepath.Base(token)
	token = strings.TrimSuffix(token, ".exe")
	token = strings.TrimSuffix(token, ".cmd")
	token = strings.TrimSuffix(token, ".bat")
	return strings.ToLower(token)
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func trimOuterQuotes(value string) string {
	for len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '`' && value[len(value)-1] == '`') {
			value = strings.TrimSpace(value[1 : len(value)-1])
			continue
		}
		break
	}
	return value
}

func isShellCommandFlag(token string) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	switch token {
	case "-c", "-lc", "-ic", "-command", "-cmd", "/c":
		return true
	default:
		return false
	}
}

func isEnvAssignment(token string) bool {
	if strings.Count(token, "=") != 1 {
		return false
	}
	name, _, _ := strings.Cut(token, "=")
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
			if i == 0 && unicode.IsDigit(r) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func isSubcommandToken(token string) bool {
	if token == "" {
		return false
	}
	if strings.HasPrefix(token, "-") {
		return false
	}
	if isEnvAssignment(token) {
		return false
	}
	if strings.ContainsAny(token, `/\`) {
		return false
	}
	return true
}
