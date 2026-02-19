// Package ai provides AI functionality for WUT
package ai

import (
	"math"
	"math/rand"
	"sort"
)

// NeuralNetwork represents a simple feedforward neural network
type NeuralNetwork struct {
	InputSize   int
	HiddenSize  int
	OutputSize  int
	NumLayers   int
	
	Weights     [][][]float64 // layer -> from -> to
	Biases      [][]float64   // layer -> neuron
	
	// For training
	learningRate float64
	cache        map[string]*PredictionCache
}

// PredictionCache caches prediction results
type PredictionCache struct {
	Input       []float64
	Output      []float64
	Timestamp   int64
}

// NewNeuralNetwork creates a new neural network
func NewNeuralNetwork(inputSize, hiddenSize, outputSize, numLayers int) *NeuralNetwork {
	if numLayers < 2 {
		numLayers = 2
	}
	
	nn := &NeuralNetwork{
		InputSize:    inputSize,
		HiddenSize:   hiddenSize,
		OutputSize:   outputSize,
		NumLayers:    numLayers,
		Weights:      make([][][]float64, numLayers),
		Biases:       make([][]float64, numLayers),
		learningRate: 0.01,
		cache:        make(map[string]*PredictionCache),
	}
	
	// Initialize weights with Xavier initialization
	nn.initializeWeights()
	
	return nn
}

// initializeWeights initializes weights with Xavier initialization
func (nn *NeuralNetwork) initializeWeights() {
	rng := rand.New(rand.NewSource(42))
	
	for layer := 0; layer < nn.NumLayers; layer++ {
		var inputSize, outputSize int
		
		if layer == 0 {
			inputSize = nn.InputSize
			outputSize = nn.HiddenSize
		} else if layer == nn.NumLayers-1 {
			inputSize = nn.HiddenSize
			outputSize = nn.OutputSize
		} else {
			inputSize = nn.HiddenSize
			outputSize = nn.HiddenSize
		}
		
		nn.Weights[layer] = make([][]float64, inputSize)
		for i := 0; i < inputSize; i++ {
			nn.Weights[layer][i] = make([]float64, outputSize)
			for j := 0; j < outputSize; j++ {
				// Xavier initialization
				scale := math.Sqrt(2.0 / float64(inputSize+outputSize))
				nn.Weights[layer][i][j] = rng.NormFloat64() * scale
			}
		}
		
		nn.Biases[layer] = make([]float64, outputSize)
		for j := 0; j < outputSize; j++ {
			nn.Biases[layer][j] = 0.0
		}
	}
}

// Forward performs forward propagation
func (nn *NeuralNetwork) Forward(input []float64) []float64 {
	if len(input) != nn.InputSize {
		input = padOrTruncate(input, nn.InputSize)
	}
	
	current := input
	
	for layer := 0; layer < nn.NumLayers; layer++ {
		inputSize := len(current)
		outputSize := len(nn.Biases[layer])
		output := make([]float64, outputSize)
		
		for j := 0; j < outputSize; j++ {
			sum := nn.Biases[layer][j]
			for i := 0; i < inputSize; i++ {
				if i < len(nn.Weights[layer]) && j < len(nn.Weights[layer][i]) {
					sum += current[i] * nn.Weights[layer][i][j]
				}
			}
			
			// Apply activation function
			if layer < nn.NumLayers-1 {
				output[j] = relu(sum)
			} else {
				output[j] = sum // Linear output for last layer
			}
		}
		
		current = output
	}
	
	// Apply softmax to output layer
	return softmax(current)
}

// Predict makes a prediction and returns confidence scores
func (nn *NeuralNetwork) Predict(input []float64) *Prediction {
	output := nn.Forward(input)
	
	// Handle empty output
	if len(output) == 0 {
		return &Prediction{
			ClassIndex: 0,
			Confidence: 0,
			Probabilities: []float64{},
		}
	}
	
	// Find the highest confidence class
	maxIdx := 0
	maxVal := output[0]
	for i, val := range output {
		if val > maxVal {
			maxVal = val
			maxIdx = i
		}
	}
	
	return &Prediction{
		ClassIndex: maxIdx,
		Confidence: maxVal,
		Probabilities: output,
	}
}

