// Package ai provides AI functionality for WUT
package ai

import (
	"sort"
	"strings"
)

// Embedding represents a word/command embedding vector
type Embedding struct {
	Vector []float64
	Word   string
}

// EmbeddingStore manages word embeddings
type EmbeddingStore struct {
	embeddings map[string][]float64
	dimensions int
}

// NewEmbeddingStore creates a new embedding store
func NewEmbeddingStore(dimensions int) *EmbeddingStore {
	return &EmbeddingStore{
		embeddings: make(map[string][]float64),
		dimensions: dimensions,
	}
}

// Get retrieves an embedding for a word
func (es *EmbeddingStore) Get(word string) []float64 {
	if emb, ok := es.embeddings[word]; ok {
		return emb
	}
	return es.generateRandomEmbedding(word)
}

// Set sets an embedding for a word
func (es *EmbeddingStore) Set(word string, vector []float64) {
	es.embeddings[word] = vector
}

// generateRandomEmbedding generates a pseudo-random embedding based on word content
func (es *EmbeddingStore) generateRandomEmbedding(word string) []float64 {
	// Simple hash-based embedding generation
	vector := make([]float64, es.dimensions)
	hash := hashString(word)
	
	for i := 0; i < es.dimensions; i++ {
		vector[i] = float64((hash+uint64(i)*2654435761) % 1000) / 1000.0
	}
	
	es.embeddings[word] = vector
	return vector
}

// hashString creates a simple hash from a string
func hashString(s string) uint64 {
	h := uint64(5381)
	for _, c := range s {
		h = ((h << 5) + h) + uint64(c)
	}
	return h
}

// CosineSimilarity calculates cosine similarity between two vectors
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt returns the square root of x
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// CommandEmbedding creates an embedding for a command string
func CommandEmbedding(command string, store *EmbeddingStore) []float64 {
	words := tokenize(command)
	if len(words) == 0 {
		return make([]float64, store.dimensions)
	}
	
	// Average word embeddings
	result := make([]float64, store.dimensions)
	for _, word := range words {
		emb := store.Get(word)
		for i := 0; i < store.dimensions; i++ {
			result[i] += emb[i]
		}
	}
	
	for i := 0; i < store.dimensions; i++ {
		result[i] /= float64(len(words))
	}
	
	return result
}

// tokenize splits a command into tokens
func tokenize(command string) []string {
	command = strings.ToLower(command)
	// Simple tokenization
	fields := strings.Fields(command)
	var tokens []string
	for _, f := range fields {
		// Split on common delimiters
		parts := strings.FieldsFunc(f, func(r rune) bool {
			return r == '-' || r == '_' || r == '/' || r == '.' || r == ','
		})
		tokens = append(tokens, parts...)
	}
	return tokens
}

// EmbeddingManager manages embeddings for commands
type EmbeddingManager struct {
	store      *EmbeddingStore
	commandMap map[string]int // command -> index
	commands   []string       // index -> command
}

// NewEmbeddingManager creates a new embedding manager
func NewEmbeddingManager(dimensions int) *EmbeddingManager {
	return &EmbeddingManager{
		store:      NewEmbeddingStore(dimensions),
		commandMap: make(map[string]int),
		commands:   make([]string, 0),
	}
}

// AddCommand adds a command to the manager
func (em *EmbeddingManager) AddCommand(command string) int {
	if idx, ok := em.commandMap[command]; ok {
		return idx
	}
	
	idx := len(em.commands)
	em.commands = append(em.commands, command)
	em.commandMap[command] = idx
	
	// Generate embedding for this command
	em.store.Set(command, CommandEmbedding(command, em.store))
	
	return idx
}

// GetCommandEmbedding gets embedding for a command
func (em *EmbeddingManager) GetCommandEmbedding(command string) []float64 {
	return em.store.Get(command)
}

// FindSimilarCommands finds similar commands
func (em *EmbeddingManager) FindSimilarCommands(command string, topK int) []SimilarCommand {
	cmdEmb := CommandEmbedding(command, em.store)
	
	var similar []SimilarCommand
	for cmd, idx := range em.commandMap {
		if cmd == command {
			continue
		}
		cmdEmbStored := em.store.Get(cmd)
		similarity := CosineSimilarity(cmdEmb, cmdEmbStored)
		similar = append(similar, SimilarCommand{
			Command:    cmd,
			Index:      idx,
			Similarity: similarity,
		})
	}
	
	// Sort by similarity
	sort.Slice(similar, func(i, j int) bool {
		return similar[i].Similarity > similar[j].Similarity
	})
	
	if topK > 0 && len(similar) > topK {
		similar = similar[:topK]
	}
	
	return similar
}

// SimilarCommand represents a similar command
type SimilarCommand struct {
	Command    string
	Index      int
	Similarity float64
}

// Save saves embeddings to file
func (em *EmbeddingManager) Save(path string) error {
	// Simplified implementation
	return nil
}

// Load loads embeddings from file
func (em *EmbeddingManager) Load(path string) error {
	// Simplified implementation
	return nil
}
