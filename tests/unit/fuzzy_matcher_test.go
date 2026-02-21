package unit

import (
	"testing"
	"wut/pkg/fuzzy"
)

func TestNewMatcher(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.5)
	if m == nil {
		t.Fatal("NewMatcher returned nil")
	}
}

func TestMatchExactMatch(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	match := m.Match("test", "test")
	
	if match.Confidence != 1.0 {
		t.Errorf("Expected confidence 1.0 for exact match, got %f", match.Confidence)
	}
	if match.Distance != 0 {
		t.Errorf("Expected distance 0 for exact match, got %d", match.Distance)
	}
}

func TestMatchCaseInsensitive(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	match := m.Match("Test", "test")
	
	// Case insensitive should have high confidence (>= 0.9)
	if match.Confidence < 0.9 {
		t.Errorf("Expected confidence >= 0.9 for case insensitive match, got %f", match.Confidence)
	}
}

func TestMatchCaseSensitive(t *testing.T) {
	m := fuzzy.NewMatcher(true, 3, 0.0)
	match := m.Match("Test", "test")
	
	if match.Confidence == 1.0 {
		t.Error("Expected confidence < 1.0 for case sensitive mismatch")
	}
}

func TestMatchPrefix(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	m.SetAlgorithm(fuzzy.AlgorithmPrefix)
	
	match := m.Match("git", "github")
	if match.Confidence <= 0 {
		t.Error("Expected positive confidence for prefix match")
	}
}

func TestMatchContains(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	m.SetAlgorithm(fuzzy.AlgorithmContains)
	
	match := m.Match("hub", "github")
	if match.Confidence <= 0 {
		t.Error("Expected positive confidence for substring match")
	}
}

func TestMatchMultiple(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.5)
	targets := []string{"git", "github", "gitlab", "docker", "kubectl"}
	
	results := m.MatchMultiple("git", targets)
	
	if len(results) == 0 {
		t.Error("Expected at least one match")
	}
	
	// Results should be sorted by confidence
	for i := 1; i < len(results); i++ {
		if results[i-1].Match.Confidence < results[i].Match.Confidence {
			t.Error("Results not sorted by confidence")
		}
	}
}

func TestMatchEmptyQuery(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	match := m.Match("", "test")
	
	if match.Confidence != 0 {
		t.Errorf("Expected confidence 0 for empty query, got %f", match.Confidence)
	}
}

func TestSetWeights(t *testing.T) {
	m := fuzzy.NewMatcher(false, 3, 0.0)
	m.SetWeights(0.5, 0.3, 0.2)
	
	// Weights are private, but we can test that it doesn't panic
	_ = m.Match("test", "test")
}