// Prediction represents a prediction result
type Prediction struct {
	ClassIndex    int
	Confidence    float64
	Probabilities []float64
}

// Train performs a single training step
func (nn *NeuralNetwork) Train(input []float64, target []float64) float64 {
	// Forward pass (store activations)
	activations := make([][]float64, nn.NumLayers+1)
	activations[0] = input
	
	current := input
	for layer := 0; layer < nn.NumLayers; layer++ {
		inputSize := len(current)
		outputSize := len(nn.Biases[layer])
		output := make([]float64, outputSize)
		
		for j := 0; j < outputSize; j++ {
			sum := nn.Biases[layer][j]
			for i := 0; i < inputSize; i++ {
				sum += current[i] * nn.Weights[layer][i][j]
			}
			
			if layer < nn.NumLayers-1 {
				output[j] = relu(sum)
			} else {
				output[j] = sum
			}
		}
		
		activations[layer+1] = output
		current = output
	}
	
	// Softmax for output
	output := softmax(current)
	
	// Calculate loss (cross-entropy)
	loss := crossEntropyLoss(output, target)
	
	// Backward pass (simplified gradient descent)
	// Calculate output error
	outputError := make([]float64, len(output))
	for i := range output {
		outputError[i] = output[i] - target[i]
	}
	
	// Update weights for last layer
	lastLayer := nn.NumLayers - 1
	for i := 0; i < len(activations[lastLayer]); i++ {
		for j := 0; j < len(outputError); j++ {
			if i < len(nn.Weights[lastLayer]) && j < len(nn.Weights[lastLayer][i]) {
				gradient := outputError[j] * activations[lastLayer][i]
				nn.Weights[lastLayer][i][j] -= nn.learningRate * gradient
			}
		}
	}
	
	// Update biases for last layer
	for j := 0; j < len(outputError); j++ {
		nn.Biases[lastLayer][j] -= nn.learningRate * outputError[j]
	}
	
	return loss
}

// SetLearningRate sets the learning rate
func (nn *NeuralNetwork) SetLearningRate(lr float64) {
	nn.learningRate = lr
}

// relu applies ReLU activation
func relu(x float64) float64 {
	if x > 0 {
		return x
	}
	return 0
}

// softmax applies softmax function
func softmax(x []float64) []float64 {
	if len(x) == 0 {
		return []float64{}
	}
	
	result := make([]float64, len(x))
	max := x[0]
	for _, v := range x[1:] {
		if v > max {
			max = v
		}
	}
	
	sum := 0.0
	for i, v := range x {
		result[i] = math.Exp(v - max)
		sum += result[i]
	}
	
	for i := range result {
		result[i] /= sum
	}
	
	return result
}

// crossEntropyLoss calculates cross-entropy loss
func crossEntropyLoss(predicted, target []float64) float64 {
	epsilon := 1e-15
	loss := 0.0
	for i := range predicted {
		if target[i] > 0 {
			loss -= target[i] * math.Log(predicted[i]+epsilon)
		}
	}
	return loss
}

// padOrTruncate pads or truncates a slice to target length
func padOrTruncate(input []float64, targetLength int) []float64 {
	if len(input) == targetLength {
		return input
	}
	
	result := make([]float64, targetLength)
	if len(input) > targetLength {
		copy(result, input[:targetLength])
	} else {
		copy(result, input)
		// Pad with zeros
		for i := len(input); i < targetLength; i++ {
			result[i] = 0
		}
	}
	return result
}

