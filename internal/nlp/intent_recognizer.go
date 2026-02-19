// Package nlp provides NLP functionality for WUT
package nlp

import (
	"regexp"
	"strings"
)

// Intent represents a user intent
type Intent int

const (
	IntentUnknown Intent = iota
	IntentHelp
	IntentCommandSearch
	IntentExecute
	IntentExplain
	IntentHistory
	IntentTrain
	IntentInstall
	IntentConfig
)

// String returns the string representation of an intent
func (i Intent) String() string {
	switch i {
	case IntentHelp:
		return "help"
	case IntentCommandSearch:
		return "command_search"
	case IntentExecute:
		return "execute"
	case IntentExplain:
		return "explain"
	case IntentHistory:
		return "history"
	case IntentTrain:
		return "train"
	case IntentInstall:
		return "install"
	case IntentConfig:
		return "config"
	default:
		return "unknown"
	}
}

// IntentResult represents the result of intent recognition
type IntentResult struct {
	Intent      Intent
	Confidence  float64
	Entities    map[string]string
	RawQuery    string
}

// IntentRecognizer recognizes user intents from natural language
type IntentRecognizer struct {
	patterns map[Intent][]*regexp.Regexp
}

// NewIntentRecognizer creates a new intent recognizer
func NewIntentRecognizer() *IntentRecognizer {
	ir := &IntentRecognizer{
		patterns: make(map[Intent][]*regexp.Regexp),
	}
	
	ir.initializePatterns()
	return ir
}

// initializePatterns initializes intent patterns
func (ir *IntentRecognizer) initializePatterns() {
	// Help intent patterns
	ir.patterns[IntentHelp] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^help\b`),
		regexp.MustCompile(`(?i)\bhelp\s+me\b`),
		regexp.MustCompile(`(?i)\bhow\s+(?:to|do|can)\b`),
		regexp.MustCompile(`(?i)\bwhat\s+(?:is|are|does)\b`),
		regexp.MustCompile(`(?i)\bshow\s+help\b`),
		regexp.MustCompile(`(?i)\bassist(?:ance)?\b`),
	}
	
	// Command search patterns
	ir.patterns[IntentCommandSearch] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:find|search|look\s+for|get)\s+(?:command|cmd)\b`),
		regexp.MustCompile(`(?i)\bhow\s+(?:to|do)\s+\w+`),
		regexp.MustCompile(`(?i)\bcommand\s+(?:for|to)\b`),
		regexp.MustCompile(`(?i)\bsuggest\b`),
		regexp.MustCompile(`(?i)\bneed\s+(?:a\s+)?command\b`),
		regexp.MustCompile(`(?i)\bwhat\s+command\b`),
		regexp.MustCompile(`(?i)\bdeploy\b`),
		regexp.MustCompile(`(?i)\bpush\s+(?:my\s+)?code\b`),
		regexp.MustCompile(`(?i)\bclone\s+repo\b`),
		regexp.MustCompile(`(?i)\bcommit\s+(?:changes?|code)\b`),
		regexp.MustCompile(`(?i)\brun\s+(?:test|build|deploy)\b`),
		regexp.MustCompile(`(?i)\bstart\s+(?:server|app|service)\b`),
		regexp.MustCompile(`(?i)\bstop\s+(?:server|app|service)\b`),
		regexp.MustCompile(`(?i)\brestart\s+(?:server|app|service)\b`),
	}
	
	// Execute intent patterns
	ir.patterns[IntentExecute] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\brun\b`),
		regexp.MustCompile(`(?i)\bexecute\b`),
		regexp.MustCompile(`(?i)\bdo\b`),
		regexp.MustCompile(`(?i)\bperform\b`),
	}
	
	// Explain intent patterns
	ir.patterns[IntentExplain] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bexplain\b`),
		regexp.MustCompile(`(?i)\bwhat\s+does\b`),
		regexp.MustCompile(`(?i)\bhow\s+does\b`),
		regexp.MustCompile(`(?i)\btell\s+me\s+about\b`),
		regexp.MustCompile(`(?i)\bdescribe\b`),
		regexp.MustCompile(`(?i)\bwhat\s+is\b`),
	}
	
	// History intent patterns
	ir.patterns[IntentHistory] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bhistory\b`),
		regexp.MustCompile(`(?i)\bpast\s+commands\b`),
		regexp.MustCompile(`(?i)\bprevious\s+commands\b`),
		regexp.MustCompile(`(?i)\bwhat\s+did\s+I\s+(?:run|do)\b`),
		regexp.MustCompile(`(?i)\bshow\s+(?:my\s+)?history\b`),
		regexp.MustCompile(`(?i)\brecent\s+commands\b`),
	}
	
	// Train intent patterns
	ir.patterns[IntentTrain] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\btrain\b`),
		regexp.MustCompile(`(?i)\blearn\b`),
		regexp.MustCompile(`(?i)\bimprove\b`),
		regexp.MustCompile(`(?i)\bupdate\s+model\b`),
		regexp.MustCompile(`(?i)\bteach\b`),
	}
	
	// Install intent patterns
	ir.patterns[IntentInstall] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\binstall\b`),
		regexp.MustCompile(`(?i)\bsetup\b`),
		regexp.MustCompile(`(?i)\bconfigure\b`),
		regexp.MustCompile(`(?i)\bintegrate\b`),
	}
	
	// Config intent patterns
	ir.patterns[IntentConfig] = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bconfig\b`),
		regexp.MustCompile(`(?i)\bsettings?\b`),
		regexp.MustCompile(`(?i)\bpreferences?\b`),
		regexp.MustCompile(`(?i)\boptions?\b`),
	}
}

