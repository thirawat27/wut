package integration

import (
	"context"
	"testing"

	"wut/internal/config"
	appcontext "wut/internal/context"
	"wut/internal/search"
)

func TestContextAwareSearch(t *testing.T) {
	cfg := &config.Config{
		Fuzzy: config.FuzzyConfig{
			CaseSensitive: false,
			MaxDistance:   3,
			Threshold:     0.5,
		},
	}

	// Create engine without storage (will use built-in commands only)
	engine, err := search.NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	query := search.Query{
		Text:    "git",
		Limit:   10,
		Sources: []search.Source{search.SourceBuiltin}, // Only use built-in commands
	}

	ctx := context.Background()
	results, err := engine.ContextAwareSearch(ctx, query)
	if err != nil {
		t.Fatalf("ContextAwareSearch failed: %v", err)
	}

	// Should have at least built-in git command
	if len(results) == 0 {
		t.Error("Expected some results from built-in commands")
	}
}

func TestBoostContextRelevant(t *testing.T) {
	cfg := &config.Config{
		Fuzzy: config.FuzzyConfig{
			CaseSensitive: false,
			MaxDistance:   3,
			Threshold:     0.5,
		},
	}

	engine, _ := search.NewEngine(cfg, nil)

	// Test through the public API
	query := search.Query{
		Text:    "git",
		Limit:   10,
		Sources: []search.Source{search.SourceBuiltin},
	}

	ctx := context.Background()
	boosted, err := engine.ContextAwareSearch(ctx, query)
	if err != nil {
		t.Fatalf("ContextAwareSearch failed: %v", err)
	}

	if len(boosted) == 0 {
		t.Error("Expected boosted results")
	}
}

func TestGenerateContextSuggestions(t *testing.T) {
	cfg := &config.Config{
		Fuzzy: config.FuzzyConfig{
			CaseSensitive: false,
			MaxDistance:   3,
			Threshold:     0.5,
		},
	}

	engine, _ := search.NewEngine(cfg, nil)

	// Test with empty query to trigger context suggestions
	query := search.Query{
		Text:    "",
		Limit:   10,
		Sources: []search.Source{search.SourceBuiltin, search.SourceContext},
	}

	ctx := context.Background()
	results, err := engine.ContextAwareSearch(ctx, query)
	if err != nil {
		t.Fatalf("ContextAwareSearch failed: %v", err)
	}

	// Should have some results (either context or default suggestions)
	if len(results) == 0 {
		t.Error("Expected context suggestions")
	}
}

func TestIsContextRelevant(t *testing.T) {
	cfg := &config.Config{
		Fuzzy: config.FuzzyConfig{
			CaseSensitive: false,
			MaxDistance:   3,
			Threshold:     0.5,
		},
	}

	engine, _ := search.NewEngine(cfg, nil)

	ctxInfo := &appcontext.Context{
		IsGitRepo:   true,
		ProjectType: "go",
	}

	tests := []struct {
		command  string
		expected bool
	}{
		{"git status", true},
		{"git commit", true},
		{"docker ps", false},
		{"npm install", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result := engine.IsContextRelevant(tt.command, ctxInfo)
			if result != tt.expected {
				t.Errorf("IsContextRelevant(%s) = %v; expected %v", 
					tt.command, result, tt.expected)
			}
		})
	}
}