// Quantize quantizes the model weights to reduce size
func (nn *NeuralNetwork) Quantize() *QuantizedNetwork {
	qn := &QuantizedNetwork{
		InputSize:  nn.InputSize,
		HiddenSize: nn.HiddenSize,
		OutputSize: nn.OutputSize,
		NumLayers:  nn.NumLayers,
		Weights:    make([][]int8, nn.NumLayers),
		Biases:     make([][]int8, nn.NumLayers),
		Scales:     make([]float64, nn.NumLayers),
		BiasesMin:  make([]float64, nn.NumLayers),
	}
	
	for layer := 0; layer < nn.NumLayers; layer++ {
		// Find min and max for weights scaling
		minVal, maxVal := math.MaxFloat64, -math.MaxFloat64
		for i := range nn.Weights[layer] {
			for j := range nn.Weights[layer][i] {
				if nn.Weights[layer][i][j] < minVal {
					minVal = nn.Weights[layer][i][j]
				}
				if nn.Weights[layer][i][j] > maxVal {
					maxVal = nn.Weights[layer][i][j]
				}
			}
		}
		
		// Quantize weights to int8
		weightScale := (maxVal - minVal) / 255.0
		if weightScale == 0 {
			weightScale = 1.0
		}
		qn.Scales[layer] = weightScale
		
		// Determine weight array size
		var weightSize int
		if layer == 0 {
			weightSize = nn.InputSize * nn.HiddenSize
		} else if layer == nn.NumLayers-1 {
			weightSize = nn.HiddenSize * nn.OutputSize
		} else {
			weightSize = nn.HiddenSize * nn.HiddenSize
		}
		
		qn.Weights[layer] = make([]int8, weightSize)
		
		// Quantize and store weights
		idx := 0
		for i := range nn.Weights[layer] {
			for j := range nn.Weights[layer][i] {
				// Convert to int8 range [-128, 127]
				normalized := (nn.Weights[layer][i][j] - minVal) / weightScale
				quantized := int8(normalized - 128)
				qn.Weights[layer][idx] = quantized
				idx++
			}
		}
		
		// Quantize biases separately (different range)
		biasMin, biasMax := math.MaxFloat64, -math.MaxFloat64
		for j := range nn.Biases[layer] {
			if nn.Biases[layer][j] < biasMin {
				biasMin = nn.Biases[layer][j]
			}
			if nn.Biases[layer][j] > biasMax {
				biasMax = nn.Biases[layer][j]
			}
		}
		
		qn.BiasesMin[layer] = biasMin
		biasScale := (biasMax - biasMin) / 255.0
		if biasScale == 0 {
			biasScale = 1.0
		}
		
		qn.Biases[layer] = make([]int8, len(nn.Biases[layer]))
		for j := range nn.Biases[layer] {
			normalized := (nn.Biases[layer][j] - biasMin) / biasScale
			qn.Biases[layer][j] = int8(normalized - 128)
		}
	}
	
	return qn
}

// QuantizedNetwork represents a quantized neural network
type QuantizedNetwork struct {
	InputSize   int
	HiddenSize  int
	OutputSize  int
	NumLayers   int
	Weights     [][]int8
	Biases      [][]int8
	Scales      []float64
	BiasesMin   []float64
}

// SuggestionNetwork is a specialized network for command suggestions
type SuggestionNetwork struct {
	*NeuralNetwork
	embeddingSize int
	numCommands   int
}

// NewSuggestionNetwork creates a new suggestion network
func NewSuggestionNetwork(embeddingSize, numCommands int) *SuggestionNetwork {
	// Simple network: input -> hidden -> output
	// Input: concatenated embeddings (command + context)
	// Output: probability distribution over commands
	
	hiddenSize := 64
	if embeddingSize > 64 {
		hiddenSize = embeddingSize / 2
	}
	
	return &SuggestionNetwork{
		NeuralNetwork: NewNeuralNetwork(embeddingSize*2, hiddenSize, numCommands, 2),
		embeddingSize: embeddingSize,
		numCommands:   numCommands,
	}
}

// Suggest returns command suggestions based on input
func (sn *SuggestionNetwork) Suggest(commandEmbedding, contextEmbedding []float64, topK int) []Suggestion {
	// Concatenate embeddings
	input := append(commandEmbedding, contextEmbedding...)
	
	prediction := sn.Predict(input)
	
	var suggestions []Suggestion
	for i, prob := range prediction.Probabilities {
		suggestions = append(suggestions, Suggestion{
			CommandIndex: i,
			Confidence:   prob,
		})
	}
	
	// Sort by confidence
	sortSuggestions(suggestions)
	
	// Return top K
	if topK > 0 && len(suggestions) > topK {
		suggestions = suggestions[:topK]
	}
	
	return suggestions
}

// Suggestion represents a command suggestion
type Suggestion struct {
	CommandIndex int
	Confidence   float64
}

// sortSuggestions sorts suggestions by confidence (descending)
func sortSuggestions(suggestions []Suggestion) {
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})
}