// Recognize recognizes the intent from a query
func (ir *IntentRecognizer) Recognize(query string) *IntentResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return &IntentResult{
			Intent:     IntentUnknown,
			Confidence: 1.0,
			RawQuery:   query,
			Entities:   make(map[string]string),
		}
	}
	
	result := &IntentResult{
		Intent:   IntentUnknown,
		RawQuery: query,
		Entities: make(map[string]string),
	}
	
	// Check for exact command matches first
	if strings.HasPrefix(query, "wut ") {
		return ir.recognizeWutCommand(query)
	}
	
	// Pattern matching
	bestIntent := IntentUnknown
	bestConfidence := 0.0
	
	for intent, patterns := range ir.patterns {
		for _, pattern := range patterns {
			if pattern.MatchString(query) {
				confidence := ir.calculateConfidence(query, pattern)
				if confidence > bestConfidence {
					bestConfidence = confidence
					bestIntent = intent
				}
			}
		}
	}
	
	result.Intent = bestIntent
	result.Confidence = bestConfidence
	
	// Extract entities
	result.Entities = ir.extractEntities(query)
	
	return result
}

// recognizeWutCommand recognizes wut subcommands
func (ir *IntentRecognizer) recognizeWutCommand(query string) *IntentResult {
	result := &IntentResult{
		RawQuery: query,
		Entities: make(map[string]string),
	}
	
	// Remove "wut " prefix
	query = strings.TrimPrefix(query, "wut ")
	
	// Check for subcommands
	switch {
	case strings.HasPrefix(query, "help") || query == "--help" || query == "-h":
		result.Intent = IntentHelp
		result.Confidence = 1.0
	case strings.HasPrefix(query, "suggest"):
		result.Intent = IntentCommandSearch
		result.Confidence = 1.0
	case strings.HasPrefix(query, "history"):
		result.Intent = IntentHistory
		result.Confidence = 1.0
	case strings.HasPrefix(query, "explain"):
		result.Intent = IntentExplain
		result.Confidence = 1.0
		// Extract command to explain
		parts := strings.Fields(query)
		if len(parts) > 1 {
			result.Entities["command"] = strings.Join(parts[1:], " ")
		}
	case strings.HasPrefix(query, "train"):
		result.Intent = IntentTrain
		result.Confidence = 1.0
	case strings.HasPrefix(query, "install"):
		result.Intent = IntentInstall
		result.Confidence = 1.0
	case strings.HasPrefix(query, "config"):
		result.Intent = IntentConfig
		result.Confidence = 1.0
	default:
		result.Intent = IntentCommandSearch
		result.Confidence = 0.7
	}
	
	return result
}

// calculateConfidence calculates confidence score for a match
func (ir *IntentRecognizer) calculateConfidence(query string, pattern *regexp.Regexp) float64 {
	// Simple confidence calculation based on match quality
	matches := pattern.FindAllStringIndex(query, -1)
	if len(matches) == 0 {
		return 0
	}
	
	// Longer matches = higher confidence
	totalMatchLen := 0
	for _, match := range matches {
		totalMatchLen += match[1] - match[0]
	}
	
	confidence := float64(totalMatchLen) / float64(len(query))
	if confidence > 1.0 {
		confidence = 1.0
	}
	
	// Boost confidence for exact word matches
	if len(matches) == 1 && matches[0][0] == 0 {
		confidence += 0.1
	}
	
	return confidence
}

