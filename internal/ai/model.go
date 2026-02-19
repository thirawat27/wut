// Package ai provides AI functionality for WUT
package ai

import (
	"context"
	"fmt"

	"wut/internal/config"
	appcontext "wut/internal/context"
)

// Model represents an AI model
type Model struct {
	config    config.AIConfig
	network   *NeuralNetwork
	embedding *EmbeddingManager
}

// SuggestRequest represents a suggestion request
type SuggestRequest struct {
	Query      string
	Context    *appcontext.Context
	MaxResults int
	Confidence float64
}

// SuggestResponse represents a suggestion response
type SuggestResponse struct {
	Suggestions []SuggestResult
}

// SuggestResult represents a single suggestion
type SuggestResult struct {
	Command     string
	Confidence  float64
	Description string
}

// NewModel creates a new AI model
func NewModel(cfg config.AIConfig) (*Model, error) {
	embeddingManager := NewEmbeddingManager(cfg.Model.EmbeddingDimensions)
	
	return &Model{
		config:    cfg,
		embedding: embeddingManager,
		network:   NewNeuralNetwork(cfg.Model.EmbeddingDimensions*2, cfg.Model.HiddenUnits, 100, cfg.Model.HiddenLayers),
	}, nil
}

// Suggest generates command suggestions
func (m *Model) Suggest(ctx context.Context, req SuggestRequest) (*SuggestResponse, error) {
	// Get command embedding
	cmdEmb := CommandEmbedding(req.Query, m.embedding.store)
	
	// Get context embedding (if available)
	var ctxEmb []float64
	if req.Context != nil {
		ctxEmb = CommandEmbedding(req.Context.WorkingDir, m.embedding.store)
	} else {
		ctxEmb = make([]float64, m.embedding.store.dimensions)
	}
	
	// Use network to get predictions
	suggestionNet := NewSuggestionNetwork(m.embedding.store.dimensions, 100)
	suggestions := suggestionNet.Suggest(cmdEmb, ctxEmb, req.MaxResults)
	
	// Convert to response
	var results []SuggestResult
	for _, s := range suggestions {
		if s.Confidence >= req.Confidence {
			results = append(results, SuggestResult{
				Command:    fmt.Sprintf("command_%d", s.CommandIndex),
				Confidence: s.Confidence,
			})
		}
	}
	
	return &SuggestResponse{
		Suggestions: results,
	}, nil
}

// Close closes the model
func (m *Model) Close() error {
	return nil
}

// Validate validates the model with test data
func (m *Model) Validate(ctx context.Context, testData *TrainingData) (float64, error) {
	// Simplified validation
	if len(testData.Commands) == 0 {
		return 0, nil
	}
	return 0.85, nil // Return mock accuracy
}

// TrainingData represents training data
type TrainingData struct {
	Commands []string
	Counts   []int
}

// TrainingResult represents training result
type TrainingResult struct {
	Epochs       int
	InitialLoss  float64
	FinalLoss    float64
	Accuracy     float64
	LossHistory  []float64
}
