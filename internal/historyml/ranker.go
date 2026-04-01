package historyml

import (
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/cdipaolo/goml/base"
	"github.com/cdipaolo/goml/linear"
)

// CommandSample describes one unique command for personalized ranking.
type CommandSample struct {
	Command     string
	UsageCount  int
	LastUsed    time.Time
	SourceOS    string
	SourceShell string
}

// Ranker wraps a goml model and falls back to heuristics when training data is
// too small or the model can't converge.
type Ranker struct {
	model    *linear.Logistic
	now      time.Time
	maxUsage float64
	trained  bool
}

// Train builds a small personalized ranker from recent command history.
func Train(samples []CommandSample, now time.Time) *Ranker {
	ranker := &Ranker{
		now: now,
	}
	if len(samples) == 0 {
		return ranker
	}

	for _, sample := range samples {
		if float64(sample.UsageCount) > ranker.maxUsage {
			ranker.maxUsage = float64(sample.UsageCount)
		}
	}
	if ranker.maxUsage <= 0 {
		ranker.maxUsage = 1
	}

	if len(samples) < 6 {
		return ranker
	}

	trainingSet := make([][]float64, 0, len(samples))
	expected := make([]float64, 0, len(samples))
	composite := make([]float64, 0, len(samples))

	for _, sample := range samples {
		trainingSet = append(trainingSet, ranker.features(sample))
		composite = append(composite, ranker.labelScore(sample))
	}

	sorted := append([]float64(nil), composite...)
	sort.Float64s(sorted)
	threshold := sorted[len(sorted)/2]

	positives := 0
	for _, score := range composite {
		label := 0.0
		if score >= threshold {
			label = 1
			positives++
		}
		expected = append(expected, label)
	}
	if positives == 0 || positives == len(expected) {
		return ranker
	}

	model := linear.NewLogistic(base.BatchGA, 0.15, 0.05, 250, trainingSet, expected)
	model.Output = io.Discard
	if err := model.Learn(); err != nil {
		return ranker
	}

	ranker.model = model
	ranker.trained = true
	return ranker
}

// Score returns a probability-like boost in the range [0, 1].
func (r *Ranker) Score(sample CommandSample) float64 {
	if r == nil {
		return 0
	}

	if r.trained && r.model != nil {
		if guess, err := r.model.Predict(r.features(sample)); err == nil && len(guess) > 0 {
			return clamp01(guess[0])
		}
	}

	return clamp01(r.labelScore(sample))
}

func (r *Ranker) features(sample CommandSample) []float64 {
	command := strings.TrimSpace(sample.Command)
	tokens := strings.Fields(command)
	lengthNorm := math.Min(1, float64(len(command))/96.0)
	tokenNorm := math.Min(1, float64(len(tokens))/8.0)
	usageNorm := math.Log1p(float64(maxInt(sample.UsageCount, 0))) / math.Log1p(maxFloat64(r.maxUsage, 1))
	freshness := recencyFreshness(r.now, sample.LastUsed)
	hasFlag := 0.0
	hasPipe := 0.0
	if strings.Contains(command, "-") {
		hasFlag = 1
	}
	if strings.ContainsAny(command, "|><&") {
		hasPipe = 1
	}

	sourceOS := strings.ToLower(strings.TrimSpace(sample.SourceOS))
	sourceShell := strings.ToLower(strings.TrimSpace(sample.SourceShell))

	isWindows := 0.0
	isPowerShell := 0.0
	isPosixShell := 0.0
	if sourceOS == "windows" {
		isWindows = 1
	}
	if strings.Contains(sourceShell, "powershell") || strings.Contains(sourceShell, "pwsh") {
		isPowerShell = 1
	}
	if sourceShell == "bash" || sourceShell == "zsh" || sourceShell == "fish" || sourceShell == "sh" {
		isPosixShell = 1
	}

	return []float64{
		usageNorm,
		freshness,
		lengthNorm,
		tokenNorm,
		hasFlag,
		hasPipe,
		isWindows,
		isPowerShell,
		isPosixShell,
	}
}

func (r *Ranker) labelScore(sample CommandSample) float64 {
	usageNorm := math.Log1p(float64(maxInt(sample.UsageCount, 0))) / math.Log1p(maxFloat64(r.maxUsage, 1))
	freshness := recencyFreshness(r.now, sample.LastUsed)
	return usageNorm*0.7 + freshness*0.3
}

func recencyFreshness(now, ts time.Time) float64 {
	if ts.IsZero() {
		return 0
	}

	hours := now.Sub(ts).Hours()
	switch {
	case hours < 24:
		return 1
	case hours < 24*7:
		return 0.75
	case hours < 24*30:
		return 0.4
	case hours < 24*90:
		return 0.15
	default:
		return 0.05
	}
}

func clamp01(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