// extractEntities extracts entities from the query
func (ir *IntentRecognizer) extractEntities(query string) map[string]string {
	entities := make(map[string]string)
	
	// Extract command mentions
	commandPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(git|docker|kubectl|npm|pip|go|python|node)\b`),
		regexp.MustCompile(`(?i)\b(run|build|test|deploy|push|pull|commit|clone)\s+(?:command|cmd)?\b`),
	}
	
	for _, pattern := range commandPatterns {
		matches := pattern.FindStringSubmatch(query)
		if len(matches) > 1 {
			entities["tool"] = matches[1]
			break
		}
	}
	
	// Extract file/directory mentions
	filePattern := regexp.MustCompile(`\b([\w\-./]+\.(?:go|js|py|ts|json|yaml|yml|md|txt|sh|ps1))\b`)
	if matches := filePattern.FindStringSubmatch(query); len(matches) > 1 {
		entities["file"] = matches[1]
	}
	
	// Extract branch names
	branchPattern := regexp.MustCompile(`(?i)\b(?:branch|to)\s+(?:\w+\/)?(\w+)\b`)
	if matches := branchPattern.FindStringSubmatch(query); len(matches) > 1 {
		entities["branch"] = matches[1]
	}
	
	// Extract numbers
	numberPattern := regexp.MustCompile(`\b(\d+)\b`)
	if matches := numberPattern.FindStringSubmatch(query); len(matches) > 1 {
		entities["number"] = matches[1]
	}
	
	return entities
}

// Tokenizer tokenizes text
type Tokenizer struct{}

// NewTokenizer creates a new tokenizer
func NewTokenizer() *Tokenizer {
	return &Tokenizer{}
}

// Tokenize tokenizes a string into words
func (t *Tokenizer) Tokenize(text string) []string {
	// Normalize text
	text = strings.ToLower(text)
	
	// Split on non-alphanumeric characters
	re := regexp.MustCompile(`[^a-z0-9]+`)
	tokens := re.Split(text, -1)
	
	// Filter empty tokens
	var result []string
	for _, token := range tokens {
		if token != "" {
			result = append(result, token)
		}
	}
	
	return result
}

// TokenizeWithPositions tokenizes and returns positions
func (t *Tokenizer) TokenizeWithPositions(text string) []Token {
	tokens := t.Tokenize(text)
	var result []Token
	pos := 0
	
	for _, tok := range tokens {
		// Find position in original text
		idx := strings.Index(text[pos:], tok)
		if idx != -1 {
			start := pos + idx
			end := start + len(tok)
			result = append(result, Token{
				Text:  tok,
				Start: start,
				End:   end,
			})
			pos = end
		}
	}
	
	return result
}

// Token represents a token with position
type Token struct {
	Text  string
	Start int
	End   int
}

// CommandMapper maps natural language to commands
type CommandMapper struct {
	mappings map[string][]string
}

// NewCommandMapper creates a new command mapper
func NewCommandMapper() *CommandMapper {
	cm := &CommandMapper{
		mappings: make(map[string][]string),
	}
	cm.initializeMappings()
	return cm
}

// initializeMappings initializes command mappings
func (cm *CommandMapper) initializeMappings() {
	// Git commands
	cm.mappings["push code to production"] = []string{"git push origin main", "git push origin master"}
	cm.mappings["deploy to production"] = []string{"git push origin main", "docker-compose up -d", "kubectl apply -f deployment.yaml"}
	cm.mappings["clone repository"] = []string{"git clone <url>"}
	cm.mappings["commit changes"] = []string{"git add .", "git commit -m 'message'"}
	cm.mappings["check status"] = []string{"git status"}
	cm.mappings["switch branch"] = []string{"git checkout <branch>"}
	cm.mappings["create branch"] = []string{"git checkout -b <branch>"}
	cm.mappings["pull changes"] = []string{"git pull origin <branch>"}
	cm.mappings["merge branch"] = []string{"git merge <branch>"}
	cm.mappings["stash changes"] = []string{"git stash"}
	cm.mappings["pop stash"] = []string{"git stash pop"}
	cm.mappings["view log"] = []string{"git log --oneline -10"}
	cm.mappings["undo commit"] = []string{"git reset --soft HEAD~1"}
	
	// Docker commands
	cm.mappings["list containers"] = []string{"docker ps", "docker ps -a"}
	cm.mappings["start container"] = []string{"docker start <container>"}
	cm.mappings["stop container"] = []string{"docker stop <container>"}
	cm.mappings["remove container"] = []string{"docker rm <container>"}
	cm.mappings["build image"] = []string{"docker build -t <image> ."}
	cm.mappings["run container"] = []string{"docker run -d <image>"}
	cm.mappings["view logs"] = []string{"docker logs <container>"}
	cm.mappings["execute in container"] = []string{"docker exec -it <container> <command>"}
	cm.mappings["compose up"] = []string{"docker-compose up -d"}
	cm.mappings["compose down"] = []string{"docker-compose down"}
	cm.mappings["compose build"] = []string{"docker-compose build"}
	cm.mappings["compose logs"] = []string{"docker-compose logs"}
	
	// Kubernetes commands
	cm.mappings["list pods"] = []string{"kubectl get pods"}
	cm.mappings["list services"] = []string{"kubectl get svc"}
	cm.mappings["list deployments"] = []string{"kubectl get deployments"}
	cm.mappings["describe pod"] = []string{"kubectl describe pod <pod>"}
	cm.mappings["view pod logs"] = []string{"kubectl logs <pod>"}
	cm.mappings["exec in pod"] = []string{"kubectl exec -it <pod> -- <command>"}
	cm.mappings["apply config"] = []string{"kubectl apply -f <file>"}
	cm.mappings["delete resource"] = []string{"kubectl delete <resource> <name>"}
	cm.mappings["port forward"] = []string{"kubectl port-forward <pod> <ports>"}
	cm.mappings["scale deployment"] = []string{"kubectl scale deployment <name> --replicas=<n>"}
	
	// Node.js/NPM commands
	cm.mappings["install packages"] = []string{"npm install", "npm ci"}
	cm.mappings["install dev package"] = []string{"npm install --save-dev <package>"}
	cm.mappings["run script"] = []string{"npm run <script>"}
	cm.mappings["run tests"] = []string{"npm test"}
	cm.mappings["start app"] = []string{"npm start"}
	cm.mappings["build app"] = []string{"npm run build"}
	cm.mappings["update packages"] = []string{"npm update"}
	
	// Python commands
	cm.mappings["install python package"] = []string{"pip install <package>"}
	cm.mappings["list python packages"] = []string{"pip list"}
	cm.mappings["freeze requirements"] = []string{"pip freeze > requirements.txt"}
	cm.mappings["run python script"] = []string{"python <script>"}
	cm.mappings["run pytest"] = []string{"pytest"}
	cm.mappings["create virtualenv"] = []string{"python -m venv venv"}
	cm.mappings["activate virtualenv"] = []string{"source venv/bin/activate", "venv\\Scripts\\activate"}
	
	// Go commands
	cm.mappings["build go project"] = []string{"go build"}
	cm.mappings["run go program"] = []string{"go run ."}
	cm.mappings["run go tests"] = []string{"go test ./..."}
	cm.mappings["get package"] = []string{"go get <package>"}
	cm.mappings["tidy modules"] = []string{"go mod tidy"}
	cm.mappings["download modules"] = []string{"go mod download"}
	cm.mappings["format code"] = []string{"go fmt ./..."}
	cm.mappings["vet code"] = []string{"go vet ./..."}
}

// MapToCommand maps natural language to commands
func (cm *CommandMapper) MapToCommand(naturalLanguage string) ([]string, float64) {
	naturalLanguage = strings.ToLower(strings.TrimSpace(naturalLanguage))
	
	// Direct match
	if commands, ok := cm.mappings[naturalLanguage]; ok {
		return commands, 1.0
	}
	
	// Partial match
	bestMatch := ""
	bestScore := 0.0
	
	for pattern := range cm.mappings {
		score := calculateSimilarity(naturalLanguage, pattern)
		if score > bestScore {
			bestScore = score
			bestMatch = pattern
		}
	}
	
	if bestScore > 0.6 {
		return cm.mappings[bestMatch], bestScore
	}
	
	return nil, 0
}

// calculateSimilarity calculates simple word overlap similarity
func calculateSimilarity(a, b string) float64 {
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)
	
	if len(wordsA) == 0 || len(wordsB) == 0 {
		return 0
	}
	
	wordSet := make(map[string]bool)
	for _, w := range wordsA {
		wordSet[w] = true
	}
	
	matchCount := 0
	for _, w := range wordsB {
		if wordSet[w] {
			matchCount++
		}
	}
	
	return float64(matchCount*2) / float64(len(wordsA)+len(wordsB))
}
